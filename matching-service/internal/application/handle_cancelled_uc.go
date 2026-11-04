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

type HandleCancelledUseCase interface {
	Execute(ctx context.Context, rideID, driverID string, cancelledBy domain.CancelledBy) error
}

type handleRideCancelledUseCase struct {
	dispatchRepo domain.DispatchRepository
	timeoutUC    HandleTimeoutUseCase
	logger       *zap.Logger
	tracer       trace.Tracer
}

func NewHandleCancelledUseCase(dispatchRepo domain.DispatchRepository, timeoutUC HandleTimeoutUseCase, logger *zap.Logger) HandleCancelledUseCase {
	return &handleRideCancelledUseCase{
		dispatchRepo: dispatchRepo,
		timeoutUC:    timeoutUC,
		logger:       logger,
		tracer:       otel.Tracer("matching-service-uc"),
	}
}

// Execute reacts to a RIDE_CANCELLED event. The branching is driven by an
// *explicit* cancelledBy field rather than inferring intent from whether
// driverID is empty — that implicit contract is brittle.
//
// Semantics:
//   - cancelledBy = RIDER : abort the entire matching state machine.
//   - cancelledBy = DRIVER: only the *current* candidate's cancellation should
//     fast-forward to the next candidate. Stale events (driver no longer the
//     active candidate) are ignored.
func (h *handleRideCancelledUseCase) Execute(ctx context.Context, rideID, driverID string, cancelledBy domain.CancelledBy) error {
	ctx, span := h.tracer.Start(ctx, "UseCase.HandleRideCancelled")
	defer span.End()

	state, err := h.dispatchRepo.GetState(ctx, rideID)
	if err != nil {
		span.RecordError(err)
		h.logger.Error("failed to fetch state", zap.String("rideID", rideID), zap.Error(err))
		return errs.ErrInternal.WithMessage("failed to fetch state").WithRootErr(err)
	}
	if state == nil {
		_ = h.dispatchRepo.RemoveTimeout(ctx, rideID)
		return nil
	}

	if state.Status != domain.RideStatusFinding {
		_ = h.dispatchRepo.RemoveTimeout(ctx, rideID)
		return nil
	}

	switch cancelledBy {
	case domain.CancelledByRider:
		h.logger.Info("Rider cancelled the ride. Stopping matching process.", zap.String("rideID", rideID))
		_ = h.dispatchRepo.RemoveTimeout(ctx, rideID)
		_ = h.dispatchRepo.DeleteState(ctx, rideID)
		return nil

	case domain.CancelledByDriver:
		if len(state.Candidates) == 0 || state.CurrentIndex >= len(state.Candidates) {
			h.logger.Warn("Driver-cancel event but no active candidate", zap.String("rideID", rideID))
			return nil
		}
		current := state.Candidates[state.CurrentIndex]
		if current.DriverID != driverID {
			h.logger.Debug("Ignoring stale driver-cancel",
				zap.String("rideID", rideID),
				zap.String("eventDriverID", driverID),
				zap.String("activeDriverID", current.DriverID))
			return nil
		}

		h.logger.Info("Active driver cancelled. Fast-forwarding to next candidate",
			zap.String("rideID", rideID), zap.String("driverID", driverID))
		return h.timeoutUC.Execute(ctx, rideID, time.Now().Unix())

	default:
		h.logger.Warn("Unknown cancelled_by value, treating as no-op",
			zap.String("rideID", rideID), zap.String("cancelled_by", string(cancelledBy)))
		return nil
	}
}
