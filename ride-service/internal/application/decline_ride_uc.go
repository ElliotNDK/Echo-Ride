package application

import (
	"context"
	"echo-ride/pkg-library/errs"
	"echo-ride/services/ride-service/internal/domain"
	"encoding/json"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type DeclineRideUC interface {
	Execute(ctx context.Context, rideID, driverID uuid.UUID) error
}

type declineRideUC struct {
	repo   domain.RideRepository
	logger *zap.Logger
	tracer trace.Tracer
}

func NewDeclineRideUseCase(repo domain.RideRepository, logger *zap.Logger) DeclineRideUC {
	return &declineRideUC{
		repo:   repo,
		logger: logger,
		tracer: otel.Tracer("ride-service-uc"),
	}
}

// Execute handles a driver declining an offered ride during the dispatch phase.
// The ride remains REQUESTED — only an outbox event is inserted so matching
// service can fast-forward to the next candidate via HandleCancelledUseCase
// (cancelledBy=DRIVER). No UPDATE on t_rides intentional: driver was never
// assigned, so there is no state to revert.
func (d *declineRideUC) Execute(ctx context.Context, rideID, driverID uuid.UUID) error {
	ctx, span := d.tracer.Start(ctx, "UseCase.DeclineRide")
	defer span.End()

	payload := map[string]interface{}{
		"ride_id":      rideID.String(),
		"driver_id":    driverID.String(),
		"cancelled_by": "DRIVER",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		span.RecordError(err)
		return errs.ErrInternal.WithMessage("Failed to marshal decline payload").WithRootErr(err)
	}

	if err := d.repo.CreateOutboxEventOnly(
		ctx,
		rideID.String(),
		string(domain.OutboxStateRide),
		string(domain.EventTypeRideDeclined),
		payloadBytes,
	); err != nil {
		span.RecordError(err)
		d.logger.Error("Failed to record ride decline event",
			zap.String("ride_id", rideID.String()),
			zap.String("driver_id", driverID.String()),
			zap.Error(err))
		return errs.ErrInternal.WithMessage("Failed to record ride decline event").WithRootErr(err)
	}

	d.logger.Info("Driver declined ride offer",
		zap.String("ride_id", rideID.String()),
		zap.String("driver_id", driverID.String()))
	return nil
}
