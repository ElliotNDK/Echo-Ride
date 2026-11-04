package kafka

import (
	"context"
	"echo-ride/services/user-service/config"
	"echo-ride/services/user-service/internal/application"
	"echo-ride/services/user-service/internal/domain"
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
	EventType    domain.EventType  `json:"event_type"`
	EventPayload string            `json:"event_payload"`
	TraceContext map[string]string `json:"trace_context,omitempty"`
}

type IdentityCreatedPayload struct {
	IdentityID string `json:"identity_id"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	Role       string `json:"role"`
}

type UserConsumer struct {
	reader      *kafka.Reader
	upsertUC    application.UpsertUserUseCase
	logger      *zap.Logger
	tracer      trace.Tracer
}

func NewUserConsumer(cfg config.KafkaConfig, identityTopic string, upsertUC application.UpsertUserUseCase, logger *zap.Logger) *UserConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  cfg.Brokers,
		Topic:    identityTopic,
		GroupID:  "user-service-consumer",
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})

	return &UserConsumer{
		reader:   reader,
		upsertUC: upsertUC,
		logger:   logger,
		tracer:   otel.Tracer("user-service-consumer"),
	}
}

func (c *UserConsumer) Start(ctx context.Context) {
	c.logger.Info("User Service Consumer started", zap.String("topic", c.reader.Config().Topic))

	for {
		m, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				c.logger.Info("User Service Consumer context canceled")
				return
			}
			c.logger.Error("Failed to fetch message", zap.Error(err))
			continue
		}
		c.processMessage(ctx, m)
	}
}

func (c *UserConsumer) processMessage(ctx context.Context, m kafka.Message) {
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

	if event.EventType != domain.EventTypeIdentityCreated {
		c.commit(ctx, m)
		return
	}

	parentCtx := otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(event.TraceContext))
	spanCtx, span := c.tracer.Start(parentCtx, "Kafka Consume "+string(event.EventType), trace.WithSpanKind(trace.SpanKindConsumer))
	defer span.End()

	c.handleIdentityCreated(spanCtx, m, event)
}

func (c *UserConsumer) handleIdentityCreated(ctx context.Context, m kafka.Message, event IncomingKafkaEvent) {
	var payload IdentityCreatedPayload
	if err := json.Unmarshal([]byte(event.EventPayload), &payload); err != nil {
		c.logger.Error("Failed to parse IDENTITY_CREATED payload", zap.Error(err), zap.String("payload", event.EventPayload))
		c.commit(ctx, m)
		return
	}

	identityID, err := uuid.Parse(payload.IdentityID)
	if err != nil {
		c.logger.Error("Invalid identity_id UUID", zap.Error(err), zap.String("identity_id", payload.IdentityID))
		c.commit(ctx, m)
		return
	}

	role := domain.AccountRole(payload.Role)
	if !role.IsValid() {
		role = domain.RoleRider
	}

	_, err = c.upsertUC.Execute(ctx, application.UpsertUserRequest{
		UserID: identityID,
		Email:  payload.Email,
		Phone:  payload.Phone,
		Role:   role,
	})
	if err != nil {
		c.logger.Error("Failed to upsert user from IDENTITY_CREATED; leaving message uncommitted",
			zap.Error(err), zap.String("identity_id", identityID.String()))
		return
	}

	c.commit(ctx, m)
	c.logger.Info("Upserted user from IDENTITY_CREATED", zap.String("identity_id", identityID.String()))
}

func (c *UserConsumer) commit(ctx context.Context, m kafka.Message) {
	if err := c.reader.CommitMessages(ctx, m); err != nil {
		c.logger.Error("Failed to commit kafka message", zap.Error(err))
	}
}

func (c *UserConsumer) Close() error {
	return c.reader.Close()
}
