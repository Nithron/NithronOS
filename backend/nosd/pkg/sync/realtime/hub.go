// Package realtime provides real-time collaboration features for NithronSync.
package realtime

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// MessageType represents the type of real-time message
type MessageType string

const (
	// Connection messages
	MsgTypeConnect    MessageType = "connect"
	MsgTypeDisconnect MessageType = "disconnect"
	MsgTypePing       MessageType = "ping"
	MsgTypePong       MessageType = "pong"

	// Presence messages
	MsgTypePresenceJoin   MessageType = "presence.join"
	MsgTypePresenceLeave  MessageType = "presence.leave"
	MsgTypePresenceUpdate MessageType = "presence.update"
	MsgTypePresenceList   MessageType = "presence.list"

	// File messages
	MsgTypeFileChange  MessageType = "file.change"
	MsgTypeFileCreate  MessageType = "file.create"
	MsgTypeFileDelete  MessageType = "file.delete"
	MsgTypeFileRename  MessageType = "file.rename"
	MsgTypeFileLock    MessageType = "file.lock"
	MsgTypeFileUnlock  MessageType = "file.unlock"

	// Cursor messages
	MsgTypeCursorMove   MessageType = "cursor.move"
	MsgTypeCursorSelect MessageType = "cursor.select"

	// Collaboration messages
	MsgTypeEditStart  MessageType = "edit.start"
	MsgTypeEditOp     MessageType = "edit.op"
	MsgTypeEditEnd    MessageType = "edit.end"
	MsgTypeEditSync   MessageType = "edit.sync"

	// Subscription messages
	MsgTypeSubscribe   MessageType = "subscribe"
	MsgTypeUnsubscribe MessageType = "unsubscribe"

	// Error messages
	MsgTypeError MessageType = "error"
)

// Message represents a real-time message
type Message struct {
	ID        string          `json:"id"`
	Type      MessageType     `json:"type"`
	Channel   string          `json:"channel,omitempty"`
	UserID    string          `json:"user_id,omitempty"`
	DeviceID  string          `json:"device_id,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// PresenceInfo represents a user's presence information
type PresenceInfo struct {
	UserID       string            `json:"user_id"`
	Username     string            `json:"username"`
	DeviceID     string            `json:"device_id"`
	DeviceName   string            `json:"device_name"`
	Status       PresenceStatus    `json:"status"`
	CurrentFile  string            `json:"current_file,omitempty"`
	CurrentShare string            `json:"current_share,omitempty"`
	CursorPos    *CursorPosition   `json:"cursor_pos,omitempty"`
	Color        string            `json:"color"`
	JoinedAt     time.Time         `json:"joined_at"`
	LastActivity time.Time         `json:"last_activity"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// PresenceStatus represents user's online status
type PresenceStatus string

const (
	StatusOnline  PresenceStatus = "online"
	StatusAway    PresenceStatus = "away"
	StatusBusy    PresenceStatus = "busy"
	StatusOffline PresenceStatus = "offline"
)

// CursorPosition represents cursor position in a file
type CursorPosition struct {
	Line      int `json:"line"`
	Column    int `json:"column"`
	EndLine   int `json:"end_line,omitempty"`
	EndColumn int `json:"end_column,omitempty"`
}

// FileLock represents a file lock
type FileLock struct {
	FileID    string    `json:"file_id"`
	ShareID   string    `json:"share_id"`
	Path      string    `json:"path"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	DeviceID  string    `json:"device_id"`
	LockType  LockType  `json:"lock_type"`
	LockedAt  time.Time `json:"locked_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// LockType represents the type of file lock
type LockType string

const (
	LockTypeExclusive LockType = "exclusive"
	LockTypeShared    LockType = "shared"
)

// Client represents a connected WebSocket client
type Client struct {
	ID           string
	UserID       string
	Username     string
	DeviceID     string
	DeviceName   string
	Color        string
	Presence     *PresenceInfo
	Subscriptions map[string]bool
	Send         chan *Message
	hub          *Hub
	mu           sync.RWMutex
}

// Hub manages all WebSocket connections and message routing
type Hub struct {
	// Registered clients
	clients map[string]*Client

	// Clients by user ID (one user can have multiple devices)
	userClients map[string]map[string]*Client

	// Channel subscriptions (channel -> client IDs)
	channels map[string]map[string]bool

	// File locks
	fileLocks map[string]*FileLock

	// Presence information
	presence map[string]*PresenceInfo

	// Message channels
	register   chan *Client
	unregister chan *Client
	broadcast  chan *Message

	// Mutex for thread safety
	mu sync.RWMutex

	// Logger
	logger zerolog.Logger

	// Context for shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// NewHub creates a new Hub instance
func NewHub(logger zerolog.Logger) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		clients:     make(map[string]*Client),
		userClients: make(map[string]map[string]*Client),
		channels:    make(map[string]map[string]bool),
		fileLocks:   make(map[string]*FileLock),
		presence:    make(map[string]*PresenceInfo),
		register:    make(chan *Client, 256),
		unregister:  make(chan *Client, 256),
		broadcast:   make(chan *Message, 1024),
		logger:      logger.With().Str("component", "realtime-hub").Logger(),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			h.shutdown()
			return

		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastMessage(message)

		case <-ticker.C:
			h.cleanupExpiredLocks()
			h.updateIdlePresence()
		}
	}
}

