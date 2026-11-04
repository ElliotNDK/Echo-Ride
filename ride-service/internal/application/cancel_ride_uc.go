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

type CancelledBy string

const (
	CancelledByRider  CancelledBy = "RIDER"
	CancelledByDriver CancelledBy = "DRIVER"
)

type CanceledRideUC interface {
	// Execute cancels a ride performed by the given actor. driverID is required
	// when cancelledBy=DRIVER (used for authorization: a driver may only cancel
	// a ride they were assigned to).
	Execute(ctx context.Context, rideID uuid.UUID, actorID uuid.UUID, cancelledBy CancelledBy) error
}

type canceledRideUC struct {
	repo   domain.RideRepository
	logger *zap.Logger
	tracer trace.Tracer
}

func NewCancelRideUseCase(repo domain.RideRepository, logger *zap.Logger) CanceledRideUC {
	return &canceledRideUC{
		repo:   repo,
		logger: logger,
		tracer: otel.Tracer("ride-service-uc"),
	}
}

func (c *canceledRideUC) Execute(ctx context.Context, rideID uuid.UUID, actorID uuid.UUID, cancelledBy CancelledBy) error {
	ctx, span := c.tracer.Start(ctx, "UseCase.CancelRide")
	defer span.End()

	if cancelledBy != CancelledByRider && cancelledBy != CancelledByDriver {
		return errs.ErrBadRequest.WithMessage("Invalid cancelledBy actor")
	}

	// Authorization: must read the ride first to verify the actor has standing.
	// This costs one extra round-trip vs. doing it all in SQL, but keeps the
	// CAS UPDATE simple and gives us better error messages.
	ride, err := c.repo.GetByID(ctx, rideID)
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrNotFound.WithMessage("Ride not found")
		}
		return errs.ErrInternal.WithMessage("Failed to load ride").WithRootErr(err)
	}

	switch cancelledBy {
	case CancelledByRider:
		if ride.RiderID != actorID {
			return errs.ErrForbidden.WithMessage("Rider can only cancel their own ride")
		}
	case CancelledByDriver:
		if ride.DriverID == nil || *ride.DriverID != actorID {
			return errs.ErrForbidden.WithMessage("Driver can only cancel a ride assigned to them")
		}
	}

	// Idempotency: if the ride is already in a terminal state, treat the
	// request as a successful no-op rather than an error. This makes retries
	// from clients safe.
	if ride.Status == domain.RideStatusCancelled || ride.Status == domain.RideStatusCompleted ||
		ride.Status == domain.RideStatusFailed {
		c.logger.Info("Cancel is a no-op (already terminal)",
			zap.String("ride_id", rideID.String()), zap.String("status", string(ride.Status)))
		return nil
	}

	driverIDStr := ""
	if ride.DriverID != nil {
		driverIDStr = ride.DriverID.String()
	}

	payload := map[string]interface{}{
		"ride_id":      rideID.String(),
		"rider_id":     ride.RiderID.String(),
		"driver_id":    driverIDStr,
		"status":       string(domain.RideStatusCancelled),
		"cancelled_by": string(cancelledBy),
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		span.RecordError(err)
		return errs.ErrInternal.WithMessage("Failed to marshal event payload").WithRootErr(err)
	}

	if _, err := c.repo.CancelRide(ctx, rideID, string(domain.EventTypeRideCancelled), payloadBytes); err != nil {
		span.RecordError(err)
		// pgx.ErrNoRows wrapped by the repo means a concurrent transition won
		// the CAS. Treat as conflict so the client can re-fetch.
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrConflict.WithMessage("Ride is no longer cancellable")
		}
		c.logger.Error("Failed to cancel ride", zap.Error(err), zap.String("ride_id", rideID.String()))
		return errs.ErrInternal.WithMessage("Failed to cancel ride").WithRootErr(err)
	}

	c.logger.Info("Ride cancelled",
		zap.String("ride_id", rideID.String()),
		zap.String("cancelled_by", string(cancelledBy)),
		zap.String("actor_id", actorID.String()))
	return nil
}
