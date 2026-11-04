package application

import (
	"context"
	"echo-ride/pkg-library/errs"
	"echo-ride/services/ride-service/internal/domain"

	"github.com/google/uuid"
)

type GetRideUseCase interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Ride, error)
	ListRides(ctx context.Context, filter domain.RideFilter) ([]*domain.Ride, error)
}

type getRideUseCase struct {
	repo domain.RideRepository
}

func NewGetRideUseCase(repo domain.RideRepository) GetRideUseCase {
	return &getRideUseCase{
		repo: repo,
	}
}

func (g getRideUseCase) GetByID(ctx context.Context, id uuid.UUID) (*domain.Ride, error) {
	ride, err := g.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errs.ErrBadRequest.WithMessage("Failed to retrieve ride").WithRootErr(err)
	}

	return ride, nil
}

func (g getRideUseCase) ListRides(ctx context.Context, filter domain.RideFilter) ([]*domain.Ride, error) {
	rides, err := g.repo.ListRides(ctx, filter)
	if err != nil {
		return nil, errs.ErrBadRequest.WithMessage("Failed to list rides").WithRootErr(err)
	}

	return rides, nil
}
