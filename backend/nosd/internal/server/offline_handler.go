package server

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"nithronos/backend/nosd/internal/config"
	"nithronos/backend/nosd/pkg/httpx"
	"nithronos/backend/nosd/pkg/sync/offline"
)

// OfflineHandler handles offline sync API endpoints
type OfflineHandler struct {
	manager *offline.OfflineManager
	logger  zerolog.Logger
	cfg     config.Config
}

// NewOfflineHandler creates a new offline handler
func NewOfflineHandler(cfg config.Config, logger zerolog.Logger) (*OfflineHandler, error) {
	dataDir := filepath.Join(cfg.AppsDataDir, "..", "sync", "offline")
	manager, err := offline.NewOfflineManager(dataDir, logger)
	if err != nil {
		return nil, err
	}

	h := &OfflineHandler{
		manager: manager,
		logger:  logger.With().Str("component", "offline-handler").Logger(),
		cfg:     cfg,
	}

	// Start the manager
	manager.Start()

	return h, nil
}

// Routes returns the chi router for offline endpoints
func (h *OfflineHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Status
	r.Get("/status", h.GetStatus)
	r.Put("/mode", h.SetMode)

	// Queue management
	r.Get("/queue", h.ListQueue)
	r.Get("/queue/stats", h.GetQueueStats)
	r.Post("/queue/{operation_id}/retry", h.RetryOperation)
	r.Post("/queue/{operation_id}/cancel", h.CancelOperation)
	r.Post("/queue/retry-all", h.RetryAllFailed)
	r.Post("/queue/clear", h.ClearCompleted)

	// Sync triggers
	r.Post("/sync", h.TriggerSync)

	// Local state
	r.Get("/changes", h.ListPendingChanges)
	r.Get("/pinned", h.ListPinnedFiles)
	r.Post("/pin", h.PinFile)
	r.Post("/unpin", h.UnpinFile)

	// Conflicts
	r.Get("/conflicts", h.ListConflicts)
	r.Put("/conflicts/{conflict_id}", h.ResolveConflict)

	return r
}

// GetStatus returns the offline sync status
func (h *OfflineHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	status := h.manager.GetStatus()
	writeJSON(w, status)
}

// SetMode sets the sync mode
func (h *OfflineHandler) SetMode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body", 0)
		return
	}

	var mode offline.SyncMode
	switch req.Mode {
	case "online":
		mode = offline.ModeOnline
	case "offline":
		mode = offline.ModeOffline
	default:
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid mode", 0)
		return
	}

	h.manager.SetMode(mode)
	w.WriteHeader(http.StatusNoContent)
}

// ListQueue returns queued operations
func (h *OfflineHandler) ListQueue(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")

	queue := h.manager.GetQueue()
	var operations []*offline.QueuedOperation

	if statusFilter != "" {
		// Filter by status
		all := queue.GetAll()
		for _, op := range all {
			if string(op.Status) == statusFilter {
				operations = append(operations, op)
			}
		}
	} else {
		operations = queue.GetAll()
	}

	writeJSON(w, operations)
}

// GetQueueStats returns queue statistics
func (h *OfflineHandler) GetQueueStats(w http.ResponseWriter, r *http.Request) {
	queue := h.manager.GetQueue()
	stats := queue.GetStats()
	writeJSON(w, stats)
}

// RetryOperation retries a failed operation
func (h *OfflineHandler) RetryOperation(w http.ResponseWriter, r *http.Request) {
	operationID := chi.URLParam(r, "operation_id")
	if operationID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "Operation ID required", 0)
		return
	}

	queue := h.manager.GetQueue()
	if err := queue.Retry(operationID); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "queue.retry_failed", err.Error(), 0)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CancelOperation cancels a pending operation
func (h *OfflineHandler) CancelOperation(w http.ResponseWriter, r *http.Request) {
	operationID := chi.URLParam(r, "operation_id")
	if operationID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "Operation ID required", 0)
		return
	}

	queue := h.manager.GetQueue()
	if err := queue.Cancel(operationID); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "queue.cancel_failed", err.Error(), 0)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RetryAllFailed retries all failed operations
func (h *OfflineHandler) RetryAllFailed(w http.ResponseWriter, r *http.Request) {
	queue := h.manager.GetQueue()
	if err := queue.RetryAll(); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "queue.retry_failed", err.Error(), 0)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ClearCompleted clears completed operations
func (h *OfflineHandler) ClearCompleted(w http.ResponseWriter, r *http.Request) {
	queue := h.manager.GetQueue()
	if err := queue.Clear(); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "queue.clear_failed", err.Error(), 0)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// TriggerSync triggers a manual sync
func (h *OfflineHandler) TriggerSync(w http.ResponseWriter, r *http.Request) {
	h.manager.TriggerSync()
	w.WriteHeader(http.StatusAccepted)
}

// ListPendingChanges returns files with pending local changes
func (h *OfflineHandler) ListPendingChanges(w http.ResponseWriter, r *http.Request) {
	changes := h.manager.GetPendingChanges()
	writeJSON(w, changes)
}

// ListPinnedFiles returns pinned files
func (h *OfflineHandler) ListPinnedFiles(w http.ResponseWriter, r *http.Request) {
	shareID := r.URL.Query().Get("share_id")
	files := h.manager.GetPinnedFiles(shareID)
	writeJSON(w, files)
}

// PinFile pins a file for offline access
func (h *OfflineHandler) PinFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ShareID string `json:"share_id"`
		Path    string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body", 0)
		return
	}

	if req.ShareID == "" || req.Path == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "share_id and path are required", 0)
		return
	}

	if err := h.manager.PinFile(req.ShareID, req.Path); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "pin.failed", err.Error(), 0)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UnpinFile unpins a file
func (h *OfflineHandler) UnpinFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ShareID string `json:"share_id"`
		Path    string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body", 0)
		return
	}

	if req.ShareID == "" || req.Path == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "share_id and path are required", 0)
		return
	}

	if err := h.manager.UnpinFile(req.ShareID, req.Path); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "unpin.failed", err.Error(), 0)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListConflicts returns sync conflicts
func (h *OfflineHandler) ListConflicts(w http.ResponseWriter, r *http.Request) {
	queue := h.manager.GetQueue()
	conflicts := queue.GetConflicts()
	writeJSON(w, conflicts)
}

// ResolveConflict resolves a sync conflict
func (h *OfflineHandler) ResolveConflict(w http.ResponseWriter, r *http.Request) {
	conflictID := chi.URLParam(r, "conflict_id")
	if conflictID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "Conflict ID required", 0)
		return
	}

	var req struct {
		Resolution string `json:"resolution"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body", 0)
		return
	}

	resolution := offline.ConflictResolution(req.Resolution)
	validResolutions := map[offline.ConflictResolution]bool{
		offline.ResolutionKeepLocal:  true,
		offline.ResolutionKeepRemote: true,
		offline.ResolutionKeepBoth:   true,
		offline.ResolutionMerge:      true,
		offline.ResolutionManual:     true,
	}

	if !validResolutions[resolution] {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid resolution", 0)
		return
	}

	queue := h.manager.GetQueue()
	if err := queue.ResolveConflict(conflictID, resolution, ""); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "conflict.resolve_failed", err.Error(), 0)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Shutdown gracefully shuts down the offline handler
func (h *OfflineHandler) Shutdown() {
	h.manager.Stop()
}

// GetManager returns the offline manager
func (h *OfflineHandler) GetManager() *offline.OfflineManager {
	return h.manager
}

