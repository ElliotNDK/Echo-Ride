package kafka

import (
	"context"
	"echo-ride/services/ride-service/config"
	"echo-ride/services/ride-service/internal/application"
	"echo-ride/services/ride-service/internal/domain"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// IncomingKafkaEvent mirrors the envelope produced by matching-service publisher
// and the Debezium relay. EventPayload is a JSON-encoded string in both paths.
type IncomingKafkaEvent struct {
	EventID      string            `json:"event_id"`
	EventType    domain.EventType  `json:"event_type"`
	EventPayload string            `json:"event_payload"`
	TraceContext map[string]string `json:"trace_context,omitempty"`
}

type MatchingFailedPayload struct {
	RideID string `json:"ride_id"`
	Reason string `json:"reason"`
}

type RideConsumer struct {
	reader             *kafka.Reader
	updateRideStatusUC application.UpdateRideUseCase
	logger             *zap.Logger
	tracer             trace.Tracer
}

func NewRideConsumer(
	cfg config.KafkaConfig,
	updateRideStatusUC application.UpdateRideUseCase,
	logger *zap.Logger,
) *RideConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  cfg.Brokers,
		Topic:    cfg.Topic,
		GroupID:  "ride-service-consumer",
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})

	return &RideConsumer{
		reader:             reader,
		updateRideStatusUC: updateRideStatusUC,
		logger:             logger,
		tracer:             otel.Tracer("ride-service-consumer"),
	}
}

func (c *RideConsumer) Start(ctx context.Context) {
	c.logger.Info("Ride Service Consumer started", zap.String("topic", c.reader.Config().Topic))

	for {
		m, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil || strings.Contains(err.Error(), "context canceled") {
				c.logger.Info("Ride Service Consumer context canceled", zap.String("topic", c.reader.Config().Topic))
				return
			}
			c.logger.Error("Failed to fetch message", zap.Error(err))
			continue
		}
		c.processMessage(ctx, m)
	}
}

func (c *RideConsumer) processMessage(ctx context.Context, m kafka.Message) {
	if len(m.Value) == 0 {
		c.commit(ctx, m)
		return
	}

	var event IncomingKafkaEvent
	if err := json.Unmarshal(m.Value, &event); err != nil {
		c.logger.Error("Failed to parse event envelope", zap.Error(err), zap.ByteString("raw", m.Value))
		c.commit(ctx, m)
		return
	}

	// Only RIDE_FAILED is actionable for ride-service. Other event types
	// (RIDE_REQUESTED/ACCEPTED/CANCELLED/DRIVER_MATCHED/...) are emitted by
	// this service or consumed elsewhere; commit & skip silently.
	if event.EventType != domain.EventTypeRideFailed {
		c.commit(ctx, m)
		return
	}

	parentCtx := otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(event.TraceContext))
	spanCtx, span := c.tracer.Start(parentCtx, "Kafka Consume "+string(event.EventType), trace.WithSpanKind(trace.SpanKindConsumer))
	defer span.End()

	c.handleMatchingFailed(spanCtx, m, event)
}

func (c *RideConsumer) handleMatchingFailed(ctx context.Context, m kafka.Message, event IncomingKafkaEvent) {
	payload, err := decodeFailedPayload(event.EventPayload)
	if err != nil {
		c.logger.Error("Failed to parse RIDE_FAILED payload", zap.Error(err), zap.String("payload", event.EventPayload))
		c.commit(ctx, m)
		return
	}

	rideID, err := uuid.Parse(payload.RideID)
	if err != nil {
		c.logger.Error("Invalid ride_id UUID", zap.Error(err), zap.String("ride_id", payload.RideID))
		c.commit(ctx, m)
		return
	}

	maxRetries := 3
	backoff := 500 * time.Millisecond
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err = c.updateRideStatusUC.Execute(ctx, rideID, domain.RideStatusFailed)
		if err == nil {
			break
		}
		c.logger.Warn("Failed to update ride status to FAILED", zap.Error(err), zap.Int("attempt", attempt))
		if attempt == maxRetries {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
			backoff *= 2
		}
	}

	if err != nil {
		c.logger.Error("Giving up updating ride status after retries; leaving message uncommitted", zap.Error(err), zap.String("ride_id", rideID.String()))
		// Do NOT commit — let the consumer group re-deliver after restart/rebalance.
		// In production this should go to DLQ; out of scope here.
		return
	}

	c.commit(ctx, m)
	c.logger.Info("Marked ride as FAILED", zap.String("ride_id", rideID.String()))
}

func decodeFailedPayload(raw string) (MatchingFailedPayload, error) {
	var p MatchingFailedPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return p, err
	}
	return p, nil
}

func (c *RideConsumer) commit(ctx context.Context, m kafka.Message) {
	if err := c.reader.CommitMessages(ctx, m); err != nil {
		c.logger.Error("Failed to commit kafka message", zap.Error(err))
	}
}

func (c *RideConsumer) Close() error {
	return c.reader.Close()
}
