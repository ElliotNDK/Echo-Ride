package application

import (
	"context"
	"echo-ride/pkg-library/errs"
	"echo-ride/services/ride-service/internal/domain"
	"encoding/json"

	"github.com/google/uuid"
)

type CreateRideUseCase interface {
	Execute(ctx context.Context, req CreateRideCommand) (*domain.Ride, error)
}

type CreateRideCommand struct {
	RiderID    uuid.UUID
	PickupLat  float64
	PickupLng  float64
	DropoffLat float64
	DropoffLng float64
}

type createRideUseCase struct {
	repo domain.RideRepository
}

func NewCreateRideUseCase(repo domain.RideRepository) CreateRideUseCase {
	return &createRideUseCase{
		repo: repo,
	}
}

func (c *createRideUseCase) Execute(ctx context.Context, req CreateRideCommand) (*domain.Ride, error) {
	if req.PickupLat == req.DropoffLat && req.PickupLng == req.DropoffLng {
		//return nil, domain.ErrInvalidRide
		return nil, errs.ErrSamePickupAndDropoff
	}

	// TODO: implement price calculation based on distance and other factors - dummy price for now
	calculatedPrice := 50000.0

	newRideID := uuid.New()

	newRide := &domain.Ride{
		ID:         newRideID,
		RiderID:    req.RiderID,
		PickupLat:  req.PickupLat,
		PickupLng:  req.PickupLng,
		DropoffLat: req.DropoffLat,
		DropoffLng: req.DropoffLng,
		Price:      calculatedPrice,
		Status:     domain.RideStatusRequested,
	}

	payloadBytes, err := json.Marshal(newRide)
	if err != nil {
		return nil, errs.ErrInternal.WithMessage("Failed to marshal event payload").WithRootErr(err)
	}

	createdRide, err := c.repo.Create(ctx, newRide, "RIDE_REQUESTED", payloadBytes)
	if err != nil {
		return nil, errs.ErrBadRequest
	}

	// TODO: publish event to message broker for ride created - Kafka (RideRequestedEvent)

	return createdRide, nil
}