// Stop gracefully stops the hub
func (h *Hub) Stop() {
	h.cancel()
}

// RegisterClient registers a new client
func (h *Hub) RegisterClient(client *Client) {
	h.register <- client
}

// UnregisterClient unregisters a client
func (h *Hub) UnregisterClient(client *Client) {
	h.unregister <- client
}

// Broadcast sends a message to all subscribed clients
func (h *Hub) Broadcast(msg *Message) {
	h.broadcast <- msg
}

func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Add to clients map
	h.clients[client.ID] = client

	// Add to user clients map
	if h.userClients[client.UserID] == nil {
		h.userClients[client.UserID] = make(map[string]*Client)
	}
	h.userClients[client.UserID][client.ID] = client

	// Create presence info
	presence := &PresenceInfo{
		UserID:       client.UserID,
		Username:     client.Username,
		DeviceID:     client.DeviceID,
		DeviceName:   client.DeviceName,
		Status:       StatusOnline,
		Color:        client.Color,
		JoinedAt:     time.Now(),
		LastActivity: time.Now(),
	}
	client.Presence = presence
	h.presence[client.ID] = presence

	h.logger.Info().
		Str("client_id", client.ID).
		Str("user_id", client.UserID).
		Str("device_id", client.DeviceID).
		Msg("Client registered")
}

func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client.ID]; !ok {
		return
	}

	// Remove from clients map
	delete(h.clients, client.ID)

	// Remove from user clients map
	if userClients, ok := h.userClients[client.UserID]; ok {
		delete(userClients, client.ID)
		if len(userClients) == 0 {
			delete(h.userClients, client.UserID)
		}
	}

	// Remove from all channels
	for channel := range client.Subscriptions {
		if subs, ok := h.channels[channel]; ok {
			delete(subs, client.ID)
			if len(subs) == 0 {
				delete(h.channels, channel)
			}
		}
	}

	// Remove presence
	delete(h.presence, client.ID)

	// Release any file locks held by this client
	h.releaseClientLocks(client.ID)

	// Close send channel
	close(client.Send)

	h.logger.Info().
		Str("client_id", client.ID).
		Str("user_id", client.UserID).
		Msg("Client unregistered")

	// Notify others about presence leave
	h.broadcastPresenceLeave(client)
}

func (h *Hub) broadcastMessage(msg *Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if msg.Channel == "" {
		// Broadcast to all clients
		for _, client := range h.clients {
			select {
			case client.Send <- msg:
			default:
				// Client's send buffer is full
				h.logger.Warn().
					Str("client_id", client.ID).
					Msg("Client send buffer full, dropping message")
			}
		}
	} else {
		// Send to channel subscribers only
		if subs, ok := h.channels[msg.Channel]; ok {
			for clientID := range subs {
				if client, ok := h.clients[clientID]; ok {
					select {
					case client.Send <- msg:
					default:
						h.logger.Warn().
							Str("client_id", clientID).
							Msg("Client send buffer full, dropping message")
					}
				}
			}
		}
	}
}

