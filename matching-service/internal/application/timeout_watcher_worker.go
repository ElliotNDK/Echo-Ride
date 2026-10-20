package application

import (
	"context"
	"echo-ride/services/matching-service/internal/domain"
	"time"

	"go.uber.org/zap"
)

type TimeoutWatcher struct {
	dispatchRepo    domain.DispatchRepository
	handleTimeoutUC HandleTimeoutUseCase
	logger          *zap.Logger
}

func NewTimeoutWatcher(dispatchRepo domain.DispatchRepository, uc HandleTimeoutUseCase, logger *zap.Logger) *TimeoutWatcher {
	return &TimeoutWatcher{
		dispatchRepo:    dispatchRepo,
		handleTimeoutUC: uc,
		logger:          logger,
	}
}

func (w *TimeoutWatcher) Start(ctx context.Context) {
	w.logger.Info("Starting timeout watcher")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Stopping timeout watcher")
			return
		case <-ticker.C:
			w.checkTimeouts(ctx)
		}
	}
}

func (w *TimeoutWatcher) checkTimeouts(ctx context.Context) {
	now := time.Now().Unix()

	expiredRides, err := w.dispatchRepo.GetExpiredRides(ctx, now, 100)
	if err != nil {
		w.logger.Error("Failed to get expired rides", zap.Error(err))
		return
	}

	if len(expiredRides) > 0 {
		w.logger.Info("Found expired rides", zap.Int("count", len(expiredRides)))
	}

	for _, rideID := range expiredRides {
		go func(rId string) {
			timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := w.handleTimeoutUC.Execute(timeoutCtx, rideID, now); err != nil {
				w.logger.Error("Failed to handle timeout for ride", zap.String("rideID", rId), zap.Error(err))
			}
		}(rideID)
	}
}
