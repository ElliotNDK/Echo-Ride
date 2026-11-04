package http

import (
	"echo-ride/services/ride-service/internal/domain"

	"github.com/google/uuid"
)

type createRideRequest struct {
	PickupLat  float64 `json:"pickup_lat" validate:"required"`
	PickupLon  float64 `json:"pickup_lon" validate:"required"`
	DropoffLat float64 `json:"dropoff_lat" validate:"required"`
	DropoffLon float64 `json:"dropoff_lon" validate:"required"`
}

type createRideResponse struct {
	RideID uuid.UUID         `json:"ride_id"`
	Status domain.RideStatus `json:"status"`
	Price  float64           `json:"price"`
}

type acceptRideRequest struct {
	DriverID string `json:"driver_id" validate:"required,uuid"`
}

type updateStatusRequest struct {
	Status domain.RideStatus `json:"status" validate:"required,oneof=IN_PROGRESS COMPLETED CANCELED"`
}

type arriveRideRequest struct {
	Lat float64 `json:"lat" validate:"required"`
	Lng float64 `json:"lng" validate:"required"`
}
