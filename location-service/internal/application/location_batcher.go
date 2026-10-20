package application

import (
	"context"
	"echo-ride/services/location-service/internal/domain"
	"time"

	"go.uber.org/zap"
)

type LocationBatcher struct {
	locationHistoryRepo domain.LocationHistoryRepository
	activeDriverRepo    domain.ActiveDriverRepository
	inputChan           chan domain.DriverLocation
	batchSize           int
	flushInterval       time.Duration // how often to flush the batch even if it's not full
	logger              *zap.Logger
	buffer              []domain.DriverLocation
}

func NewLocationBatcher(locationHistoryRepo domain.LocationHistoryRepository, activeDriverRepo domain.ActiveDriverRepository, logger *zap.Logger) *LocationBatcher {
	return &LocationBatcher{
		locationHistoryRepo: locationHistoryRepo,
		activeDriverRepo:    activeDriverRepo,
		inputChan:           make(chan domain.DriverLocation, 5000), // buffered channel to handle bursts
		batchSize:           500,                                    // batch size of 100 locations
		flushInterval:       500 * time.Millisecond,                 // flush every 5 seconds if batch is not full
		logger:              logger,
	}
}

func (b *LocationBatcher) Push(loc domain.DriverLocation) {
	select {
	case b.inputChan <- loc:
	default:
		b.logger.Warn("Location batcher input channel is full, dropping location update", zap.String("driver_id", loc.DriverID.String()))
	}
}

func (b *LocationBatcher) Start(ctx context.Context) {
	b.logger.Info("Location Batcher Worker started")

	buffer := make([]domain.DriverLocation, 0, b.batchSize)
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			b.flush(buffer)
			b.logger.Info("Location Batcher Worker stopped")
			return

		case loc := <-b.inputChan:
			buffer = append(buffer, loc)
			if len(buffer) >= b.batchSize {
				b.flush(buffer)
				buffer = buffer[:0] // reset buffer
			}

		case <-ticker.C:
			if len(buffer) > 0 {
				b.flush(buffer)
				buffer = buffer[:0] // reset buffer
			}
		}
	}
}

func (b *LocationBatcher) flush(locations []domain.DriverLocation) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCassandra := b.locationHistoryRepo.SaveLocationBatch(ctx, locations)
	if errCassandra != nil {
		b.logger.Error("Failed to save location batch to Cassandra", zap.Error(errCassandra))
	} else {
		b.logger.Debug("Flushed locations to Cassandra", zap.Int("batch_size", len(locations)))
	}

	errRedis := b.activeDriverRepo.UpsertActiveLocation(ctx, locations)
	if errRedis != nil {
		b.logger.Error("Failed to upsert active locations to Redis", zap.Error(errRedis))
	} else {
		b.logger.Debug("Upserted active locations to Redis", zap.Int("batch_size", len(locations)))
	}

}
