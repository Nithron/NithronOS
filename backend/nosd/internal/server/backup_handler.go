package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"nos/backend/nosd/pkg/backup"
)

// BackupHandler provides API handlers for backup operations
type BackupHandler struct {
	logger     zerolog.Logger
	scheduler  *backup.Scheduler
	replicator *backup.Replicator
	restorer   *backup.Restorer
}

// NewBackupHandler creates a new backup handler
func NewBackupHandler(logger zerolog.Logger, scheduler *backup.Scheduler, replicator *backup.Replicator, restorer *backup.Restorer) *BackupHandler {
	return &BackupHandler{
		logger:     logger.With().Str("component", "backup-handler").Logger(),
		scheduler:  scheduler,
		replicator: replicator,
		restorer:   restorer,
	}
}

// Routes returns the backup API routes
func (h *BackupHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Schedules
	r.Route("/schedules", func(r chi.Router) {
		r.Get("/", h.ListSchedules)
		r.Post("/", h.CreateSchedule)
		r.Get("/{id}", h.GetSchedule)
		r.Patch("/{id}", h.UpdateSchedule)
		r.Delete("/{id}", h.DeleteSchedule)
	})

	// Snapshots
	r.Route("/snapshots", func(r chi.Router) {
		r.Get("/", h.ListSnapshots)
		r.Post("/create", h.CreateSnapshot)
		r.Delete("/{id}", h.DeleteSnapshot)
		r.Get("/stats", h.GetSnapshotStats)
	})

	// Destinations
	r.Route("/destinations", func(r chi.Router) {
		r.Get("/", h.ListDestinations)
		r.Post("/", h.CreateDestination)
		r.Get("/{id}", h.GetDestination)
		r.Patch("/{id}", h.UpdateDestination)
		r.Delete("/{id}", h.DeleteDestination)
		r.Post("/{id}/test", h.TestDestination)
		r.Post("/{id}/key", h.StoreSSHKey)
	})

	// Replication
	r.Post("/replicate", h.StartReplication)

	// Restore
	r.Route("/restore", func(r chi.Router) {
		r.Post("/plan", h.CreateRestorePlan)
		r.Post("/apply", h.ApplyRestore)
		r.Get("/points", h.ListRestorePoints)
	})

	// Jobs
	r.Route("/jobs", func(r chi.Router) {
		r.Get("/", h.ListJobs)
		r.Get("/{id}", h.GetJob)
		r.Post("/{id}/cancel", h.CancelJob)
	})

	return r
}

// Schedule handlers

func (h *BackupHandler) ListSchedules(w http.ResponseWriter, r *http.Request) {
	schedules := h.scheduler.ListSchedules()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"schedules": schedules,
	})
}

func (h *BackupHandler) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	var schedule backup.Schedule
	if err := json.NewDecoder(r.Body).Decode(&schedule); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.scheduler.CreateSchedule(&schedule); err != nil {
		h.logger.Error().Err(err).Msg("Failed to create schedule")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, schedule)
}

func (h *BackupHandler) GetSchedule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	schedule, err := h.scheduler.GetSchedule(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Schedule not found")
		return
	}

	respondJSON(w, http.StatusOK, schedule)
}

func (h *BackupHandler) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var schedule backup.Schedule
	if err := json.NewDecoder(r.Body).Decode(&schedule); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.scheduler.UpdateSchedule(id, &schedule); err != nil {
		h.logger.Error().Err(err).Msg("Failed to update schedule")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, schedule)
}

func (h *BackupHandler) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.scheduler.DeleteSchedule(id); err != nil {
		h.logger.Error().Err(err).Msg("Failed to delete schedule")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// Snapshot handlers

func (h *BackupHandler) ListSnapshots(w http.ResponseWriter, r *http.Request) {
	snapshots := h.scheduler.ListSnapshots()

	// Filter by subvolume if specified
	if subvol := r.URL.Query().Get("subvolume"); subvol != "" {
		filtered := []*backup.Snapshot{}
		for _, snap := range snapshots {
			if snap.Subvolume == subvol {
				filtered = append(filtered, snap)
			}
		}
		snapshots = filtered
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"snapshots": snapshots,
	})
}

