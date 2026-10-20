package kafka

import (
	"context"
	"echo-ride/services/auth-service/config"
	"echo-ride/services/auth-service/internal/application"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type IncomingKafkaEvent struct {
	EventID      string            `json:"event_id"`
	EventType    string            `json:"event_type"`
	EventPayload string            `json:"event_payload"`
	TraceContext map[string]string `json:"trace_context,omitempty"`
}

type RoleChangedPayload struct {
	UserID  string `json:"user_id"`
	NewRole string `json:"new_role"`
}

type StatusChangedPayload struct {
	UserID    string `json:"user_id"`
	NewStatus string `json:"new_status"`
}

type AuthConsumer struct {
	reader        *kafka.Reader
	handleUserUC  application.HandleUserEventUseCase
	logger        *zap.Logger
	tracer        trace.Tracer
}

func NewAuthConsumer(cfg config.KafkaConfig, userTopic string, handleUserUC application.HandleUserEventUseCase, logger *zap.Logger) *AuthConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  cfg.Brokers,
		Topic:    userTopic,
		GroupID:  "auth-service-consumer",
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})
	return &AuthConsumer{
		reader:       reader,
		handleUserUC: handleUserUC,
		logger:       logger,
		tracer:       otel.Tracer("auth-service-consumer"),
	}
}

func (c *AuthConsumer) Start(ctx context.Context) {
	c.logger.Info("Auth Service Consumer started", zap.String("topic", c.reader.Config().Topic))
	for {
		m, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				c.logger.Info("Auth Service Consumer context canceled")
				return
			}
			c.logger.Error("Failed to fetch message", zap.Error(err))
			continue
		}
		c.processMessage(ctx, m)
	}
}

func (c *AuthConsumer) processMessage(ctx context.Context, m kafka.Message) {
	if len(m.Value) == 0 {
		c.commit(ctx, m)
		return
	}

	var event IncomingKafkaEvent
	if err := json.Unmarshal(m.Value, &event); err != nil {
		c.logger.Error("Failed to parse event envelope", zap.Error(err))
		c.commit(ctx, m)
		return
	}

	parentCtx := otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(event.TraceContext))
	spanCtx, span := c.tracer.Start(parentCtx, "Kafka Consume "+event.EventType, trace.WithSpanKind(trace.SpanKindConsumer))
	defer span.End()

	switch event.EventType {
	case "ROLE_CHANGED":
		c.handleRoleChanged(spanCtx, m, event)
	case "STATUS_CHANGED":
		c.handleStatusChanged(spanCtx, m, event)
	default:
		c.commit(ctx, m)
	}
}

func (c *AuthConsumer) handleRoleChanged(ctx context.Context, m kafka.Message, event IncomingKafkaEvent) {
	var p RoleChangedPayload
	if err := json.Unmarshal([]byte(event.EventPayload), &p); err != nil {
		c.logger.Error("Failed to parse ROLE_CHANGED payload", zap.Error(err))
		c.commit(ctx, m)
		return
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		c.logger.Error("Invalid user_id", zap.Error(err))
		c.commit(ctx, m)
		return
	}
	if err := c.handleUserUC.HandleRoleChanged(ctx, userID, p.NewRole); err != nil {
		c.logger.Error("HandleRoleChanged failed; leaving uncommitted", zap.Error(err))
		return
	}
	c.commit(ctx, m)
}

func (c *AuthConsumer) handleStatusChanged(ctx context.Context, m kafka.Message, event IncomingKafkaEvent) {
	var p StatusChangedPayload
	if err := json.Unmarshal([]byte(event.EventPayload), &p); err != nil {
		c.logger.Error("Failed to parse STATUS_CHANGED payload", zap.Error(err))
		c.commit(ctx, m)
		return
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		c.logger.Error("Invalid user_id", zap.Error(err))
		c.commit(ctx, m)
		return
	}
	if err := c.handleUserUC.HandleStatusChanged(ctx, userID, p.NewStatus); err != nil {
		c.logger.Error("HandleStatusChanged failed; leaving uncommitted", zap.Error(err))
		return
	}
	c.commit(ctx, m)
}

func (c *AuthConsumer) commit(ctx context.Context, m kafka.Message) {
	if err := c.reader.CommitMessages(ctx, m); err != nil {
		c.logger.Error("Failed to commit kafka message", zap.Error(err))
	}
}

func (c *AuthConsumer) Close() error {
	return c.reader.Close()
}
