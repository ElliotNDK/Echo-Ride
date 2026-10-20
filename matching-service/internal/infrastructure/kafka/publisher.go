package kafka

import (
	"context"
	"echo-ride/services/matching-service/internal/domain"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type matchingPublisher struct {
	writer *kafka.Writer
	tracer trace.Tracer
}

func NewMatchingPublisher(brokers []string, topic string) domain.MatchingEventPublisher {
	w := &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  topic,
		Balancer:               &kafka.Hash{}, // Use hash balancer to ensure same ride_id goes to same partition
		AllowAutoTopicCreation: true,
	}

	return &matchingPublisher{
		writer: w,
		tracer: otel.Tracer("matching-service-kafka-publisher"),
	}
}

func (m *matchingPublisher) publish(ctx context.Context, key string, eventType domain.RideEventStatus, payload interface{}) error {
	ctx, span := m.tracer.Start(ctx, "Kafka Publish "+string(eventType), trace.WithSpanKind(trace.SpanKindProducer))
	defer span.End()

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	headers := []kafka.Header{}
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	for k, v := range carrier {
		headers = append(headers, kafka.Header{Key: k, Value: []byte(v)})
	}

	event := IncomingKafkaEvent{
		EventID:      fmt.Sprintf("%d", time.Now().UnixNano()), // Unique event ID, can be improved with UUID
		EventType:    eventType,
		EventPayload: string(payloadBytes),
	}
	eventBytes, _ := json.Marshal(event)

	msg := kafka.Message{
		Key:     []byte(key),
		Value:   eventBytes,
		Headers: headers,
	}

	if err := m.writer.WriteMessages(ctx, msg); err != nil {
		span.RecordError(err)
		return err
	}

	return nil
}

func (m *matchingPublisher) PublishDriverMatched(ctx context.Context, rideID, driverID string) error {
	payload := map[string]interface{}{
		"ride_id":   rideID,
		"driver_id": driverID,
		"timestamp": time.Now().Unix(),
	}

	return m.publish(ctx, rideID, domain.RideDriverMatched, payload)
}

func (m *matchingPublisher) PublishMatchingFailed(ctx context.Context, rideID string) error {
	payload := map[string]interface{}{
		"ride_id": rideID,
		"reason":  "NO_DRIVERS_AVAILABLE",
	}

	return m.publish(ctx, rideID, domain.RideEventStatusFailed, payload)
}
