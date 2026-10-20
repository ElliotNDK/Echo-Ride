package application

import (
	"context"
	"echo-ride/pkg-library/errs"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type UserNotifier interface {
	NotifyUser(ctx context.Context, userID uuid.UUID, messageType string, payload interface{}) error
}

type NotifyUserUseCase interface {
	Execute(ctx context.Context, userID uuid.UUID, messageType string, payload interface{}) error
}

type notifyUserUC struct {
	notifier UserNotifier
	logger   *zap.Logger
	tracer   trace.Tracer
}

func NewNotifyDriverUseCase(notifier UserNotifier, logger *zap.Logger) NotifyUserUseCase {
	return &notifyUserUC{
		notifier: notifier,
		logger:   logger,
		tracer:   otel.Tracer("location-service-uc"),
	}
}

func (n *notifyUserUC) Execute(ctx context.Context, userID uuid.UUID, messageType string, payload interface{}) error {
	ctx, span := n.tracer.Start(ctx, "UseCase.NotifyDriver")
	defer span.End()

	n.logger.Info("Notifying user", zap.String("userID", userID.String()), zap.String("type", messageType))

	if err := n.notifier.NotifyUser(ctx, userID, messageType, payload); err != nil {
		span.RecordError(err)
		n.logger.Error("Failed to notify user", zap.String("userID", userID.String()), zap.Error(err))
		return errs.ErrInternal.WithMessage("Failed to notify driver of matched ride").WithRootErr(err)
	}

	return nil
}
