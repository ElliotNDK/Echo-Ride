package kafka

import (
	"context"
	"echo-ride/services/matching-service/config"
	"echo-ride/services/matching-service/internal/application"
	"echo-ride/services/matching-service/internal/domain"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type IncomingKafkaEvent struct {
	EventID            string                 `json:"event_id"`
	EventAggregateID   string                 `json:"event_aggregate_id"`
	EventAggregateType string                 `json:"event_aggregate_type"`
	EventType          domain.RideEventStatus `json:"event_type"`
	EventPayload       string                 `json:"event_payload"`
	EventStatus        string                 `json:"event_status"`
	TraceContext       map[string]string      `json:"trace_context"`
}

type RideRequestedPayload struct {
	ID        string  `json:"id"`
	RiderID   string  `json:"rider_id"`
	PickupLat float64 `json:"pickup_lat"`
	PickupLng float64 `json:"pickup_lng"`
}

type CancelledPayload struct {
	RideID      string `json:"ride_id"`
	DriverID    string `json:"driver_id"`
	CancelledBy string `json:"cancelled_by"`
}

type RideConsumer struct {
	reader            *kafka.Reader
	dlqWriter         *kafka.Writer
	uc                application.ProcessMatchingUseCase
	handleAcceptUC    application.HandleRideAcceptedUseCase
	handleCancelledUC application.HandleCancelledUseCase
	dispatchRepo      domain.DispatchRepository
	logger            *zap.Logger
}

func NewRideConsumer(
	cfg config.KafkaConfig,
	uc application.ProcessMatchingUseCase,
	handleAcceptUC application.HandleRideAcceptedUseCase,
	handleCancelledUC application.HandleCancelledUseCase,
	dispatchRepo domain.DispatchRepository,
	logger *zap.Logger,
) *RideConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  cfg.Brokers,
		Topic:    cfg.Topic,
		GroupID:  cfg.GroupID,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})

	dlqWriter := &kafka.Writer{
		Addr:     kafka.TCP(cfg.Brokers...),
		Topic:    cfg.Topic,
		Balancer: &kafka.LeastBytes{},
	}

	return &RideConsumer{
		reader:            reader,
		dlqWriter:         dlqWriter,
		uc:                uc,
		handleAcceptUC:    handleAcceptUC,
		handleCancelledUC: handleCancelledUC,
		dispatchRepo:      dispatchRepo,
		logger:            logger,
	}
}

func (c *RideConsumer) Start(ctx context.Context) {
	c.logger.Info("Kafka Consumer started, listening to topic", zap.String("topic", c.reader.Config().Topic))

	for {
		m, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil || strings.Contains(err.Error(), "context canceled") {
				c.logger.Info("Kafka Consumer context canceled", zap.String("topic", c.reader.Config().Topic))
				return
			}
			c.logger.Error("Kafka Consumer error", zap.Error(err))
			continue
		}
		c.processMessage(ctx, m)
	}
}

func (c *RideConsumer) processMessage(ctx context.Context, m kafka.Message) {
	if len(m.Value) == 0 {
		c.logger.Debug("Received tombstone message (empty value), skipping", zap.ByteString("key", m.Key))
		c.reader.CommitMessages(ctx, m)
		return
	}

	var event IncomingKafkaEvent
	if err := json.Unmarshal(m.Value, &event); err != nil {
		c.logger.Error("Failed to unmarshal Kafka message", zap.Error(err), zap.ByteString("message", m.Value))
		c.sendToDLQ(ctx, m, err.Error())
		c.reader.CommitMessages(ctx, m)
		return
	}

	switch event.EventType {
	case domain.RideEventStatusRequested,
		domain.RideEventStatusAccepted,
		domain.RideEventStatusCancelled,
		domain.RideEventStatusDeclined:
		// fallthrough to processing below
	default:
		// Not actionable for this consumer — commit & skip silently.
		c.reader.CommitMessages(ctx, m)
		return
	}

	parentCtx := otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(event.TraceContext))
	tracer := otel.Tracer("matching-service")
	spanCtx, span := tracer.Start(parentCtx, string("Kafka Consume "+event.EventType), trace.WithSpanKind(trace.SpanKindConsumer))
	defer span.End()

	// Idempotency: read-only check first. The token is *only* persisted after
	// the handler succeeds (via MarkProcessed). This prevents the classic
	// "lost event" bug where a SETNX-before-process consumes the token even
	// when the handler crashes mid-flight.
	processed, err := c.dispatchRepo.WasProcessed(spanCtx, event.EventID)
	if err != nil {
		c.logger.Error("Failed to check idempotency", zap.Error(err), zap.String("event_id", event.EventID))
		return
	}
	if processed {
		c.logger.Debug("Skipping already-processed event", zap.String("event_id", event.EventID))
		c.reader.CommitMessages(ctx, m)
		return
	}

	var handlerErr error
	switch event.EventType {
	case domain.RideEventStatusRequested:
		handlerErr = c.handleRideRequested(spanCtx, m, event)
	case domain.RideEventStatusAccepted:
		handlerErr = c.handleRideAccepted(spanCtx, m, event)
	case domain.RideEventStatusCancelled:
		handlerErr = c.handleRideCancelled(spanCtx, m, event)
	case domain.RideEventStatusDeclined:
		// RIDE_DECLINED = driver refusing an offer during dispatch.
		// Reuse handleRideCancelled: payload shape is identical and
		// cancelled_by is always DRIVER, so the use-case fast-forwards
		// to the next candidate exactly as a driver-cancel does.
		handlerErr = c.handleRideCancelled(spanCtx, m, event)
	}

	if handlerErr != nil {
		c.reader.CommitMessages(ctx, m)
		return
	}

	if err := c.dispatchRepo.MarkProcessed(spanCtx, event.EventID); err != nil {
		// Logged but non-fatal — at-least-once is the contract; duplicate
		// delivery will be re-handled idempotently by the business logic
		// (CAS on Redis state in the use-cases).
		c.logger.Warn("Failed to mark event processed", zap.Error(err), zap.String("event_id", event.EventID))
	}
	c.reader.CommitMessages(ctx, m)
}

