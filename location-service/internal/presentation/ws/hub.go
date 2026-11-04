package ws

import (
	"context"
	"echo-ride/services/location-service/internal/domain"
	"encoding/json"
	"sync"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Hub struct {
	Clients     map[uuid.UUID]*Client
	Register    chan *Client
	Unregister  chan *Client
	mu          sync.RWMutex
	logger      *zap.Logger
	redisClient *redis.Client
}

type LocationUpdateMsg struct {
	RideID   string  `json:"ride_id"`
	DriverID string  `json:"driver_id"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	Bearing  float32 `json:"bearing,omitempty"`
	Speed    float32 `json:"speed,omitempty"`
}

func NewHub(redisClient *redis.Client, logger *zap.Logger) *Hub {
	return &Hub{
		Clients:     make(map[uuid.UUID]*Client),
		Register:    make(chan *Client),
		Unregister:  make(chan *Client),
		logger:      logger,
		redisClient: redisClient,
	}
}

func (h *Hub) Run() {
	h.logger.Info("Websocket Hub is running...")
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.Clients[client.UserID] = client
			h.mu.Unlock()
			h.logger.Info("Client registered", zap.String("user_id", client.UserID.String()), zap.Int("total_clients", len(h.Clients)))

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[client.UserID]; ok {
				delete(h.Clients, client.UserID)
				close(client.Send)
				h.logger.Info("Client unregistered", zap.String("user_id", client.UserID.String()), zap.Int("total_clients", len(h.Clients)))
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) SendMessageToUser(userID uuid.UUID, message []byte) {
	h.mu.Lock()
	client, ok := h.Clients[userID]
	h.mu.Unlock()

	if ok {
		select {
		case client.Send <- message:
			h.logger.Debug("Message sent to user", zap.String("user_id", userID.String()))
		default:
			h.logger.Warn("Send channel full, dropping message for user", zap.String("user_id", userID.String()))
		}
	} else {
		h.logger.Debug("Client not registered, dropping message for user", zap.String("user_id", userID.String()))
	}
}

func (h *Hub) NotifyUser(ctx context.Context, userID uuid.UUID, messageType string, payload interface{}) error {
	wsMessage := map[string]interface{}{
		"type": messageType,
		"data": payload,
	}

	msgBytes, err := json.Marshal(wsMessage)
	if err != nil {
		h.logger.Error("Failed to marshal WebSocket message", zap.Error(err), zap.String("user_id", userID.String()))
		return err
	}

	h.SendMessageToUser(userID, msgBytes)

	return nil
}

// BroadcastDriverLocation implements application.LocationBroadcaster.
// Converts a domain.DriverLocation to the Redis Pub/Sub message format and
// publishes it to the "ride_tracking:{rideID}" channel so all hub instances
// subscribed for that ride can push to the rider's WebSocket.
func (h *Hub) BroadcastDriverLocation(ctx context.Context, loc domain.DriverLocation) {
	h.BroadcastLocationToRedis(ctx, LocationUpdateMsg{
		RideID:   loc.RideID.String(),
		DriverID: loc.DriverID.String(),
		Lat:      loc.Lat,
		Lng:      loc.Lng,
		Bearing:  loc.Bearing,
		Speed:    loc.Speed,
	})
}

func (h *Hub) BroadcastLocationToRedis(ctx context.Context, msg LocationUpdateMsg) {
	channelName := "ride_tracking:" + msg.RideID

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("Failed to marshal location msg", zap.Error(err))
		return
	}

	err = h.redisClient.Publish(ctx, channelName, msgBytes).Err()
	if err != nil {
		h.logger.Error("Failed to publish location to Redis", zap.Error(err))
	}
}

func (h *Hub) SubscribeToRideTracking(ctx context.Context, riderID uuid.UUID, rideID string) {
	channelName := "ride_tracking:" + rideID
	pubsub := h.redisClient.Subscribe(ctx, channelName)

	h.logger.Info("Rider subscribed to ride tracking", zap.String("rider_id", riderID.String()), zap.String("ride_id", rideID))

	go func() {
		defer pubsub.Close()
		ch := pubsub.Channel()

		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-ch:
				wsMessage := map[string]interface{}{
					"type": "DRIVER_LOCATION_UPDATE",
					"data": json.RawMessage(msg.Payload),
				}
				wsBytes, _ := json.Marshal(wsMessage)

				h.SendMessageToUser(riderID, wsBytes)
			}
		}
	}()
}
