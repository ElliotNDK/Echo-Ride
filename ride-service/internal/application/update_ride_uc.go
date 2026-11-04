package application

import (
	"context"
	"echo-ride/pkg-library/errs"
	"echo-ride/services/ride-service/internal/domain"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type UpdateRideUseCase interface {
	UpdateStatus(ctx context.Context, rideID uuid.UUID, newStatus domain.RideStatus) (*domain.Ride, error)
	Execute(ctx context.Context, rideID uuid.UUID, newStatus domain.RideStatus) error
}

type udpateRideUseCase struct {
	repo   domain.RideRepository
	logger *zap.Logger
	tracer trace.Tracer
}

func NewUdpateRideUseCase(repo domain.RideRepository, logger *zap.Logger) UpdateRideUseCase {
	return &udpateRideUseCase{
		repo:   repo,
		logger: logger,
		tracer: otel.Tracer("ride-service-uc"),
	}
}

func (u *udpateRideUseCase) UpdateStatus(ctx context.Context, rideID uuid.UUID, newStatus domain.RideStatus) (*domain.Ride, error) {
	currentRide, err := u.repo.GetByID(ctx, rideID)
	if err != nil {
		return nil, errs.ErrNotFound.WithMessage("Ride not found").WithRootErr(err)
	}

	if !isValidStatusTransition(currentRide.Status, newStatus) {
		return nil, errs.ErrBadRequest.WithMessage(string("Invalid state transition from " + currentRide.Status + " to " + newStatus))
	}

	currentRide.Status = newStatus

	payloadBytes, err := json.Marshal(currentRide)
	if err != nil {
		return nil, errs.ErrInternal.WithMessage("Failed to marshal event payload").WithRootErr(err)
	}

	updatedRide, err := u.repo.UpdateStatus(ctx, rideID, newStatus, "RIDE_STATUS_UPDATED", payloadBytes)
	if err != nil {
		return nil, errs.ErrBadRequest.WithMessage("Failed to update ride status").WithRootErr(err)
	}

	return updatedRide, nil
}

func isValidStatusTransition(current, new domain.RideStatus) bool {
	transitions := map[domain.RideStatus]map[domain.RideStatus]bool{
		domain.RideStatusRequested:  {domain.RideStatusCancelled: true},
		domain.RideStatusAccepted:   {domain.RideStatusInProgress: true, domain.RideStatusCancelled: true},
		domain.RideStatusInProgress: {domain.RideStatusCompleted: true, domain.RideStatusCancelled: true},
		domain.RideStatusCompleted:  {},
		domain.RideStatusCancelled:  {},
	}

	validNextStates, exists := transitions[current]
	if !exists {
		return false
	}

	return validNextStates[new]
}

func (u *udpateRideUseCase) Execute(ctx context.Context, rideID uuid.UUID, newStatus domain.RideStatus) error {
	ctx, span := u.tracer.Start(ctx, "UseCase.UpdateRideStatus")
	defer span.End()

	ride, err := u.repo.GetByID(ctx, rideID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			u.logger.Warn("Ride not found, skipping status update", zap.String("ride_id", rideID.String()))
			return nil
		}
		span.RecordError(err)
		return errs.ErrInternal.WithMessage("Failed to load ride").WithRootErr(err)
	}
	if ride == nil {
		u.logger.Warn("Ride not found, skipping status update", zap.String("ride_id", rideID.String()))
		return nil
	}

	_, err = u.repo.UpdateStatus(ctx, rideID, newStatus, "", nil)
	if err != nil {
		span.RecordError(err)
		return errs.ErrInternal.WithMessage("Failed to update ride status").WithRootErr(err)
	}

	u.logger.Info("Ride status updated successfully",
		zap.String("ride_id", rideID.String()),
		zap.String("new_status", string(newStatus)))

	return nil
}
