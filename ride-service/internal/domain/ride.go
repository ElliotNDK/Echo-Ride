package domain

import (
	"context"

	"github.com/google/uuid"
)

type RideStatus string
type EventType string
type OutboxState string

const (
	RideStatusRequested  RideStatus = "REQUESTED"
	RideStatusAccepted   RideStatus = "ACCEPTED"
	RideStatusInProgress RideStatus = "IN_PROGRESS"
	RideStatusCompleted  RideStatus = "COMPLETED"
	RideStatusCancelled  RideStatus = "CANCELLED"
	RideStatusFailed     RideStatus = "FAILED"
)

const (
	OutboxStateRide OutboxState = "RIDE"
)

const (
	EventTypeRideRequested  EventType = "RIDE_REQUESTED"
	EventTypeRideAccepted   EventType = "RIDE_ACCEPTED"
	EventTypeDriverArrived  EventType = "DRIVER_ARRIVED"
	EventTypeRideInProgress EventType = "IN_PROGRESS"
	EventTypeRideCompleted  EventType = "COMPLETED"
	EventTypeRideCancelled  EventType = "RIDE_CANCELLED"
	EventTypeRideFailed     EventType = "RIDE_FAILED"
	EventTypeRideDeclined   EventType = "RIDE_DECLINED"
)

type Ride struct {
	ID         uuid.UUID  `json:"id"`
	RiderID    uuid.UUID  `json:"rider_id"`
	DriverID   *uuid.UUID `json:"driver_id,omitempty"` // pointer to allow null value when no driver assigned
	PickupLat  float64    `json:"pickup_lat"`
	PickupLng  float64    `json:"pickup_lng"`
	DropoffLat float64    `json:"dropoff_lat"`
	DropoffLng float64    `json:"dropoff_lng"`
	Status     RideStatus `json:"status"`
	Price      float64    `json:"price"`
}

type RideFilter struct {
	RiderID  *uuid.UUID
	DriverID *uuid.UUID
	Status   *string // optional filter by status (e.g., "requested", "accepted", "completed") why pointer because we want to distinguish between "no filter" (nil) and "filter by empty string" ("")
	Limit    int32
	Offset   int32
}

type RideRepository interface {
	Create(ctx context.Context, ride *Ride, eventType string, eventPayload []byte) (*Ride, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Ride, error)
	ListRides(ctx context.Context, filter RideFilter) ([]*Ride, error)
	AcceptRide(ctx context.Context, rideID, driverID uuid.UUID, eventType string, eventPayload []byte) (*Ride, error)
	UpdateStatus(ctx context.Context, rideID uuid.UUID, status RideStatus, eventType string, eventPayload []byte) (*Ride, error)
	UpdateTripStatus(ctx context.Context, rideID, driverID uuid.UUID, oldStatus, newStatus RideStatus, eventType string, eventPayload []byte) (*Ride, error)
	// CancelRide atomically transitions a ride to CANCELLED (CAS over the set
	// of cancellable states) and writes the matching outbox event in the same
	// transaction. Returns the updated ride, or an error wrapping pgx.ErrNoRows
	// if the ride is not in a cancellable state.
	CancelRide(ctx context.Context, rideID uuid.UUID, eventType string, eventPayload []byte) (*Ride, error)
	CreateOutboxEventOnly(ctx context.Context, aggregateID, aggregateType, eventType string, eventPayload []byte) error
}
