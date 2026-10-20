package application

import (
	"context"
	"echo-ride/pkg-library/errs"
	"echo-ride/services/matching-service/internal/domain"

	"go.uber.org/zap"
)

type HandleTimeoutUseCase interface {
	Execute(ctx context.Context, rideID string, now int64) error
}

type handleTimeoutUC struct {
	dispatchRepo domain.DispatchRepository
	publisher    domain.MatchingEventPublisher
	logger       *zap.Logger
}

func NewHandleTimeoutUseCase(dispatchRepo domain.DispatchRepository, publisher domain.MatchingEventPublisher, logger *zap.Logger) HandleTimeoutUseCase {
	return &handleTimeoutUC{
		dispatchRepo: dispatchRepo,
		publisher:    publisher,
		logger:       logger,
	}
}

func (h *handleTimeoutUC) Execute(ctx context.Context, rideID string, now int64) error {
	_ = h.dispatchRepo.RemoveTimeout(ctx, rideID)

	state, err := h.dispatchRepo.GetState(ctx, rideID)
	if err != nil || state == nil {
		h.logger.Error("failed to fetch state", zap.String("rideID", rideID), zap.Error(err))
		return errs.ErrInternal.WithMessage("failed to fetch state").WithRootErr(err)
	}

	if state.Status != domain.RideStatusFinding {
		h.dispatchRepo.RemoveTimeout(ctx, rideID)
		return nil
	}

	state.CurrentIndex++

	if state.CurrentIndex >= len(state.Candidates) {
		h.logger.Warn("Timeout watcher has expired rides but no more candidates to try", zap.String("rideID", rideID))
		state.Status = domain.RideStatusFailed
		state.UpdatedAt = now
		h.dispatchRepo.SaveState(ctx, *state) // why pointer ? because we need to update the state in redis
		h.dispatchRepo.RemoveTimeout(ctx, rideID)

		_ = h.publisher.PublishMatchingFailed(ctx, rideID)
		return nil
	}

	state.UpdatedAt = now
	if err := h.dispatchRepo.SaveState(ctx, *state); err != nil {
		h.logger.Error("failed to update state for expired ride", zap.String("rideID", rideID), zap.Error(err))
		return errs.ErrInternal.WithMessage("failed to update state for expired ride").WithRootErr(err)
	}

	newExpireAt := now + 15
	if err := h.dispatchRepo.SetTimeout(ctx, rideID, newExpireAt); err != nil {
		h.logger.Error("failed to set new timeout for expired ride", zap.String("rideID", rideID), zap.Error(err))
		return errs.ErrInternal.WithMessage("failed to set new timeout for expired ride").WithRootErr(err)
	}

	newDriver := state.Candidates[state.CurrentIndex]
	h.logger.Info("Timeout watcher is trying next candidate driver", zap.String("rideID", rideID), zap.String("driverID", newDriver.DriverID))

	_ = h.publisher.PublishDriverMatched(ctx, rideID, newDriver.DriverID)
	return nil
}
