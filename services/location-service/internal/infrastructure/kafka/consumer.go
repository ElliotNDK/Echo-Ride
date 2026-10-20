package kafka

import (
	"context"
	"echo-ride/services/location-service/config"
	"echo-ride/services/location-service/internal/application"
	"echo-ride/services/location-service/internal/domain"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type IncomingKafkaEvent struct {
	EventID      string            `json:"event_id"`
	EventType    domain.UserStatus `json:"event_type"`
	EventPayload string            `json:"event_payload"`
}

type DriverMatchedPayload struct {
	RideID   string `json:"ride_id"`
	DriverID string `json:"driver_id"`
}

type RideAcceptedPayload struct {
	RideID   string `json:"ride_id"`
	RiderID  string `json:"rider_id"`
	DriverID string `json:"driver_id"`
}

type TripStatusPayload struct {
	RideID   string            `json:"ride_id"`
	RiderID  string            `json:"rider_id"`
	DriverID string            `json:"driver_id"`
	Status   domain.UserStatus `json:"status"`
}

type RideConsumer struct {
	reader    *kafka.Reader
	notifyUC  application.NotifyUserUseCase
	dlqWriter *kafka.Writer
	// trackingSub subscribes a rider's active WS connection to the Redis
	// Pub/Sub channel for their ride as soon as the ride is accepted, so no
	// location frames are lost between the "accepted" notification and the
	// rider app manually sending SUBSCRIBE_RIDE.
	trackingSub application.RideTrackingSubscriber
	logger      *zap.Logger
}

func NewRideConsumer(
	cfg config.KafkaConfig,
	notifyUC application.NotifyUserUseCase,
	trackingSub application.RideTrackingSubscriber,
	logger *zap.Logger,
) *RideConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  cfg.Brokers,
		Topic:    cfg.Topic,
		GroupID:  cfg.GroupID,
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})

	dlqWriter := &kafka.Writer{
		Addr:     kafka.TCP(cfg.Brokers...),
		Topic:    cfg.Topic,
		Balancer: &kafka.LeastBytes{},
	}

	return &RideConsumer{
		reader:      reader,
		dlqWriter:   dlqWriter,
		notifyUC:    notifyUC,
		trackingSub: trackingSub,
		logger:      logger,
	}
}

func (c *RideConsumer) Start(ctx context.Context) {
	c.logger.Info("Location Service Consumer started, listening to topic")

	for {
		m, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			c.logger.Warn("Failed to fetch Message", zap.Error(err))
			continue
		}

		if len(m.Value) == 0 {
			c.reader.CommitMessages(ctx, m)
			continue
		}

		var event IncomingKafkaEvent
		if err := json.Unmarshal(m.Value, &event); err != nil {
			c.logger.Error("Failed to unmarshal Kafka message", zap.Error(err), zap.ByteString("message", m.Value))
			c.reader.CommitMessages(ctx, m)
			continue
		}

		switch event.EventType {
		case domain.DriverStatusMatched:
			c.handleDriverMatched(ctx, m, event)
		case domain.RideAcceptStatusAccepted:
			c.handleRideAccepted(ctx, m, event)
		case domain.DriverArrivedStatus, domain.InProgressStatus, domain.CompletedStatus, domain.CancelledStatus:
			c.handleTripStatusChange(ctx, m, event)
		default:
			c.logger.Debug("Received unsupported event type, skipping", zap.String("event_type", string(event.EventType)))
			c.reader.CommitMessages(ctx, m)
		}
	}
}

func (c *RideConsumer) handleDriverMatched(ctx context.Context, m kafka.Message, event IncomingKafkaEvent) {
	var payload DriverMatchedPayload

	if err := json.Unmarshal([]byte(event.EventPayload), &payload); err != nil {
		c.logger.Error("Failed to unmarshal Kafka message", zap.Error(err), zap.ByteString("message", m.Value))
		c.reader.CommitMessages(ctx, m)
		return
	}

	driverUUID, err := uuid.Parse(payload.DriverID)
	if err != nil {
		c.logger.Error("Failed to parse driver UUID", zap.Error(err), zap.String("driver_id", payload.DriverID))
		c.reader.CommitMessages(ctx, m)
		return
	}

	if err := c.notifyUC.Execute(ctx, driverUUID, "NEW_RIDE_ASSIGNED", payload); err != nil {
		c.logger.Error("Failed to notify driver of matched ride", zap.Error(err), zap.String("driver_id", payload.DriverID), zap.String("ride_id", payload.RideID))
	}

	c.reader.CommitMessages(ctx, m)
}

func (c *RideConsumer) handleRideAccepted(ctx context.Context, m kafka.Message, event IncomingKafkaEvent) {
	var payload RideAcceptedPayload
	if err := json.Unmarshal([]byte(event.EventPayload), &payload); err != nil {
		c.logger.Error("Failed to parse RIDE_ACCEPTED payload", zap.Error(err))
		c.reader.CommitMessages(ctx, m)
		return
	}

	riderUUID, err := uuid.Parse(payload.RiderID)
	if err != nil {
		c.logger.Error("Failed to parse rider UUID", zap.Error(err), zap.String("rider_id", payload.RiderID))
		c.reader.CommitMessages(ctx, m)
		return
	}

	// Notify rider that their ride has been accepted.
	_ = c.notifyUC.Execute(ctx, riderUUID, "RIDE_STATUS_ACCEPTED", map[string]string{
		"ride_id":   payload.RideID,
		"driver_id": payload.DriverID,
		"message":   "Your ride has been accepted by a driver",
	})

	// Auto-subscribe the rider's WS connection to the live-tracking channel
	// so that driver location frames start arriving immediately — no separate
	// SUBSCRIBE_RIDE message from the client app required.
	if payload.RideID != "" {
		c.trackingSub.SubscribeToRideTracking(ctx, riderUUID, payload.RideID)
		c.logger.Info("Auto-subscribed rider to ride tracking",
			zap.String("rider_id", payload.RiderID),
			zap.String("ride_id", payload.RideID),
		)
	}

	c.reader.CommitMessages(ctx, m)
}

func (c *RideConsumer) handleTripStatusChange(ctx context.Context, m kafka.Message, event IncomingKafkaEvent) {
	var payload TripStatusPayload
	if err := json.Unmarshal([]byte(event.EventPayload), &payload); err != nil {
		c.logger.Error("Failed to parse TripStatus payload", zap.Error(err))
		c.reader.CommitMessages(ctx, m)
		return
	}

	riderUUID, err := uuid.Parse(payload.RiderID)
	if err != nil {
		c.reader.CommitMessages(ctx, m)
		return
	}

	messageText := ""
	switch payload.Status {
	case domain.DriverArrivedStatus:
		messageText = "Your driver has arrived at the pickup point"
	case domain.InProgressStatus:
		messageText = "Your ride is now in progress"
	case domain.CompletedStatus:
		messageText = "Your ride has been completed"
	case domain.CancelledStatus:
		messageText = "Your ride has been cancelled"
	}

	err = c.notifyUC.Execute(ctx, riderUUID, "TRIP_STATUS_UPDATED", map[string]string{
		"ride_id": payload.RideID,
		"status":  string(payload.Status),
		"message": messageText,
	})

	if err != nil {
		c.logger.Error("Failed to notify rider about trip status", zap.Error(err))
	}

	c.reader.CommitMessages(ctx, m)
}

func (c *RideConsumer) Close() error {
	c.dlqWriter.Close()
	return c.reader.Close()
}
