package application

import (
	"context"
	"echo-ride/pkg-library/errs"
	"echo-ride/services/matching-service/internal/domain"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type ProcessMatchingUseCase interface {
	Execute(ctx context.Context, event domain.RideRequestEvent) error
}

type processMatchingUC struct {
	dispatchRepo    domain.DispatchRepository
	locationGateway domain.LocationGateway
	publisher       domain.MatchingEventPublisher
	logger          *zap.Logger
	tracer          trace.Tracer
}

func NewProcessMatchingUseCase(
	repo domain.DispatchRepository,
	gateway domain.LocationGateway,
	publisher domain.MatchingEventPublisher,
	logger *zap.Logger,
) ProcessMatchingUseCase {
	return &processMatchingUC{
		dispatchRepo:    repo,
		locationGateway: gateway,
		publisher:       publisher,
		logger:          logger,
		tracer:          otel.Tracer("matching-service-uc"),
	}
}

func (p *processMatchingUC) Execute(ctx context.Context, event domain.RideRequestEvent) error {
	ctx, span := p.tracer.Start(ctx, "UseCase.ProcessMatching")
	defer span.End()

	p.logger.Info("Starting matching process", zap.String("rideID", event.RideID))

	candidates, err := p.locationGateway.GetNearestDrivers(ctx, event.RideID, event.PickupLat, event.PickupLng, 5.0, 5)
	if err != nil {
		span.RecordError(err)
		return errs.ErrInternal.WithMessage("Failed to get nearest drivers").WithRootErr(err)
	}

	if len(candidates) == 0 {
		p.logger.Info("No drivers found nearby", zap.String("rideID", event.RideID))
		_ = p.publisher.PublishMatchingFailed(ctx, event.RideID)
		return nil
	}

	state := domain.RideDispatchState{
		RideID:       event.RideID,
		Candidates:   candidates,
		CurrentIndex: 0,
		Status:       domain.RideStatusFinding,
		UpdatedAt:    time.Now().Unix(),
	}

	if err := p.dispatchRepo.SaveState(ctx, state); err != nil {
		return errs.ErrInternal.WithMessage("Failed to save dispatch state").WithRootErr(err)
	}

	firstDriver := candidates[0]
	p.logger.Info("Publishing RideMatchingStarted event", zap.String("rideID", event.RideID), zap.String("driverID", firstDriver.DriverID))
	if err := p.publisher.PublishDriverMatched(ctx, event.RideID, firstDriver.DriverID); err != nil {
		p.logger.Error("Failed to publish DRIVER_MATCHED event", zap.Error(err))
	}

	expireAt := time.Now().Add(time.Second * 15).Unix()
	if err := p.dispatchRepo.SetTimeout(ctx, event.RideID, expireAt); err != nil {
		return errs.ErrInternal.WithMessage("Failed to set timeout for ride").WithRootErr(err)
	}

	return nil
}
