package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type RideStatus string
type UserStatus string

const (
	NewRideStatusAssigned RideStatus = "NEW_RIDE_ASSIGNED"
)

const (
	DriverStatusMatched      UserStatus = "DRIVER_MATCHED"
	RideAcceptStatusAccepted UserStatus = "RIDE_ACCEPTED"
	DriverArrivedStatus      UserStatus = "DRIVER_ARRIVED"
	InProgressStatus         UserStatus = "IN_PROGRESS"
	CompletedStatus          UserStatus = "COMPLETED"
	CancelledStatus          UserStatus = "CANCELLED"
)

type DriverLocation struct {
	RideID     uuid.UUID `json:"ride_id"`
	DriverID   uuid.UUID `json:"driver_id"`
	Lat        float64   `json:"lat"`
	Lng        float64   `json:"lng"`
	Bearing    float32   `json:"bearing,omitempty"`
	Speed      float32   `json:"speed,omitempty"`
	DistanceKm float64   `json:"distance_km,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type LocationRequest struct {
	RideID uuid.UUID
	Lat    float64
	Lng    float64
	Radius float64
	Limit  int
}

type DriverETA struct {
	DriverID uuid.UUID
	Lat      float64
	Lng      float64
	ETA      float64
	Distance float64
}

type ActiveDriverRepository interface {
	UpsertActiveLocation(ctx context.Context, locations []DriverLocation) error
	RemoveStaleDrivers(ctx context.Context, olderThan time.Time) (int64, error)
	FindNearestDrivers(ctx context.Context, lat, lng, radius float64, limit int) ([]DriverLocation, error)
	FindDriversInRadius(ctx context.Context, lat, lng, radius float64, limit int) ([]DriverLocation, error)
}

// LocationIngester buffers incoming GPS pings and flushes them in batches to
// both Cassandra (history) and Redis GEO (active driver index). Implemented by
// LocationBatcher. Interface lives here to avoid import cycles with application.
type LocationIngester interface {
	Push(loc DriverLocation)
}

type LocationHistoryRepository interface {
	SaveLocationBatch(ctx context.Context, locations []DriverLocation) error
}

type RoutingService interface {
	CalculateETAMatrix(ctx context.Context, originLat, originLng float64, destinations []DriverLocation) ([]DriverETA, error)
}
