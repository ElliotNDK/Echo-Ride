package outbox

//
//import (
//	"context"
//	"echo-ride/services/ride-service/internal/infrastructure/db/dbgen"
//	"time"
//
//	"github.com/jackc/pgx/v5/pgxpool"
//	"github.com/segmentio/kafka-go"
//	"go.uber.org/zap"
//)
//
//type RelayWorker struct {
//	dbPool      *pgxpool.Pool
//	kafkaWriter *kafka.Writer
//	logger      *zap.Logger
//}
//
//func NewRelayWorker(dbPool *pgxpool.Pool, brokers []string, topic string, logger *zap.Logger) *RelayWorker {
//	w := &kafka.Writer{
//		Addr:        kafka.TCP(brokers...),
//		Topic:       topic,
//		Balancer:    &kafka.LeastBytes{},
//		MaxAttempts: 3,
//	}
//
//	return &RelayWorker{
//		dbPool:      dbPool,
//		kafkaWriter: w,
//		logger:      logger,
//	}
//}
//
//func (w *RelayWorker) Start(ctx context.Context) {
//	ticker := time.NewTicker(5 * time.Second)
//	defer ticker.Stop()
//
//	w.logger.Info("starting relay worker")
//
//	for {
//		select {
//		case <-ctx.Done():
//			w.logger.Info("stopping relay worker...")
//			err := w.kafkaWriter.Close()
//			if err != nil {
//				return
//			}
//			return
//		case <-ticker.C:
//			w.processPendingEvents(ctx)
//		}
//	}
//}
//
//func (w *RelayWorker) processPendingEvents(ctx context.Context) {
//	q := dbgen.New(w.dbPool)
//
//	tx, err := w.dbPool.Begin(ctx)
//	if err != nil {
//		w.logger.Error("failed to begin transaction", zap.Error(err))
//		return
//	}
//	defer tx.Rollback(ctx)
//
//	qtx := q.WithTx(tx)
//
//	events, err := qtx.GetPendingOutboxEvents(ctx, 50)
//	if err != nil {
//		w.logger.Error("failed to get pending outbox events", zap.Error(err))
//		return
//	}
//
//	if len(events) == 0 {
//		return
//	}
//
//	for _, evt := range events {
//		msg := kafka.Message{
//			Key:   []byte(evt.EventAggregateID),
//			Value: evt.EventPayload,
//		}
//
//		err := w.kafkaWriter.WriteMessages(ctx, msg)
//		if err != nil {
//			w.logger.Error("failed to publish event to Kafka", zap.Error(err))
//			continue
//		}
//
//		err = qtx.MarkOutboxEventAsPublished(ctx, evt.EventID)
//		if err != nil {
//			w.logger.Error("failed to mark outbox event as published", zap.Error(err))
//			continue
//		} else {
//			w.logger.Info("successfully published event to Kafka", zap.String("event_id", evt.EventID.String()), zap.String("aggregate_id", evt.EventAggregateID), zap.String("event_type", evt.EventType))
//		}
//	}
//
//	_ = tx.Commit(ctx)
//}