func (h *BackupHandler) CreateSnapshot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Subvolumes []string `json:"subvolumes"`
		Tag        string   `json:"tag,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.Subvolumes) == 0 {
		respondError(w, http.StatusBadRequest, "At least one subvolume is required")
		return
	}

	job, err := h.scheduler.CreateSnapshot(req.Subvolumes, req.Tag)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to create snapshot")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusAccepted, job)
}

func (h *BackupHandler) DeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.scheduler.DeleteSnapshot(id); err != nil {
		h.logger.Error().Err(err).Msg("Failed to delete snapshot")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *BackupHandler) GetSnapshotStats(w http.ResponseWriter, r *http.Request) {
	stats := h.scheduler.GetSnapshotStats()
	respondJSON(w, http.StatusOK, stats)
}

// Destination handlers

func (h *BackupHandler) ListDestinations(w http.ResponseWriter, r *http.Request) {
	destinations := h.replicator.ListDestinations()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"destinations": destinations,
	})
}

func (h *BackupHandler) CreateDestination(w http.ResponseWriter, r *http.Request) {
	var dest backup.Destination
	if err := json.NewDecoder(r.Body).Decode(&dest); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.replicator.CreateDestination(&dest); err != nil {
		h.logger.Error().Err(err).Msg("Failed to create destination")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, dest)
}

func (h *BackupHandler) GetDestination(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	dest, err := h.replicator.GetDestination(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Destination not found")
		return
	}

	respondJSON(w, http.StatusOK, dest)
}

func (h *BackupHandler) UpdateDestination(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var dest backup.Destination
	if err := json.NewDecoder(r.Body).Decode(&dest); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.replicator.UpdateDestination(id, &dest); err != nil {
		h.logger.Error().Err(err).Msg("Failed to update destination")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, dest)
}

func (h *BackupHandler) DeleteDestination(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.replicator.DeleteDestination(id); err != nil {
		h.logger.Error().Err(err).Msg("Failed to delete destination")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *BackupHandler) TestDestination(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.replicator.TestDestination(id); err != nil {
		h.logger.Error().Err(err).Msg("Destination test failed")
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

func (h *BackupHandler) StoreSSHKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Key string `json:"key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Key == "" {
		respondError(w, http.StatusBadRequest, "SSH key is required")
		return
	}

	if err := h.replicator.StoreSSHKey(id, req.Key); err != nil {
		h.logger.Error().Err(err).Msg("Failed to store SSH key")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "stored"})
}

// Replication handlers

func (h *BackupHandler) StartReplication(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DestinationID  string `json:"destination_id"`
		SnapshotID     string `json:"snapshot_id"`
		BaseSnapshotID string `json:"base_snapshot_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.DestinationID == "" || req.SnapshotID == "" {
		respondError(w, http.StatusBadRequest, "destination_id and snapshot_id are required")
		return
	}

	job, err := h.replicator.Replicate(req.DestinationID, req.SnapshotID, req.BaseSnapshotID)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to start replication")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusAccepted, job)
}

// Restore handlers

func (h *BackupHandler) CreateRestorePlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceType  string `json:"source_type"`
		SourceID    string `json:"source_id"`
		RestoreType string `json:"restore_type"`
		TargetPath  string `json:"target_path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Always dry-run for plan creation
	plan, err := h.restorer.CreateRestorePlan(req.SourceType, req.SourceID, req.RestoreType, req.TargetPath, true)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to create restore plan")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, plan)
}

func (h *BackupHandler) ApplyRestore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceType  string `json:"source_type"`
		SourceID    string `json:"source_id"`
		RestoreType string `json:"restore_type"`
		TargetPath  string `json:"target_path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Create plan without dry-run
	plan, err := h.restorer.CreateRestorePlan(req.SourceType, req.SourceID, req.RestoreType, req.TargetPath, false)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to create restore plan")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Execute restore
	job, err := h.restorer.ExecuteRestore(plan)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to execute restore")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusAccepted, job)
}

func (h *BackupHandler) ListRestorePoints(w http.ResponseWriter, r *http.Request) {
	points, err := h.restorer.ListRestorePoints()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to list restore points")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"restore_points": points,
	})
}

// Job handlers

func (h *BackupHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	// Check for limit parameter
	limit := 0
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	var jobs []*backup.BackupJob
	if limit > 0 {
		jobs = h.scheduler.GetJobManager().ListRecentJobs(limit)
	} else {
		jobs = h.scheduler.GetJobManager().ListJobs()
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"jobs": jobs,
	})
}

func (h *BackupHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	job, ok := h.scheduler.GetJobManager().GetJob(id)
	if !ok {
		respondError(w, http.StatusNotFound, "Job not found")
		return
	}

	respondJSON(w, http.StatusOK, job)
}

func (h *BackupHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.scheduler.GetJobManager().CancelJob(id); err != nil {
		h.logger.Error().Err(err).Msg("Failed to cancel job")
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "canceled"})
}

// Helper for JSON responses
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		fmt.Fprintf(w, `{"error": "Failed to encode response"}`)
	}
}

// Helper for error responses
func respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
