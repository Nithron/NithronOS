package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512 * 1024 // 512KB

	// Size of the client send channel buffer
	sendBufferSize = 256
)

// WebSocketHandler handles WebSocket connections for real-time communication
type WebSocketHandler struct {
	hub      *Hub
	upgrader websocket.Upgrader
	logger   zerolog.Logger
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(hub *Hub, logger zerolog.Logger) *WebSocketHandler {
	return &WebSocketHandler{
		hub: hub,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// In production, validate origin against allowed domains
				return true
			},
		},
		logger: logger.With().Str("component", "websocket-handler").Logger(),
	}
}

// ServeHTTP handles WebSocket upgrade requests
func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract authentication from headers (set by middleware)
	userID := r.Header.Get("X-Device-User-ID")
	deviceID := r.Header.Get("X-Device-ID")
	username := r.Header.Get("X-Username")
	deviceName := r.Header.Get("X-Device-Name")

	if userID == "" || deviceID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to upgrade connection")
		return
	}

	// Generate a unique color for this user
	color := generateUserColor(userID)

	// Create client
	client := &Client{
		ID:            uuid.New().String(),
		UserID:        userID,
		Username:      username,
		DeviceID:      deviceID,
		DeviceName:    deviceName,
		Color:         color,
		Subscriptions: make(map[string]bool),
		Send:          make(chan *Message, sendBufferSize),
		hub:           h.hub,
	}

	// Register client with hub
	h.hub.RegisterClient(client)

	// Start goroutines for reading and writing
	go h.writePump(client, conn)
	go h.readPump(client, conn)

	// Send connection confirmation
	h.sendConnectionConfirmation(client)
}

func (h *WebSocketHandler) readPump(client *Client, conn *websocket.Conn) {
	defer func() {
		h.hub.UnregisterClient(client)
		conn.Close()
	}()

	conn.SetReadLimit(maxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, messageBytes, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.logger.Error().Err(err).Str("client_id", client.ID).Msg("Unexpected close error")
			}
			break
		}

		// Parse message
		var msg Message
		if err := json.Unmarshal(messageBytes, &msg); err != nil {
			h.logger.Warn().Err(err).Str("client_id", client.ID).Msg("Invalid message format")
			h.sendError(client, "Invalid message format")
			continue
		}

		// Process message
		h.handleMessage(client, &msg)
	}
}

func (h *WebSocketHandler) writePump(client *Client, conn *websocket.Conn) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			msgBytes, err := json.Marshal(message)
			if err != nil {
				h.logger.Error().Err(err).Msg("Failed to marshal message")
				continue
			}

			_, _ = w.Write(msgBytes)

			// Write queued messages in the same frame if available
			n := len(client.Send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})
				msg := <-client.Send
				msgBytes, _ := json.Marshal(msg)
				_, _ = w.Write(msgBytes)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *WebSocketHandler) handleMessage(client *Client, msg *Message) {
	// Update activity timestamp
	if client.Presence != nil {
		client.Presence.LastActivity = time.Now()
	}

	switch msg.Type {
	case MsgTypePing:
		h.handlePing(client)

	case MsgTypeSubscribe:
		h.handleSubscribe(client, msg)

	case MsgTypeUnsubscribe:
		h.handleUnsubscribe(client, msg)

	case MsgTypePresenceUpdate:
		h.handlePresenceUpdate(client, msg)

	case MsgTypePresenceList:
		h.handlePresenceList(client, msg)

	case MsgTypeCursorMove:
		h.handleCursorMove(client, msg)

	case MsgTypeCursorSelect:
		h.handleCursorSelect(client, msg)

	case MsgTypeFileLock:
		h.handleFileLock(client, msg)

	case MsgTypeFileUnlock:
		h.handleFileUnlock(client, msg)

	case MsgTypeEditStart:
		h.handleEditStart(client, msg)

	case MsgTypeEditOp:
		h.handleEditOp(client, msg)

	case MsgTypeEditEnd:
		h.handleEditEnd(client, msg)

	default:
		h.logger.Warn().
			Str("client_id", client.ID).
			Str("type", string(msg.Type)).
			Msg("Unknown message type")
	}
}

