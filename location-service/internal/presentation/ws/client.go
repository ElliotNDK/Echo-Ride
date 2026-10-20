package ws

import (
	"bytes"
	"context"
	"echo-ride/services/location-service/internal/application"
	"echo-ride/services/location-service/pkg/jwt"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
	authTimeout    = 5 * time.Second
)

type Client struct {
	Hub              *Hub
	UserID           uuid.UUID
	Conn             *websocket.Conn
	Send             chan []byte
	updateLocationUC application.UpdateDriverLocationUseCase
	jwtSecret        string
	isAuthenticated  bool
}

func (c *Client) readPump() {
	defer func() {
		if c.isAuthenticated {
			c.Hub.Unregister <- c
		}
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(authTimeout))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Println("read error:", err)
			}
			break
		}

		var rawMsg struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}

		if err := json.Unmarshal(message, &rawMsg); err != nil {
			log.Printf("Invalid base message format from user %s: %s", c.UserID.String(), string(message))
			continue
		}

		if !c.isAuthenticated {
			if rawMsg.Type != "AUTH" {
				log.Printf("Unauthenticated message type '%s' from user %s", rawMsg.Type, c.UserID.String())
				break
			}

			var authPayload struct {
				Token string `json:"token"`
			}
			if err := json.Unmarshal(rawMsg.Data, &authPayload); err != nil {
				log.Println("Invalid auth payload")
				break
			}

			userID, err := jwt.VerifyAndExtractUserID(authPayload.Token, c.jwtSecret)
			if err != nil {
				log.Printf("Token verification failed: %v", err)
				break
			}

			c.UserID = userID
			c.isAuthenticated = true
			c.Hub.Register <- c

			c.Conn.SetReadDeadline(time.Now().Add(pongWait))

			authSuccessMsg, err := json.Marshal(map[string]string{
				"type": "AUTH_SUCCESS",
			})
			if err != nil {
				log.Printf("Failed to marshal AUTH_SUCCESS for user %s: %v", c.UserID.String(), err)
				break
			}
			c.Send <- authSuccessMsg

			continue
		}

		switch rawMsg.Type {
		case "DRIVER_LOCATION_SYNC":
			var payload struct {
				RideID  string  `json:"ride_id"`
				Lat     float64 `json:"lat"`
				Lng     float64 `json:"lng"`
				Bearing float32 `json:"bearing"`
				Speed   float32 `json:"speed"`
			}
			if err := json.Unmarshal(rawMsg.Data, &payload); err != nil {
				log.Printf("Invalid location payload from driver %s", c.UserID.String())
				continue
			}

			rideID, parseErr := uuid.Parse(payload.RideID)
			if parseErr != nil {
				log.Printf("Invalid ride_id in DRIVER_LOCATION_SYNC from driver %s: %q — skipping",
					c.UserID.String(), payload.RideID)
				continue
			}

			cmd := application.UpdateLocationCommand{
				DriverID: c.UserID,
				RideID:   rideID,
				Lat:      payload.Lat,
				Lng:      payload.Lng,
				Bearing:  payload.Bearing,
				Speed:    payload.Speed,
			}

			if err := c.updateLocationUC.Execute(context.Background(), cmd); err != nil {
				log.Printf("Failed to process location update from driver %s: %v", c.UserID.String(), err)
			}

		case "SUBSCRIBE_RIDE":
			var payload struct {
				RideID string `json:"ride_id"`
			}
			if err := json.Unmarshal(rawMsg.Data, &payload); err != nil || payload.RideID == "" {
				log.Printf("Invalid subscribe payload from rider %s", c.UserID.String())
				continue
			}

			c.Hub.SubscribeToRideTracking(context.Background(), c.UserID, payload.RideID)
		default:
			log.Printf("Unknown message type '%s' from user %s", rawMsg.Type, c.UserID.String())
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write(bytes.TrimSpace(<-c.Send))
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
