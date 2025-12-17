package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"nithronos/backend/nosd/pkg/sync/realtime"
)

// RealtimeHandler handles real-time WebSocket connections
type RealtimeHandler struct {
	connMgr *realtime.ConnectionManager
	logger  zerolog.Logger
}

// NewRealtimeHandler creates a new realtime handler
func NewRealtimeHandler(logger zerolog.Logger) *RealtimeHandler {
	return &RealtimeHandler{
		connMgr: realtime.NewConnectionManager(logger),
		logger:  logger.With().Str("component", "realtime-handler").Logger(),
	}
}

// Routes returns the chi router for realtime endpoints
func (h *RealtimeHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// WebSocket endpoint
	r.Get("/ws", h.HandleWebSocket)

	// REST endpoints for realtime info
	r.Get("/stats", h.GetStats)
	r.Get("/presence/{channel}", h.GetChannelPresence)
	r.Get("/locks/{share_id}", h.GetFileLocks)

	return r
}

// HandleWebSocket handles WebSocket upgrade requests
func (h *RealtimeHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	h.connMgr.GetHandler().ServeHTTP(w, r)
}

// GetStats returns realtime connection statistics
func (h *RealtimeHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.connMgr.GetHub().GetStats()
	writeJSON(w, stats)
}

// GetChannelPresence returns presence information for a channel
func (h *RealtimeHandler) GetChannelPresence(w http.ResponseWriter, r *http.Request) {
	channel := chi.URLParam(r, "channel")
	if channel == "" {
		http.Error(w, "Channel required", http.StatusBadRequest)
		return
	}

	presence := h.connMgr.GetHub().GetChannelPresence(channel)
	writeJSON(w, map[string]interface{}{
		"channel":  channel,
		"presence": presence,
		"count":    len(presence),
	})
}

// GetFileLocks returns active file locks for a share
func (h *RealtimeHandler) GetFileLocks(w http.ResponseWriter, r *http.Request) {
	shareID := chi.URLParam(r, "share_id")
	if shareID == "" {
		http.Error(w, "Share ID required", http.StatusBadRequest)
		return
	}

	// In a full implementation, we'd filter by share ID
	// For now, return empty list as locks are managed in-memory
	writeJSON(w, map[string]interface{}{
		"share_id": shareID,
		"locks":    []interface{}{},
	})
}

// NotifyFileChange sends a file change notification to subscribers
func (h *RealtimeHandler) NotifyFileChange(shareID, path string, changeType string, payload interface{}) {
	channel := "share:" + shareID

	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"share_id":    shareID,
		"path":        path,
		"change_type": changeType,
		"details":     payload,
	})

	msg := &realtime.Message{
		Type:    realtime.MessageType("file." + changeType),
		Channel: channel,
		Payload: payloadBytes,
	}

	h.connMgr.GetHub().Broadcast(msg)
}

// Shutdown gracefully shuts down the realtime handler
func (h *RealtimeHandler) Shutdown() {
	h.connMgr.GetHub().Stop()
}

