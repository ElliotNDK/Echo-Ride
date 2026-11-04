package application

import (
	"context"
	"echo-ride/services/location-service/internal/domain"
	"fmt"
	"sort"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type FindDriversUseCase interface {
	Execute(ctx context.Context, req domain.LocationRequest) ([]domain.DriverETA, error)
}

type findDriversUseCase struct {
	repo           domain.ActiveDriverRepository
	routingService domain.RoutingService
	tracer         trace.Tracer
}

func NewFindDriversUseCase(repo domain.ActiveDriverRepository, routing domain.RoutingService) FindDriversUseCase {
	return &findDriversUseCase{
		repo:           repo,
		routingService: routing,
		tracer:         otel.Tracer("location-service-uc"),
	}
}

func (f *findDriversUseCase) Execute(ctx context.Context, req domain.LocationRequest) ([]domain.DriverETA, error) {
	ctx, span := f.tracer.Start(ctx, "findDriversUsecase.Execute")
	defer span.End()

	if req.Lat < -90 || req.Lat > 90 || req.Lng < -180 || req.Lng > 180 {
		return nil, fmt.Errorf("invalid coordinates")
	}
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 50
	}
	if req.Radius <= 0 || req.Radius > 10 {
		req.Radius = 5
	}

	nearbyDrivers, err := f.repo.FindDriversInRadius(ctx, req.Lat, req.Lng, req.Radius, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to find nearby drivers: %w", err)
	}

	if len(nearbyDrivers) == 0 {
		return []domain.DriverETA{}, nil
	}

	osrmCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()

	driverETAs, err := f.routingService.CalculateETAMatrix(osrmCtx, req.Lat, req.Lng, nearbyDrivers)
	if err != nil {
		span.AddEvent("OSRM failed or timeout, falling back to distance-based sorting", trace.WithAttributes())
		return f.fallbackToRedisDistance(nearbyDrivers), nil
	}

	sort.Slice(driverETAs, func(i, j int) bool {
		return driverETAs[i].ETA < driverETAs[j].ETA
	})

	returnLimit := 5
	if len(driverETAs) < returnLimit {
		returnLimit = len(driverETAs)
	}

	return driverETAs[:returnLimit], nil
}

func (f *findDriversUseCase) fallbackToRedisDistance(drivers []domain.DriverLocation) []domain.DriverETA {
	var etas []domain.DriverETA
	for _, d := range drivers {
		etas = append(etas, domain.DriverETA{
			DriverID: d.DriverID,
			Lat:      d.Lat,
			Lng:      d.Lng,
			ETA:      d.DistanceKm * 120, // ETA is unknown in fallback, could be estimated based on distance if needed
			Distance: d.DistanceKm,
		})
	}

	return etas
}