// retryHandler runs op with exponential backoff. Returns the last error if all
// attempts fail or ctx is cancelled.
func (c *RideConsumer) retryHandler(ctx context.Context, label string, op func() error) error {
	const maxRetries = 3
	backoff := 1 * time.Second
	var err error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err = op(); err == nil {
			return nil
		}
		c.logger.Warn("handler attempt failed", zap.String("op", label), zap.Int("attempt", attempt), zap.Error(err))
		if attempt == maxRetries {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
		}
	}
	return err
}

func (c *RideConsumer) handleRideRequested(ctx context.Context, m kafka.Message, event IncomingKafkaEvent) error {
	payload, err := decodeRequestedPayload(event.EventPayload)
	if err != nil {
		c.logger.Error("Failed to decode REQUESTED payload", zap.Error(err))
		c.sendToDLQ(ctx, m, "invalid_payload")
		return err
	}

	domainEvent := domain.RideRequestEvent{
		RideID:    payload.ID,
		RiderID:   payload.RiderID,
		PickupLat: payload.PickupLat,
		PickupLng: payload.PickupLng,
	}

	if err := c.retryHandler(ctx, "ProcessMatching", func() error {
		return c.uc.Execute(ctx, domainEvent)
	}); err != nil {
		c.logger.Error("Failed to process REQUESTED after retries", zap.Error(err), zap.String("ride_id", payload.ID))
		c.sendToDLQ(ctx, m, err.Error())
		return err
	}

	c.logger.Info("Processed REQUESTED event", zap.String("ride_id", payload.ID))
	return nil
}

func (c *RideConsumer) handleRideAccepted(ctx context.Context, m kafka.Message, event IncomingKafkaEvent) error {
	rideID := event.EventAggregateID
	c.logger.Info("Ride has been ACCEPTED. Stopping dispatcher...", zap.String("ride_id", rideID))

	if err := c.retryHandler(ctx, "HandleAccepted", func() error {
		return c.handleAcceptUC.Execute(ctx, rideID)
	}); err != nil {
		c.logger.Error("Failed to handle ACCEPTED after retries", zap.Error(err), zap.String("ride_id", rideID))
		c.sendToDLQ(ctx, m, err.Error())
		return err
	}
	return nil
}

func (c *RideConsumer) handleRideCancelled(ctx context.Context, m kafka.Message, event IncomingKafkaEvent) error {
	payload, err := decodeCancelledPayload(event.EventPayload)
	if err != nil {
		c.logger.Error("Failed to decode CANCELLED payload", zap.Error(err))
		c.sendToDLQ(ctx, m, "invalid_payload")
		return err
	}

	cancelledBy := payload.CancelledBy
	if cancelledBy == "" {
		c.logger.Warn("CANCELLED event missing cancelled_by; defaulting to RIDER",
			zap.String("ride_id", payload.RideID))
		cancelledBy = string(domain.CancelledByRider)
	}

	if err := c.handleCancelledUC.Execute(ctx, payload.RideID, payload.DriverID, domain.CancelledBy(cancelledBy)); err != nil {
		c.logger.Error("Failed to handle CANCELLED", zap.Error(err), zap.String("ride_id", payload.RideID))
		c.sendToDLQ(ctx, m, err.Error())
		return err
	}
	return nil
}

func decodeRequestedPayload(raw string) (RideRequestedPayload, error) {
	var p RideRequestedPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return p, err
	}
	return p, nil
}

func decodeCancelledPayload(raw string) (CancelledPayload, error) {
	var p CancelledPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return p, err
	}
	return p, nil
}

func (c *RideConsumer) sendToDLQ(ctx context.Context, m kafka.Message, errorMsg string) {
	headers := []kafka.Header{
		{Key: "error-reason", Value: []byte(errorMsg)},
		{Key: "original-topic", Value: []byte(c.reader.Config().Topic)},
		{Key: "original-partition", Value: []byte(fmt.Sprintf("%d", m.Partition))},
		{Key: "original-offset", Value: []byte(fmt.Sprintf("%d", m.Offset))},
		{Key: "failure-timestamp", Value: []byte(time.Now().Format(time.RFC3339))},
	}

	headers = append(headers, m.Headers...)

	dlsMsg := kafka.Message{
		Key:     m.Key,
		Value:   m.Value,
		Headers: headers,
	}

	if err := c.dlqWriter.WriteMessages(ctx, dlsMsg); err != nil {
		c.logger.Error("Failed to write message to DLQ", zap.Error(err), zap.ByteString("message", m.Value))
	} else {
		c.logger.Info("Message sent to DLQ", zap.ByteString("message", m.Value), zap.String("error", errorMsg))
	}
}

func (c *RideConsumer) Close() error {
	c.dlqWriter.Close()
	return c.reader.Close()
}
