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

type StartTripUseCase interface {
	Execute(ctx context.Context, rideID, driverID uuid.UUID) (*domain.Ride, error)
}

type CompleteTripUseCase interface {
	Execute(ctx context.Context, rideID, driverID uuid.UUID) (*domain.Ride, error)
}

type startTripUC struct {
	repo   domain.RideRepository
	logger *zap.Logger
	tracer trace.Tracer
}

type completeTripUC struct {
	repo   domain.RideRepository
	logger *zap.Logger
	tracer trace.Tracer
}

func NewStartTripUseCase(repo domain.RideRepository, logger *zap.Logger) StartTripUseCase {
	return &startTripUC{repo: repo, logger: logger, tracer: otel.Tracer("ride-service-uc")}
}

func NewCompleteTripUseCase(repo domain.RideRepository, logger *zap.Logger) CompleteTripUseCase {
	return &completeTripUC{repo: repo, logger: logger, tracer: otel.Tracer("ride-service-uc")}
}

func (u *startTripUC) Execute(ctx context.Context, rideID, driverID uuid.UUID) (*domain.Ride, error) {
	ctx, span := u.tracer.Start(ctx, "UseCase.StartTrip")
	defer span.End()

	return runTripTransition(ctx, span, u.repo, u.logger, rideID, driverID,
		domain.RideStatusAccepted, domain.RideStatusInProgress, domain.EventTypeRideInProgress)
}

func (u *completeTripUC) Execute(ctx context.Context, rideID, driverID uuid.UUID) (*domain.Ride, error) {
	ctx, span := u.tracer.Start(ctx, "UseCase.CompleteTrip")
	defer span.End()

	return runTripTransition(ctx, span, u.repo, u.logger, rideID, driverID,
		domain.RideStatusInProgress, domain.RideStatusCompleted, domain.EventTypeRideCompleted)
}

// runTripTransition wraps the common shape of start/complete: load the ride
// for ownership check + payload data, then run the CAS UPDATE that enforces
// the (ride_id, driver_id, expected_old_status) precondition atomically.
// The CAS itself is the source of truth — the GetByID is for nicer 4xx
// errors, not authorization (the SQL would also reject a wrong driver).
func runTripTransition(
	ctx context.Context,
	span trace.Span,
	repo domain.RideRepository,
	logger *zap.Logger,
	rideID, driverID uuid.UUID,
	expectedOld, newStatus domain.RideStatus,
	eventType domain.EventType,
) (*domain.Ride, error) {
	ride, err := repo.GetByID(ctx, rideID)
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrNotFound.WithMessage("Ride not found")
		}
		return nil, errs.ErrInternal.WithMessage("Failed to load ride").WithRootErr(err)
	}

	if ride.DriverID == nil || *ride.DriverID != driverID {
		return nil, errs.ErrForbidden.WithMessage("Driver is not assigned to this ride")
	}

	payload := map[string]interface{}{
		"ride_id":   rideID.String(),
		"rider_id":  ride.RiderID.String(),
		"driver_id": driverID.String(),
		"status":    string(newStatus),
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		span.RecordError(err)
		return nil, errs.ErrInternal.WithMessage("Failed to marshal event payload").WithRootErr(err)
	}

	updated, err := repo.UpdateTripStatus(ctx, rideID, driverID, expectedOld, newStatus, string(eventType), payloadBytes)
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrConflict.WithMessage("Ride state changed; cannot apply transition")
		}
		return nil, errs.ErrInternal.WithMessage("Failed to update trip status").WithRootErr(err)
	}

	logger.Info("Trip status transitioned",
		zap.String("ride_id", rideID.String()),
		zap.String("driver_id", driverID.String()),
		zap.String("from", string(expectedOld)),
		zap.String("to", string(newStatus)))
	return updated, nil
}
