package domain

import (
	"context"
)

type RideDispatchStatus string
type RideEventStatus string
type CancelledBy string

const (
	CancelledByRider  CancelledBy = "RIDER"
	CancelledByDriver CancelledBy = "DRIVER"
)

const (
	RideStatusFinding  RideDispatchStatus = "FINDING"
	RideStatusAccepted RideDispatchStatus = "ACCEPTED"
	RideStatusFailed   RideDispatchStatus = "FAILED"
)

const (
	RideEventStatusPending    RideEventStatus = "PENDING"
	RideEventStatusPublished  RideEventStatus = "PUBLISHED"
	RideEventStatusFailed     RideEventStatus = "RIDE_FAILED"
	RideEventStatusRequested  RideEventStatus = "RIDE_REQUESTED"
	RideEventStatusAccepted   RideEventStatus = "RIDE_ACCEPTED"
	RideEventStatusInProgress RideEventStatus = "IN_PROGRESS"
	RideEventStatusCompleted  RideEventStatus = "RIDE_COMPLETED"
	RideEventStatusCancelled  RideEventStatus = "RIDE_CANCELLED"
	RideEventStatusDeclined   RideEventStatus = "RIDE_DECLINED"
	RideDriverMatched         RideEventStatus = "DRIVER_MATCHED"
)

type CandidateDriver struct {
	DriverID   string
	Lat        float64
	Lng        float64
	DistanceKm float64
	Score      float64
}

type RideRequestEvent struct {
	RideID    string  `json:"id"`
	RiderID   string  `json:"rider_id"`
	PickupLat float64 `json:"pickup_lat"`
	PickupLng float64 `json:"pickup_lng"`
}

type RideDispatchState struct {
	RideID       string             `json:"ride_id"`
	Candidates   []CandidateDriver  `json:"candidates"`
	CurrentIndex int                `json:"current_index"`
	Status       RideDispatchStatus `json:"status"` // FINDING, ACCEPTED, FAILED
	UpdatedAt    int64              `json:"updated_at"`
}

type LocationGateway interface {
	GetNearestDrivers(ctx context.Context, rideID string, lat, lng, radiusKm float64, limit int) ([]CandidateDriver, error)
}

type DispatchRepository interface {
	SaveState(ctx context.Context, state RideDispatchState) error
	GetState(ctx context.Context, rideID string) (*RideDispatchState, error)
	DeleteState(ctx context.Context, rideID string) error
	SetTimeout(ctx context.Context, rideID string, expireAt int64) error
	GetExpiredRides(ctx context.Context, now int64, limit int) ([]string, error)
	RemoveTimeout(ctx context.Context, rideID string) error
	// WasProcessed checks (without writing) whether an event has already been
	// successfully handled. Used together with MarkProcessed to ensure the
	// idempotency token is only persisted *after* the handler succeeds.
	WasProcessed(ctx context.Context, eventID string) (bool, error)
	MarkProcessed(ctx context.Context, eventID string) error
}
