package server

import (
	"encoding/json"
	"net/http"
	"time"

	"nos/backend/nosd/internal/notifications"
	"nos/backend/nosd/pkg/httpx"

	"github.com/go-chi/chi/v5"
)

// NotificationHandler handles notification API endpoints
type NotificationHandler struct {
	manager *notifications.Manager
}

// NewNotificationHandler creates a new notification handler
func NewNotificationHandler(manager *notifications.Manager) *NotificationHandler {
	return &NotificationHandler{
		manager: manager,
	}
}

// Routes registers notification routes
func (h *NotificationHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Notifications
	r.Get("/", h.ListNotifications)
	r.Get("/{id}", h.GetNotification)
	r.Put("/{id}/read", h.MarkRead)
	r.Put("/read-all", h.MarkAllRead)
	r.Delete("/{id}", h.DeleteNotification)
	r.Get("/subscribe", h.Subscribe)

	// Channels
	r.Get("/channels", h.ListChannels)
	r.Post("/channels", h.CreateChannel)
	r.Get("/channels/{id}", h.GetChannel)
	r.Put("/channels/{id}", h.UpdateChannel)
	r.Delete("/channels/{id}", h.DeleteChannel)
	r.Post("/channels/{id}/test", h.TestChannel)

	return r
}

// ListNotifications returns all notifications
func (h *NotificationHandler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	unreadOnly := r.URL.Query().Get("unread") == "true"
	notifications := h.manager.List(unreadOnly)
	writeJSON(w, notifications)
}

// GetNotification returns a specific notification
func (h *NotificationHandler) GetNotification(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	notif, ok := h.manager.Get(id)
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "Notification not found")
		return
	}

	writeJSON(w, notif)
}

// MarkRead marks a notification as read
func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.manager.MarkRead(id); err != nil {
		httpx.WriteError(w, http.StatusNotFound, "Notification not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// MarkAllRead marks all notifications as read
func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	if err := h.manager.MarkAllRead(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "Failed to mark notifications as read")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteNotification deletes a notification
func (h *NotificationHandler) DeleteNotification(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.manager.Delete(id); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "Failed to delete notification")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Subscribe creates a real-time subscription for notifications
func (h *NotificationHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Get client ID from session
	clientID := r.Header.Get("X-UID")
	if clientID == "" {
		clientID = "anonymous"
	}

	// Create subscription
	ch := h.manager.Subscribe(clientID)
	defer h.manager.Unsubscribe(clientID, ch)

	// Send initial ping
	_, _ = w.Write([]byte("event: ping\ndata: {}\n\n"))
	w.(http.Flusher).Flush()

	// Listen for notifications
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case notif := <-ch:
			data, _ := json.Marshal(notif)
			_, _ = w.Write([]byte("event: notification\ndata: "))
			_, _ = w.Write(data)
			_, _ = w.Write([]byte("\n\n"))
			w.(http.Flusher).Flush()

		case <-ticker.C:
			// Send keepalive
			_, _ = w.Write([]byte("event: ping\ndata: {}\n\n"))
			w.(http.Flusher).Flush()

		case <-r.Context().Done():
			return
		}
	}
}

// ListChannels returns all notification channels
func (h *NotificationHandler) ListChannels(w http.ResponseWriter, r *http.Request) {
	channels := h.manager.ListChannels()

	// Hide sensitive config
	for _, ch := range channels {
		ch.Config = h.sanitizeConfig(ch.Config, ch.Type)
	}

	writeJSON(w, channels)
}

// GetChannel returns a specific channel
func (h *NotificationHandler) GetChannel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	channel, ok := h.manager.GetChannel(id)
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "Channel not found")
		return
	}

	// Hide sensitive config
	channel.Config = h.sanitizeConfig(channel.Config, channel.Type)
	writeJSON(w, channel)
}

// CreateChannel creates a new notification channel
func (h *NotificationHandler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	var channel notifications.Channel
	if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.manager.CreateChannel(&channel); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "Failed to create channel")
		return
	}

	// Hide sensitive config
	channel.Config = h.sanitizeConfig(channel.Config, channel.Type)
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, channel)
}

// UpdateChannel updates a notification channel
func (h *NotificationHandler) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var updates notifications.Channel
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.manager.UpdateChannel(id, &updates); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "Failed to update channel")
		return
	}

	channel, _ := h.manager.GetChannel(id)
	channel.Config = h.sanitizeConfig(channel.Config, channel.Type)
	writeJSON(w, channel)
}

// DeleteChannel deletes a notification channel
func (h *NotificationHandler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.manager.DeleteChannel(id); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "Failed to delete channel")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// TestChannel tests a notification channel
func (h *NotificationHandler) TestChannel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.manager.TestChannel(id); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": "Test notification sent successfully",
	})
}

// sanitizeConfig removes sensitive information from config
func (h *NotificationHandler) sanitizeConfig(config map[string]interface{}, channelType string) map[string]interface{} {
	sanitized := make(map[string]interface{})
	for k, v := range config {
		switch k {
		case "password", "apiKey", "token", "secret":
			sanitized[k] = "***"
		default:
			sanitized[k] = v
		}
	}
	return sanitized
}