func (h *WebSocketHandler) handlePing(client *Client) {
	h.sendMessage(client, &Message{
		ID:        uuid.New().String(),
		Type:      MsgTypePong,
		Timestamp: time.Now(),
	})
}

func (h *WebSocketHandler) handleSubscribe(client *Client, msg *Message) {
	var payload struct {
		Channel string `json:"channel"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil || payload.Channel == "" {
		h.sendError(client, "Invalid channel")
		return
	}

	if err := h.hub.Subscribe(client.ID, payload.Channel); err != nil {
		h.sendError(client, err.Error())
		return
	}

	// Send confirmation
	h.sendMessage(client, &Message{
		ID:        uuid.New().String(),
		Type:      MsgTypeSubscribe,
		Channel:   payload.Channel,
		Timestamp: time.Now(),
	})

	// Broadcast join to channel
	h.broadcastPresenceJoin(client, payload.Channel)
}

func (h *WebSocketHandler) handleUnsubscribe(client *Client, msg *Message) {
	var payload struct {
		Channel string `json:"channel"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil || payload.Channel == "" {
		h.sendError(client, "Invalid channel")
		return
	}

	if err := h.hub.Unsubscribe(client.ID, payload.Channel); err != nil {
		h.sendError(client, err.Error())
		return
	}

	// Broadcast leave to channel
	h.broadcastPresenceLeave(client, payload.Channel)
}

func (h *WebSocketHandler) handlePresenceUpdate(client *Client, msg *Message) {
	var update PresenceInfo
	if err := json.Unmarshal(msg.Payload, &update); err != nil {
		h.sendError(client, "Invalid presence data")
		return
	}

	if err := h.hub.UpdatePresence(client.ID, &update); err != nil {
		h.sendError(client, err.Error())
		return
	}

	// Broadcast presence update to subscribed channels
	client.mu.RLock()
	channels := make([]string, 0, len(client.Subscriptions))
	for ch := range client.Subscriptions {
		channels = append(channels, ch)
	}
	client.mu.RUnlock()

	payload, _ := json.Marshal(client.Presence)
	for _, channel := range channels {
		h.hub.Broadcast(&Message{
			ID:        uuid.New().String(),
			Type:      MsgTypePresenceUpdate,
			Channel:   channel,
			UserID:    client.UserID,
			DeviceID:  client.DeviceID,
			Timestamp: time.Now(),
			Payload:   payload,
		})
	}
}

func (h *WebSocketHandler) handlePresenceList(client *Client, msg *Message) {
	var payload struct {
		Channel string `json:"channel"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil || payload.Channel == "" {
		h.sendError(client, "Invalid channel")
		return
	}

	presence := h.hub.GetChannelPresence(payload.Channel)
	responsePayload, _ := json.Marshal(map[string]interface{}{
		"channel":  payload.Channel,
		"presence": presence,
	})

	h.sendMessage(client, &Message{
		ID:        uuid.New().String(),
		Type:      MsgTypePresenceList,
		Channel:   payload.Channel,
		Timestamp: time.Now(),
		Payload:   responsePayload,
	})
}

func (h *WebSocketHandler) handleCursorMove(client *Client, msg *Message) {
	var payload struct {
		Channel  string         `json:"channel"`
		Position CursorPosition `json:"position"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		h.sendError(client, "Invalid cursor data")
		return
	}

	// Update client's cursor position
	if client.Presence != nil {
		client.Presence.CursorPos = &payload.Position
	}

	// Broadcast to channel
	broadcastPayload, _ := json.Marshal(map[string]interface{}{
		"user_id":   client.UserID,
		"username":  client.Username,
		"color":     client.Color,
		"position":  payload.Position,
	})

	h.hub.Broadcast(&Message{
		ID:        uuid.New().String(),
		Type:      MsgTypeCursorMove,
		Channel:   payload.Channel,
		UserID:    client.UserID,
		DeviceID:  client.DeviceID,
		Timestamp: time.Now(),
		Payload:   broadcastPayload,
	})
}

func (h *WebSocketHandler) handleCursorSelect(client *Client, msg *Message) {
	var payload struct {
		Channel   string         `json:"channel"`
		Selection CursorPosition `json:"selection"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		h.sendError(client, "Invalid selection data")
		return
	}

	// Broadcast to channel
	broadcastPayload, _ := json.Marshal(map[string]interface{}{
		"user_id":   client.UserID,
		"username":  client.Username,
		"color":     client.Color,
		"selection": payload.Selection,
	})

	h.hub.Broadcast(&Message{
		ID:        uuid.New().String(),
		Type:      MsgTypeCursorSelect,
		Channel:   payload.Channel,
		UserID:    client.UserID,
		DeviceID:  client.DeviceID,
		Timestamp: time.Now(),
		Payload:   broadcastPayload,
	})
}

func (h *WebSocketHandler) handleFileLock(client *Client, msg *Message) {
	var payload struct {
		ShareID  string   `json:"share_id"`
		Path     string   `json:"path"`
		LockType LockType `json:"lock_type"`
		Duration int      `json:"duration"` // seconds
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		h.sendError(client, "Invalid lock request")
		return
	}

	if payload.LockType == "" {
		payload.LockType = LockTypeExclusive
	}
	if payload.Duration == 0 {
		payload.Duration = 300 // 5 minutes default
	}

	lock, err := h.hub.LockFile(client.ID, payload.ShareID, payload.Path, payload.LockType, time.Duration(payload.Duration)*time.Second)
	if err != nil {
		h.sendError(client, err.Error())
		return
	}

	responsePayload, _ := json.Marshal(lock)
	h.sendMessage(client, &Message{
		ID:        uuid.New().String(),
		Type:      MsgTypeFileLock,
		Timestamp: time.Now(),
		Payload:   responsePayload,
	})

	// Broadcast lock notification to share channel
	channel := "share:" + payload.ShareID
	h.hub.Broadcast(&Message{
		ID:        uuid.New().String(),
		Type:      MsgTypeFileLock,
		Channel:   channel,
		UserID:    client.UserID,
		DeviceID:  client.DeviceID,
		Timestamp: time.Now(),
		Payload:   responsePayload,
	})
}

func (h *WebSocketHandler) handleFileUnlock(client *Client, msg *Message) {
	var payload struct {
		ShareID string `json:"share_id"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		h.sendError(client, "Invalid unlock request")
		return
	}

	if err := h.hub.UnlockFile(client.ID, payload.ShareID, payload.Path); err != nil {
		h.sendError(client, err.Error())
		return
	}

	responsePayload, _ := json.Marshal(payload)
	h.sendMessage(client, &Message{
		ID:        uuid.New().String(),
		Type:      MsgTypeFileUnlock,
		Timestamp: time.Now(),
		Payload:   responsePayload,
	})

	// Broadcast unlock notification to share channel
	channel := "share:" + payload.ShareID
	h.hub.Broadcast(&Message{
		ID:        uuid.New().String(),
		Type:      MsgTypeFileUnlock,
		Channel:   channel,
		UserID:    client.UserID,
		DeviceID:  client.DeviceID,
		Timestamp: time.Now(),
		Payload:   responsePayload,
	})
}

func (h *WebSocketHandler) handleEditStart(client *Client, msg *Message) {
	var payload struct {
		Channel string `json:"channel"`
		FileID  string `json:"file_id"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		h.sendError(client, "Invalid edit start request")
		return
	}

	broadcastPayload, _ := json.Marshal(map[string]interface{}{
		"user_id":  client.UserID,
		"username": client.Username,
		"color":    client.Color,
		"file_id":  payload.FileID,
	})

	h.hub.Broadcast(&Message{
		ID:        uuid.New().String(),
		Type:      MsgTypeEditStart,
		Channel:   payload.Channel,
		UserID:    client.UserID,
		DeviceID:  client.DeviceID,
		Timestamp: time.Now(),
		Payload:   broadcastPayload,
	})
}

func (h *WebSocketHandler) handleEditOp(client *Client, msg *Message) {
	// Forward the operation to all subscribers
	// The payload contains the OT (Operational Transform) operation
	h.hub.Broadcast(&Message{
		ID:        uuid.New().String(),
		Type:      MsgTypeEditOp,
		Channel:   msg.Channel,
		UserID:    client.UserID,
		DeviceID:  client.DeviceID,
		Timestamp: time.Now(),
		Payload:   msg.Payload,
	})
}

func (h *WebSocketHandler) handleEditEnd(client *Client, msg *Message) {
	var payload struct {
		Channel string `json:"channel"`
		FileID  string `json:"file_id"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		h.sendError(client, "Invalid edit end request")
		return
	}

	broadcastPayload, _ := json.Marshal(map[string]interface{}{
		"user_id": client.UserID,
		"file_id": payload.FileID,
	})

	h.hub.Broadcast(&Message{
		ID:        uuid.New().String(),
		Type:      MsgTypeEditEnd,
		Channel:   payload.Channel,
		UserID:    client.UserID,
		DeviceID:  client.DeviceID,
		Timestamp: time.Now(),
		Payload:   broadcastPayload,
	})
}

func (h *WebSocketHandler) sendConnectionConfirmation(client *Client) {
	payload, _ := json.Marshal(map[string]interface{}{
		"client_id":   client.ID,
		"user_id":     client.UserID,
		"device_id":   client.DeviceID,
		"color":       client.Color,
		"server_time": time.Now(),
	})

	h.sendMessage(client, &Message{
		ID:        uuid.New().String(),
		Type:      MsgTypeConnect,
		Timestamp: time.Now(),
		Payload:   payload,
	})
}

func (h *WebSocketHandler) broadcastPresenceJoin(client *Client, channel string) {
	payload, _ := json.Marshal(client.Presence)
	h.hub.Broadcast(&Message{
		ID:        uuid.New().String(),
		Type:      MsgTypePresenceJoin,
		Channel:   channel,
		UserID:    client.UserID,
		DeviceID:  client.DeviceID,
		Timestamp: time.Now(),
		Payload:   payload,
	})
}

func (h *WebSocketHandler) broadcastPresenceLeave(client *Client, channel string) {
	payload, _ := json.Marshal(map[string]string{
		"user_id":   client.UserID,
		"device_id": client.DeviceID,
		"client_id": client.ID,
	})

	h.hub.Broadcast(&Message{
		ID:        uuid.New().String(),
		Type:      MsgTypePresenceLeave,
		Channel:   channel,
		UserID:    client.UserID,
		DeviceID:  client.DeviceID,
		Timestamp: time.Now(),
		Payload:   payload,
	})
}

func (h *WebSocketHandler) sendMessage(client *Client, msg *Message) {
	select {
	case client.Send <- msg:
	default:
		h.logger.Warn().Str("client_id", client.ID).Msg("Send buffer full")
	}
}

func (h *WebSocketHandler) sendError(client *Client, message string) {
	payload, _ := json.Marshal(map[string]string{
		"error": message,
	})

	h.sendMessage(client, &Message{
		ID:        uuid.New().String(),
		Type:      MsgTypeError,
		Timestamp: time.Now(),
		Payload:   payload,
	})
}

// generateUserColor generates a consistent color for a user based on their ID
func generateUserColor(userID string) string {
	// Pre-defined colors for collaboration
	colors := []string{
		"#FF6B6B", // Red
		"#4ECDC4", // Teal
		"#45B7D1", // Blue
		"#96CEB4", // Green
		"#FFEAA7", // Yellow
		"#DDA0DD", // Plum
		"#98D8C8", // Mint
		"#F7DC6F", // Gold
		"#BB8FCE", // Purple
		"#85C1E9", // Sky Blue
	}

	// Simple hash to select color
	hash := 0
	for _, c := range userID {
		hash = (hash*31 + int(c)) % len(colors)
	}

	return colors[hash]
}

// ConnectionManager manages WebSocket connections with authentication
type ConnectionManager struct {
	handler *WebSocketHandler
	hub     *Hub
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(logger zerolog.Logger) *ConnectionManager {
	hub := NewHub(logger)
	go hub.Run()

	return &ConnectionManager{
		handler: NewWebSocketHandler(hub, logger),
		hub:     hub,
	}
}

// GetHandler returns the WebSocket handler
func (cm *ConnectionManager) GetHandler() *WebSocketHandler {
	return cm.handler
}

// GetHub returns the hub
func (cm *ConnectionManager) GetHub() *Hub {
	return cm.hub
}

// Shutdown gracefully shuts down the connection manager
func (cm *ConnectionManager) Shutdown(ctx context.Context) error {
	cm.hub.Stop()
	return nil
}

