package application

import (
	"context"
	"echo-ride/pkg-library/errs"
	"echo-ride/services/location-service/internal/domain"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// LocationBroadcaster publishes a driver's current location to the realtime
// fan-out channel (Redis Pub/Sub → rider WebSocket). Implemented by ws.Hub.
// Defined in the application package to break the import cycle:
//
//	ws → application (already) and application → ws (would be circular).
type LocationBroadcaster interface {
	BroadcastDriverLocation(ctx context.Context, loc domain.DriverLocation)
}

// RideTrackingSubscriber wires a rider's WebSocket connection to the
// Redis Pub/Sub channel for their active ride. Implemented by ws.Hub.
// Defined here for the same cycle-breaking reason as LocationBroadcaster.
type RideTrackingSubscriber interface {
	SubscribeToRideTracking(ctx context.Context, riderID uuid.UUID, rideID string)
}

type UpdateLocationCommand struct {
	DriverID uuid.UUID
	RideID   uuid.UUID // zero UUID = driver is idle, no active trip
	Lat      float64
	Lng      float64
	Bearing  float32
	Speed    float32
}

type UpdateDriverLocationUseCase interface {
	Execute(ctx context.Context, cmd UpdateLocationCommand) error
}

type updateDriverLocationUC struct {
	ingester    domain.LocationIngester
	broadcaster LocationBroadcaster
	logger      *zap.Logger
	tracer      trace.Tracer
}

func NewUpdateDriverLocationUseCase(
	ingester domain.LocationIngester,
	broadcaster LocationBroadcaster,
	logger *zap.Logger,
) UpdateDriverLocationUseCase {
	return &updateDriverLocationUC{
		ingester:    ingester,
		broadcaster: broadcaster,
		logger:      logger,
		tracer:      otel.Tracer("location-service-uc"),
	}
}

func (u *updateDriverLocationUC) Execute(ctx context.Context, cmd UpdateLocationCommand) error {
	ctx, span := u.tracer.Start(ctx, "UseCase.UpdateDriverLocation")
	defer span.End()

	if err := validateLocationCmd(cmd); err != nil {
		span.RecordError(err)
		return errs.ErrInvalidArgument.WithMessage(err.Error())
	}

	loc := domain.DriverLocation{
		RideID:    cmd.RideID,
		DriverID:  cmd.DriverID,
		Lat:       cmd.Lat,
		Lng:       cmd.Lng,
		Bearing:   cmd.Bearing,
		Speed:     cmd.Speed,
		CreatedAt: time.Now(),
	}

	// Async batch: Cassandra history + Redis GEO (for FindNearestDrivers).
	// Push is non-blocking; the batcher owns retry / flush logic.
	u.ingester.Push(loc)

	// Realtime fan-out: publish to Redis Pub/Sub only when driver is on an
	// active trip. Idle pings still update GEO but there is no rider to push to.
	if cmd.RideID != (uuid.UUID{}) {
		u.broadcaster.BroadcastDriverLocation(ctx, loc)
	}

	u.logger.Debug("Driver location updated",
		zap.String("driver_id", cmd.DriverID.String()),
		zap.Float64("lat", cmd.Lat),
		zap.Float64("lng", cmd.Lng),
		zap.Float32("bearing", cmd.Bearing),
		zap.Float32("speed", cmd.Speed),
	)

	return nil
}

// validateLocationCmd guards against invalid or obviously spoofed GPS data.
// Speed > 300 km/h is physically impossible for a ride-hailing vehicle.
func validateLocationCmd(cmd UpdateLocationCommand) error {
	if cmd.DriverID == (uuid.UUID{}) {
		return fmt.Errorf("driver_id is required")
	}
	if cmd.Lat < -90 || cmd.Lat > 90 {
		return fmt.Errorf("lat out of range: %.6f", cmd.Lat)
	}
	if cmd.Lng < -180 || cmd.Lng > 180 {
		return fmt.Errorf("lng out of range: %.6f", cmd.Lng)
	}
	if cmd.Bearing < 0 || cmd.Bearing > 360 {
		return fmt.Errorf("bearing out of range: %.1f", cmd.Bearing)
	}
	if cmd.Speed < 0 || cmd.Speed > 300 {
		return fmt.Errorf("speed out of range (km/h): %.1f", cmd.Speed)
	}
	return nil
}
