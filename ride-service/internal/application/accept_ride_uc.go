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

type AcceptRideUseCase interface {
	Execute(ctx context.Context, rideID, driverID uuid.UUID) (*domain.Ride, error)
}

type acceptRideUC struct {
	repo   domain.RideRepository
	logger *zap.Logger
	tracer trace.Tracer
}

func NewAcceptRideUseCase(repo domain.RideRepository, logger *zap.Logger) AcceptRideUseCase {
	return &acceptRideUC{
		repo:   repo,
		logger: logger,
		tracer: otel.Tracer("ride-service-uc"),
	}
}

func (a *acceptRideUC) Execute(ctx context.Context, rideID, driverID uuid.UUID) (*domain.Ride, error) {
	ctx, span := a.tracer.Start(ctx, "UseCase.AcceptRide")
	defer span.End()

	a.logger.Info("Attempting to accept ride", zap.String("rideID", rideID.String()), zap.String("driverID", driverID.String()))

	ride, err := a.repo.GetByID(ctx, rideID)
	if err != nil || ride == nil {
		return nil, errs.ErrNotFound.WithMessage("Ride not found").WithRootErr(err)
	}

	payload := map[string]interface{}{
		"ride_id":   rideID.String(),
		"rider_id":  ride.RiderID.String(),
		"driver_id": driverID.String(),
		"status":    string(domain.RideStatusAccepted),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		span.RecordError(err)
		a.logger.Error("Failed to marshal event payload", zap.String("rideID", rideID.String()), zap.String("driverID", driverID.String()), zap.Error(err))
		return nil, errs.ErrInternal.WithMessage("Failed to marshal event payload").WithRootErr(err)
	}

	acceptedRide, err := a.repo.AcceptRide(ctx, rideID, driverID, string(domain.EventTypeRideAccepted), payloadBytes)
	if err != nil {
		a.logger.Error("Failed to accept ride", zap.String("rideID", rideID.String()), zap.String("driverID", driverID.String()), zap.Error(err))
		span.RecordError(err)
		return nil, errs.ErrConflict.WithMessage("Failed to accept ride - it may have already been accepted by another driver").WithRootErr(err)
	}

	a.logger.Info("Successfully accepted ride", zap.String("rideID", rideID.String()), zap.String("driverID", driverID.String()))
	return acceptedRide, nil
}
