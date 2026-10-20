package application

import (
	"context"
	"echo-ride/pkg-library/errs"
	"echo-ride/services/ride-service/internal/domain"
	"encoding/json"
	"errors"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const arrivalGeofenceMeters = 150.0

type ArriveRideCommand struct {
	RideID    uuid.UUID
	DriverID  uuid.UUID
	DriverLat float64
	DriverLng float64
}

type ArriveRideUseCase interface {
	Execute(ctx context.Context, cmd ArriveRideCommand) error
}

type arriveRideUC struct {
	repo   domain.RideRepository
	logger *zap.Logger
	tracer trace.Tracer
}

func NewArriveRideUseCase(repo domain.RideRepository, logger *zap.Logger) ArriveRideUseCase {
	return &arriveRideUC{
		repo:   repo,
		logger: logger,
		tracer: otel.Tracer("ride-service-uc"),
	}
}

func (a *arriveRideUC) Execute(ctx context.Context, cmd ArriveRideCommand) error {
	ctx, span := a.tracer.Start(ctx, "UseCase.ArriveRide")
	defer span.End()

	ride, err := a.repo.GetByID(ctx, cmd.RideID)
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrNotFound.WithMessage("Ride not found")
		}
		return errs.ErrInternal.WithMessage("Failed to load ride").WithRootErr(err)
	}

	if ride.DriverID == nil || *ride.DriverID != cmd.DriverID {
		return errs.ErrForbidden.WithMessage("Driver is not assigned to this ride")
	}
	if ride.Status != domain.RideStatusAccepted {
		return errs.ErrConflict.WithMessage("Ride must be in ACCEPTED state to mark arrival")
	}

	distance := haversineMeters(cmd.DriverLat, cmd.DriverLng, ride.PickupLat, ride.PickupLng)
	if distance > arrivalGeofenceMeters {
		return errs.ErrBadRequest.WithMessage("Too far from pickup location")
	}

	payload := map[string]interface{}{
		"ride_id":   cmd.RideID.String(),
		"rider_id":  ride.RiderID.String(),
		"driver_id": cmd.DriverID.String(),
		"status":    string(domain.EventTypeDriverArrived),
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		span.RecordError(err)
		return errs.ErrInternal.WithMessage("Failed to marshal event payload").WithRootErr(err)
	}

	if err := a.repo.CreateOutboxEventOnly(ctx,
		cmd.RideID.String(),
		string(domain.OutboxStateRide),
		string(domain.EventTypeDriverArrived),
		payloadBytes,
	); err != nil {
		span.RecordError(err)
		return errs.ErrInternal.WithMessage("Failed to publish DRIVER_ARRIVED event").WithRootErr(err)
	}

	a.logger.Info("Driver arrived at pickup",
		zap.String("ride_id", cmd.RideID.String()),
		zap.String("driver_id", cmd.DriverID.String()),
		zap.Float64("distance_m", distance))
	return nil
}

func haversineMeters(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusMeters = 6371000.0
	toRad := func(d float64) float64 { return d * math.Pi / 180 }

	dLat := toRad(lat2 - lat1)
	dLng := toRad(lng2 - lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusMeters * c
}
