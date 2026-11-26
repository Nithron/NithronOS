package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"nos/backend/nosd/pkg/alerts"
	"nos/backend/nosd/pkg/monitor"
)

// MonitorHandler provides API handlers for monitoring
type MonitorHandler struct {
	logger      zerolog.Logger
	collector   *monitor.Collector
	storage     *monitor.TimeSeriesStorage
	alertEngine *alerts.Engine
}

// NewMonitorHandler creates a new monitor handler
func NewMonitorHandler(logger zerolog.Logger, collector *monitor.Collector, storage *monitor.TimeSeriesStorage, alertEngine *alerts.Engine) *MonitorHandler {
	return &MonitorHandler{
		logger:      logger.With().Str("component", "monitor-handler").Logger(),
		collector:   collector,
		storage:     storage,
		alertEngine: alertEngine,
	}
}

// Routes returns the monitoring API routes. Alert rules, channels (which
// hold SMTP passwords and webhook URLs) and events are for admins.
func (h *MonitorHandler) Routes(adminRequired func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()

	// Metrics
	r.Get("/overview", h.GetOverview)
	r.Post("/timeseries", h.QueryTimeSeries)
	r.Get("/devices", h.GetDevices)
	r.Get("/services", h.GetServices)
	r.Get("/btrfs", h.GetBtrfsMetrics)

	// Alerts
	r.Route("/alerts", func(r chi.Router) {
		r.Use(adminRequired)
		// Rules
		r.Get("/rules", h.ListAlertRules)
		r.Post("/rules", h.CreateAlertRule)
		r.Get("/rules/{id}", h.GetAlertRule)
		r.Patch("/rules/{id}", h.UpdateAlertRule)
		r.Delete("/rules/{id}", h.DeleteAlertRule)

		// Channels
		r.Get("/channels", h.ListChannels)
		r.Post("/channels", h.CreateChannel)
		r.Get("/channels/{id}", h.GetChannel)
		r.Patch("/channels/{id}", h.UpdateChannel)
		r.Delete("/channels/{id}", h.DeleteChannel)
		r.Post("/channels/{id}/test", h.TestChannel)

		// Events
		r.Get("/events", h.ListAlertEvents)
	})

	return r
}

// Metrics handlers

func (h *MonitorHandler) GetOverview(w http.ResponseWriter, r *http.Request) {
	overview, err := h.collector.GetOverview()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get overview")
		respondError(w, http.StatusInternalServerError, "Failed to get overview")
		return
	}

	// Add active alerts count
	events := h.alertEngine.ListEvents(100)
	activeCount := 0
	for _, event := range events {
		if event.State == "firing" && event.ClearedAt == nil {
			activeCount++
		}
	}
	overview.AlertsActive = activeCount

	respondJSON(w, http.StatusOK, overview)
}

func (h *MonitorHandler) QueryTimeSeries(w http.ResponseWriter, r *http.Request) {
	var query monitor.TimeSeriesQuery
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Default time range if not specified
	if query.EndTime.IsZero() {
		query.EndTime = time.Now()
	}
	if query.StartTime.IsZero() {
		query.StartTime = query.EndTime.Add(-1 * time.Hour)
	}

	// Default step if not specified
	if query.Step == 0 {
		duration := query.EndTime.Sub(query.StartTime)
		if duration <= time.Hour {
			query.Step = time.Minute
		} else if duration <= 24*time.Hour {
			query.Step = 5 * time.Minute
		} else {
			query.Step = time.Hour
		}
	}

	ts, err := h.storage.Query(query)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to query time series")
		respondError(w, http.StatusInternalServerError, "Failed to query metrics")
		return
	}

	respondJSON(w, http.StatusOK, ts)
}

func (h *MonitorHandler) GetDevices(w http.ResponseWriter, r *http.Request) {
	// Get disk metrics with SMART data
	overview, err := h.collector.GetOverview()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get devices")
		respondError(w, http.StatusInternalServerError, "Failed to get devices")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"disks": overview.Disks,
	})
}

func (h *MonitorHandler) GetServices(w http.ResponseWriter, r *http.Request) {
	overview, err := h.collector.GetOverview()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get services")
		respondError(w, http.StatusInternalServerError, "Failed to get services")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"services": overview.Services,
	})
}

