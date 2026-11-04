package application

import (
	"context"
	"echo-ride/services/matching-service/internal/domain"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type HandleRideAcceptedUseCase interface {
	Execute(ctx context.Context, rideID string) error
}

type handleRideAcceptedUC struct {
	dispatchRepo domain.DispatchRepository
	logger       *zap.Logger
	tracer       trace.Tracer
}

func NewHandleRideAcceptedUseCase(dispatchRepo domain.DispatchRepository, logger *zap.Logger) HandleRideAcceptedUseCase {
	return &handleRideAcceptedUC{
		dispatchRepo: dispatchRepo,
		logger:       logger,
		tracer:       otel.Tracer("matching-service-uc"),
	}
}

func (h *handleRideAcceptedUC) Execute(ctx context.Context, rideID string) error {
	ctx, span := h.tracer.Start(ctx, "UseCase.HandleRideAccepted")
	defer span.End()

	h.logger.Info("Handling ride accepted event", zap.String("rideID", rideID))

	if err := h.dispatchRepo.RemoveTimeout(ctx, rideID); err != nil {
		span.RecordError(err)
		h.logger.Error("Failed to remove timeout for accepted ride", zap.String("rideID", rideID), zap.Error(err))
	}

	if err := h.dispatchRepo.DeleteState(ctx, rideID); err != nil {
		span.RecordError(err)
		h.logger.Error("Failed to delete dispatch state for accepted ride", zap.String("rideID", rideID), zap.Error(err))
		return err
	}
	
	h.logger.Info("Successfully handled ride accepted event", zap.String("rideID", rideID))
	return nil
}
