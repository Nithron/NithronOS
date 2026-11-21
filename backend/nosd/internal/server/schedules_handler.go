package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Schedule represents a scheduled task
type Schedule struct {
	ID      string  `json:"id"`
	Type    string  `json:"type"` // smart_scan, btrfs_scrub, snapshot, backup
	Cron    string  `json:"cron"`
	Enabled bool    `json:"enabled"`
	Target  string  `json:"target,omitempty"` // Pool ID or device for targeted schedules
	LastRun *string `json:"lastRun,omitempty"`
	NextRun *string `json:"nextRun,omitempty"`
}

// SchedulesHandler handles schedule-related endpoints
type SchedulesHandler struct {
	// In a real implementation, this would have a cron scheduler
	schedules []Schedule
}

// NewSchedulesHandler creates a new schedules handler
func NewSchedulesHandler() *SchedulesHandler {
	// Initialize with some default schedules
	now := time.Now()
	lastRun := now.Add(-24 * time.Hour).Format(time.RFC3339)
	nextRun := now.Add(24 * time.Hour).Format(time.RFC3339)

	return &SchedulesHandler{
		schedules: []Schedule{
			{
				ID:      "schedule-smart-1",
				Type:    "smart_scan",
				Cron:    "0 3 * * 0", // Every Sunday at 3 AM
				Enabled: true,
				LastRun: &lastRun,
				NextRun: &nextRun,
			},
			{
				ID:      "schedule-scrub-1",
				Type:    "btrfs_scrub",
				Cron:    "0 3 1-7 * 0", // First Sunday of each month at 3 AM
				Enabled: true,
				Target:  "main-pool",
				LastRun: &lastRun,
				NextRun: &nextRun,
			},
		},
	}
}

// Routes registers the schedules routes
func (h *SchedulesHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.GetSchedules)
	r.Post("/", h.CreateSchedule)
	r.Get("/{id}", h.GetSchedule)
	r.Put("/{id}", h.UpdateSchedule)
	r.Delete("/{id}", h.DeleteSchedule)

	return r
}

// GetSchedules returns all schedules
// GET /api/v1/schedules
func (h *SchedulesHandler) GetSchedules(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(h.schedules); err != nil {
		log.Error().Err(err).Msg("Failed to encode schedules")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// GetSchedule returns a specific schedule
// GET /api/v1/schedules/{id}
func (h *SchedulesHandler) GetSchedule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	for _, schedule := range h.schedules {
		if schedule.ID == id {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(schedule); err != nil {
				fmt.Printf("Failed to write response: %v\n", err)
			}
			return
		}
	}

	http.Error(w, "Schedule not found", http.StatusNotFound)
}

// CreateSchedule creates a new schedule
// POST /api/v1/schedules
func (h *SchedulesHandler) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	var schedule Schedule
	if err := json.NewDecoder(r.Body).Decode(&schedule); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Generate ID if not provided
	if schedule.ID == "" {
		schedule.ID = "schedule-" + uuid.New().String()[:8]
	}

	// Validate schedule type
	validTypes := map[string]bool{
		"smart_scan":  true,
		"btrfs_scrub": true,
		"snapshot":    true,
		"backup":      true,
	}

	if !validTypes[schedule.Type] {
		http.Error(w, "Invalid schedule type", http.StatusBadRequest)
		return
	}

	// Add to schedules
	h.schedules = append(h.schedules, schedule)

	// In real implementation, this would register with cron scheduler
	log.Info().Str("id", schedule.ID).Str("type", schedule.Type).Msg("Created schedule")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(schedule); err != nil {
		fmt.Printf("Failed to write response: %v\n", err)
	}
}

// UpdateSchedule updates an existing schedule
// PUT /api/v1/schedules/{id}
func (h *SchedulesHandler) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var updates Schedule
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	for i, schedule := range h.schedules {
		if schedule.ID == id {
			// Update fields
			if updates.Cron != "" {
				h.schedules[i].Cron = updates.Cron
			}
			if updates.Type != "" {
				h.schedules[i].Type = updates.Type
			}
			h.schedules[i].Enabled = updates.Enabled
			if updates.Target != "" {
				h.schedules[i].Target = updates.Target
			}

			// In real implementation, this would update cron scheduler
			log.Info().Str("id", id).Msg("Updated schedule")

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(h.schedules[i]); err != nil {
				fmt.Printf("Failed to write response: %v\n", err)
			}
			return
		}
	}

	http.Error(w, "Schedule not found", http.StatusNotFound)
}

// DeleteSchedule deletes a schedule
// DELETE /api/v1/schedules/{id}
func (h *SchedulesHandler) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	for i, schedule := range h.schedules {
		if schedule.ID == id {
			// Remove from slice
			h.schedules = append(h.schedules[:i], h.schedules[i+1:]...)

			// In real implementation, this would unregister from cron scheduler
			log.Info().Str("id", id).Msg("Deleted schedule")

			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	http.Error(w, "Schedule not found", http.StatusNotFound)
}
