package application

import (
	"context"
	"echo-ride/services/location-service/internal/domain"
	"time"

	"go.uber.org/zap"
)

type LocationCleaner struct {
	repo    domain.ActiveDriverRepository
	timeout time.Duration
	logger  *zap.Logger
}

func NewLocationCleaner(repo domain.ActiveDriverRepository, timeout time.Duration, logger *zap.Logger) *LocationCleaner {
	return &LocationCleaner{
		repo:    repo,
		timeout: timeout,
		logger:  logger,
	}
}

func (c *LocationCleaner) Start(ctx context.Context) {
	c.logger.Info("starting location cleaner")

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("stopping location cleaner")
			return
		case <-ticker.C:
			c.clean(ctx)
		}
	}
}

func (c *LocationCleaner) clean(ctx context.Context) {
	olderThan := time.Now().Add(-c.timeout)

	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	count, err := c.repo.RemoveStaleDrivers(dbCtx, olderThan)
	if err != nil {
		c.logger.Error("failed to clean stale driver locations", zap.Error(err))
		return
	}

	if count > 0 {
		c.logger.Info("cleaned stale driver locations", zap.Int64("count", count))
	}
}