func (h *MonitorHandler) GetBtrfsMetrics(w http.ResponseWriter, r *http.Request) {
	overview, err := h.collector.GetOverview()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get btrfs metrics")
		respondError(w, http.StatusInternalServerError, "Failed to get btrfs metrics")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"btrfs": overview.Btrfs,
	})
}

// Alert handlers

func (h *MonitorHandler) ListAlertRules(w http.ResponseWriter, r *http.Request) {
	rules := h.alertEngine.ListRules()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"rules": rules,
	})
}

func (h *MonitorHandler) CreateAlertRule(w http.ResponseWriter, r *http.Request) {
	var rule alerts.AlertRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.alertEngine.CreateRule(&rule); err != nil {
		h.logger.Error().Err(err).Msg("Failed to create alert rule")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, rule)
}

func (h *MonitorHandler) GetAlertRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	rule, err := h.alertEngine.GetRule(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Rule not found")
		return
	}

	respondJSON(w, http.StatusOK, rule)
}

func (h *MonitorHandler) UpdateAlertRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var rule alerts.AlertRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.alertEngine.UpdateRule(id, &rule); err != nil {
		h.logger.Error().Err(err).Msg("Failed to update alert rule")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, rule)
}

func (h *MonitorHandler) DeleteAlertRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.alertEngine.DeleteRule(id); err != nil {
		h.logger.Error().Err(err).Msg("Failed to delete alert rule")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *MonitorHandler) ListChannels(w http.ResponseWriter, r *http.Request) {
	channels := h.alertEngine.ListChannels()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"channels": channels,
	})
}

func (h *MonitorHandler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	var channel alerts.NotificationChannel
	if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.alertEngine.CreateChannel(&channel); err != nil {
		h.logger.Error().Err(err).Msg("Failed to create channel")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, channel)
}

func (h *MonitorHandler) GetChannel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	channel, err := h.alertEngine.GetChannel(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Channel not found")
		return
	}

	// Hide sensitive config
	switch channel.Type {
	case "email":
		if cfg, ok := channel.Config["smtp_password"]; ok && cfg != "" {
			channel.Config["smtp_password"] = "***"
		}
	case "webhook":
		if cfg, ok := channel.Config["secret"]; ok && cfg != "" {
			channel.Config["secret"] = "***"
		}
	case "ntfy":
		if cfg, ok := channel.Config["password"]; ok && cfg != "" {
			channel.Config["password"] = "***"
		}
		if cfg, ok := channel.Config["token"]; ok && cfg != "" {
			channel.Config["token"] = "***"
		}
	}

	respondJSON(w, http.StatusOK, channel)
}

func (h *MonitorHandler) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Get existing channel to preserve sensitive fields
	existing, err := h.alertEngine.GetChannel(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Channel not found")
		return
	}

	var channel alerts.NotificationChannel
	if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Preserve sensitive fields if not provided
	switch channel.Type {
	case "email":
		if pwd, ok := channel.Config["smtp_password"].(string); !ok || pwd == "***" || pwd == "" {
			if existingPwd, ok := existing.Config["smtp_password"]; ok {
				channel.Config["smtp_password"] = existingPwd
			}
		}
	case "webhook":
		if secret, ok := channel.Config["secret"].(string); !ok || secret == "***" || secret == "" {
			if existingSecret, ok := existing.Config["secret"]; ok {
				channel.Config["secret"] = existingSecret
			}
		}
	case "ntfy":
		if pwd, ok := channel.Config["password"].(string); !ok || pwd == "***" || pwd == "" {
			if existingPwd, ok := existing.Config["password"]; ok {
				channel.Config["password"] = existingPwd
			}
		}
		if token, ok := channel.Config["token"].(string); !ok || token == "***" || token == "" {
			if existingToken, ok := existing.Config["token"]; ok {
				channel.Config["token"] = existingToken
			}
		}
	}

	if err := h.alertEngine.UpdateChannel(id, &channel); err != nil {
		h.logger.Error().Err(err).Msg("Failed to update channel")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, channel)
}

func (h *MonitorHandler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.alertEngine.DeleteChannel(id); err != nil {
		h.logger.Error().Err(err).Msg("Failed to delete channel")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *MonitorHandler) TestChannel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.alertEngine.TestChannel(id); err != nil {
		h.logger.Error().Err(err).Msg("Test notification failed")
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

func (h *MonitorHandler) ListAlertEvents(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	events := h.alertEngine.ListEvents(limit)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"events": events,
	})
}