func (h *Hub) broadcastPresenceLeave(client *Client) {
	payload, _ := json.Marshal(map[string]string{
		"user_id":   client.UserID,
		"device_id": client.DeviceID,
		"client_id": client.ID,
	})

	msg := &Message{
		ID:        uuid.New().String(),
		Type:      MsgTypePresenceLeave,
		UserID:    client.UserID,
		DeviceID:  client.DeviceID,
		Timestamp: time.Now(),
		Payload:   payload,
	}

	// Send to all channels the client was subscribed to
	for channel := range client.Subscriptions {
		msg.Channel = channel
		h.broadcastMessage(msg)
	}
}

// Subscribe adds a client to a channel
func (h *Hub) Subscribe(clientID, channel string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	client, ok := h.clients[clientID]
	if !ok {
		return ErrClientNotFound
	}

	// Add to channel
	if h.channels[channel] == nil {
		h.channels[channel] = make(map[string]bool)
	}
	h.channels[channel][clientID] = true

	// Track in client subscriptions
	client.mu.Lock()
	if client.Subscriptions == nil {
		client.Subscriptions = make(map[string]bool)
	}
	client.Subscriptions[channel] = true
	client.mu.Unlock()

	h.logger.Debug().
		Str("client_id", clientID).
		Str("channel", channel).
		Msg("Client subscribed to channel")

	return nil
}

// Unsubscribe removes a client from a channel
func (h *Hub) Unsubscribe(clientID, channel string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	client, ok := h.clients[clientID]
	if !ok {
		return ErrClientNotFound
	}

	// Remove from channel
	if subs, ok := h.channels[channel]; ok {
		delete(subs, clientID)
		if len(subs) == 0 {
			delete(h.channels, channel)
		}
	}

	// Remove from client subscriptions
	client.mu.Lock()
	delete(client.Subscriptions, channel)
	client.mu.Unlock()

	return nil
}

// GetChannelPresence returns all presence info for a channel
func (h *Hub) GetChannelPresence(channel string) []*PresenceInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []*PresenceInfo
	if subs, ok := h.channels[channel]; ok {
		for clientID := range subs {
			if presence, ok := h.presence[clientID]; ok {
				result = append(result, presence)
			}
		}
	}
	return result
}

// UpdatePresence updates a client's presence information
func (h *Hub) UpdatePresence(clientID string, update *PresenceInfo) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	presence, ok := h.presence[clientID]
	if !ok {
		return ErrClientNotFound
	}

	// Update fields
	if update.Status != "" {
		presence.Status = update.Status
	}
	if update.CurrentFile != "" {
		presence.CurrentFile = update.CurrentFile
	}
	if update.CurrentShare != "" {
		presence.CurrentShare = update.CurrentShare
	}
	if update.CursorPos != nil {
		presence.CursorPos = update.CursorPos
	}
	presence.LastActivity = time.Now()

	return nil
}

// LockFile attempts to lock a file
func (h *Hub) LockFile(clientID, shareID, path string, lockType LockType, duration time.Duration) (*FileLock, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	client, ok := h.clients[clientID]
	if !ok {
		return nil, ErrClientNotFound
	}

	lockKey := shareID + ":" + path

	// Check if already locked
	if existing, ok := h.fileLocks[lockKey]; ok {
		if existing.ExpiresAt.After(time.Now()) {
			if existing.UserID != client.UserID {
				return nil, ErrFileLocked
			}
			// Same user, extend lock
			existing.ExpiresAt = time.Now().Add(duration)
			return existing, nil
		}
		// Lock expired, can be taken
	}

	lock := &FileLock{
		FileID:    uuid.New().String(),
		ShareID:   shareID,
		Path:      path,
		UserID:    client.UserID,
		Username:  client.Username,
		DeviceID:  client.DeviceID,
		LockType:  lockType,
		LockedAt:  time.Now(),
		ExpiresAt: time.Now().Add(duration),
	}

	h.fileLocks[lockKey] = lock

	h.logger.Info().
		Str("client_id", clientID).
		Str("share_id", shareID).
		Str("path", path).
		Msg("File locked")

	return lock, nil
}

// UnlockFile releases a file lock
func (h *Hub) UnlockFile(clientID, shareID, path string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	client, ok := h.clients[clientID]
	if !ok {
		return ErrClientNotFound
	}

	lockKey := shareID + ":" + path
	lock, ok := h.fileLocks[lockKey]
	if !ok {
		return nil // Already unlocked
	}

	// Only the lock holder or admin can unlock
	if lock.UserID != client.UserID {
		return ErrNotLockOwner
	}

	delete(h.fileLocks, lockKey)

	h.logger.Info().
		Str("client_id", clientID).
		Str("share_id", shareID).
		Str("path", path).
		Msg("File unlocked")

	return nil
}

// GetFileLock returns the current lock on a file
func (h *Hub) GetFileLock(shareID, path string) *FileLock {
	h.mu.RLock()
	defer h.mu.RUnlock()

	lockKey := shareID + ":" + path
	if lock, ok := h.fileLocks[lockKey]; ok {
		if lock.ExpiresAt.After(time.Now()) {
			return lock
		}
	}
	return nil
}

func (h *Hub) releaseClientLocks(clientID string) {
	client, ok := h.clients[clientID]
	if !ok {
		return
	}

	for key, lock := range h.fileLocks {
		if lock.DeviceID == client.DeviceID {
			delete(h.fileLocks, key)
		}
	}
}

func (h *Hub) cleanupExpiredLocks() {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	for key, lock := range h.fileLocks {
		if lock.ExpiresAt.Before(now) {
			delete(h.fileLocks, key)
			h.logger.Debug().
				Str("share_id", lock.ShareID).
				Str("path", lock.Path).
				Msg("Expired file lock removed")
		}
	}
}

func (h *Hub) updateIdlePresence() {
	h.mu.Lock()
	defer h.mu.Unlock()

	idleThreshold := time.Now().Add(-5 * time.Minute)
	for _, presence := range h.presence {
		if presence.Status == StatusOnline && presence.LastActivity.Before(idleThreshold) {
			presence.Status = StatusAway
		}
	}
}

func (h *Hub) shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Close all client connections
	for _, client := range h.clients {
		close(client.Send)
	}

	h.clients = make(map[string]*Client)
	h.userClients = make(map[string]map[string]*Client)
	h.channels = make(map[string]map[string]bool)
	h.fileLocks = make(map[string]*FileLock)
	h.presence = make(map[string]*PresenceInfo)

	h.logger.Info().Msg("Hub shutdown complete")
}

// GetClient returns a client by ID
func (h *Hub) GetClient(clientID string) *Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.clients[clientID]
}

// GetUserClients returns all clients for a user
func (h *Hub) GetUserClients(userID string) []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var clients []*Client
	if userClients, ok := h.userClients[userID]; ok {
		for _, client := range userClients {
			clients = append(clients, client)
		}
	}
	return clients
}

// SendToUser sends a message to all of a user's connected clients
func (h *Hub) SendToUser(userID string, msg *Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if userClients, ok := h.userClients[userID]; ok {
		for _, client := range userClients {
			select {
			case client.Send <- msg:
			default:
			}
		}
	}
}

// SendToClient sends a message to a specific client
func (h *Hub) SendToClient(clientID string, msg *Message) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	client, ok := h.clients[clientID]
	if !ok {
		return ErrClientNotFound
	}

	select {
	case client.Send <- msg:
		return nil
	default:
		return ErrSendBufferFull
	}
}

// GetStats returns hub statistics
func (h *Hub) GetStats() HubStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return HubStats{
		TotalClients:   len(h.clients),
		TotalUsers:     len(h.userClients),
		TotalChannels:  len(h.channels),
		TotalFileLocks: len(h.fileLocks),
	}
}

// HubStats contains hub statistics
type HubStats struct {
	TotalClients   int `json:"total_clients"`
	TotalUsers     int `json:"total_users"`
	TotalChannels  int `json:"total_channels"`
	TotalFileLocks int `json:"total_file_locks"`
}

