package server

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"nithronos/backend/nosd/internal/config"
	"nithronos/backend/nosd/internal/httpx"
	"nithronos/backend/nosd/pkg/sync/smartsync"
)

// SmartSyncHandler handles smart sync (on-demand files) API endpoints
type SmartSyncHandler struct {
	manager *smartsync.SmartSyncManager
	logger  zerolog.Logger
	cfg     config.Config
}

// NewSmartSyncHandler creates a new smart sync handler
func NewSmartSyncHandler(cfg config.Config, logger zerolog.Logger) (*SmartSyncHandler, error) {
	dataDir := filepath.Join(cfg.DataDir, "sync", "smartsync")
	manager, err := smartsync.NewSmartSyncManager(dataDir, logger)
	if err != nil {
		return nil, err
	}

	h := &SmartSyncHandler{
		manager: manager,
		logger:  logger.With().Str("component", "smartsync-handler").Logger(),
		cfg:     cfg,
	}

	// Start the manager
	manager.Start()

	return h, nil
}

// Routes returns the chi router for smart sync endpoints
func (h *SmartSyncHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Status and settings
	r.Get("/status", h.GetStatus)
	r.Get("/stats", h.GetStats)
	r.Get("/policy", h.GetPolicy)
	r.Put("/policy", h.UpdatePolicy)

	// Placeholder management
	r.Get("/placeholders", h.ListPlaceholders)
	r.Get("/placeholders/{share_id}", h.ListSharePlaceholders)
	r.Get("/placeholders/{share_id}/{path}", h.GetPlaceholder)

	// Hydration
	r.Post("/hydrate", h.RequestHydration)
	r.Delete("/hydrate/{share_id}/{path}", h.CancelHydration)
	r.Get("/hydration-queue", h.GetHydrationQueue)

	// Pinning
	r.Post("/pin", h.PinFile)
	r.Delete("/pin/{share_id}/{path}", h.UnpinFile)
	r.Get("/pinned", h.ListPinnedFiles)

	// Dehydration
	r.Post("/dehydrate", h.DehydrateFile)

	// Cloud-only files
	r.Get("/cloud-only", h.ListCloudOnlyFiles)
	r.Get("/local", h.ListLocalFiles)

	return r
}

// GetStatus returns the smart sync status
func (h *SmartSyncHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.GetStats()
	policy := h.manager.GetPolicy()

	writeJSON(w, map[string]interface{}{
		"enabled":         policy.Enabled,
		"total_files":     stats.TotalFiles,
		"cloud_only":      stats.CloudOnlyFiles,
		"local_files":     stats.LocalFiles,
		"pinned_files":    stats.PinnedFiles,
		"hydrating_files": stats.HydratingFiles,
		"queue_length":    stats.QueueLength,
		"local_size":      stats.LocalSize,
		"cloud_size":      stats.CloudOnlySize,
	})
}

// GetStats returns detailed smart sync statistics
func (h *SmartSyncHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.GetStats()
	writeJSON(w, stats)
}

// GetPolicy returns the dehydration policy
func (h *SmartSyncHandler) GetPolicy(w http.ResponseWriter, r *http.Request) {
	policy := h.manager.GetPolicy()
	writeJSON(w, policy)
}

// UpdatePolicy updates the dehydration policy
func (h *SmartSyncHandler) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	var policy smartsync.DehydrationPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body", 0)
		return
	}

	h.manager.SetPolicy(policy)
	writeJSON(w, policy)
}

// ListPlaceholders returns all placeholder files
func (h *SmartSyncHandler) ListPlaceholders(w http.ResponseWriter, r *http.Request) {
	shareID := r.URL.Query().Get("share_id")
	
	var placeholders []*smartsync.PlaceholderFile
	if shareID != "" {
		placeholders = h.manager.ListPlaceholders(shareID)
	} else {
		// List all - would need to aggregate from all shares
		placeholders = []*smartsync.PlaceholderFile{}
	}

	writeJSON(w, placeholders)
}

// ListSharePlaceholders returns placeholders for a specific share
func (h *SmartSyncHandler) ListSharePlaceholders(w http.ResponseWriter, r *http.Request) {
	shareID := chi.URLParam(r, "share_id")
	if shareID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "Share ID required", 0)
		return
	}

	placeholders := h.manager.ListPlaceholders(shareID)
	writeJSON(w, placeholders)
}

// GetPlaceholder returns a specific placeholder file
func (h *SmartSyncHandler) GetPlaceholder(w http.ResponseWriter, r *http.Request) {
	shareID := chi.URLParam(r, "share_id")
	path := chi.URLParam(r, "path")

	if shareID == "" || path == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "Share ID and path required", 0)
		return
	}

	placeholder := h.manager.GetPlaceholder(shareID, path)
	if placeholder == nil {
		httpx.WriteTypedError(w, http.StatusNotFound, "file.not_found", "Placeholder not found", 0)
		return
	}

	writeJSON(w, placeholder)
}

// RequestHydration requests a file to be downloaded
func (h *SmartSyncHandler) RequestHydration(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ShareID  string `json:"share_id"`
		Path     string `json:"path"`
		Priority int    `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body", 0)
		return
	}

	if req.ShareID == "" || req.Path == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "share_id and path are required", 0)
		return
	}

	priority := smartsync.HydrationPriority(req.Priority)
	if priority == 0 {
		priority = smartsync.PriorityNormal
	}

	if err := h.manager.RequestHydration(req.ShareID, req.Path, priority, nil); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "hydration.failed", err.Error(), 0)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// CancelHydration cancels a pending hydration request
func (h *SmartSyncHandler) CancelHydration(w http.ResponseWriter, r *http.Request) {
	shareID := chi.URLParam(r, "share_id")
	path := chi.URLParam(r, "path")

	if shareID == "" || path == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "Share ID and path required", 0)
		return
	}

	if err := h.manager.CancelHydration(shareID, path); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "hydration.cancel_failed", err.Error(), 0)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetHydrationQueue returns the current hydration queue
func (h *SmartSyncHandler) GetHydrationQueue(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.GetStats()
	writeJSON(w, map[string]interface{}{
		"queue_length":    stats.QueueLength,
		"hydrating_files": stats.HydratingFiles,
	})
}

// PinFile pins a file to always keep it local
func (h *SmartSyncHandler) PinFile(w http.ResponseWriter, r *http.Request) {
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
		httpx.WriteTypedError(w, http.StatusBadRequest, "pin.failed", err.Error(), 0)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UnpinFile unpins a file
func (h *SmartSyncHandler) UnpinFile(w http.ResponseWriter, r *http.Request) {
	shareID := chi.URLParam(r, "share_id")
	path := chi.URLParam(r, "path")

	if shareID == "" || path == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "Share ID and path required", 0)
		return
	}

	if err := h.manager.UnpinFile(shareID, path); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "unpin.failed", err.Error(), 0)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListPinnedFiles returns all pinned files
func (h *SmartSyncHandler) ListPinnedFiles(w http.ResponseWriter, r *http.Request) {
	shareID := r.URL.Query().Get("share_id")
	
	stats := h.manager.GetStats()
	writeJSON(w, map[string]interface{}{
		"share_id":     shareID,
		"pinned_count": stats.PinnedFiles,
		"pinned_size":  stats.PinnedSize,
	})
}

// DehydrateFile converts a local file to a placeholder
func (h *SmartSyncHandler) DehydrateFile(w http.ResponseWriter, r *http.Request) {
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

	if err := h.manager.Dehydrate(req.ShareID, req.Path); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "dehydrate.failed", err.Error(), 0)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListCloudOnlyFiles returns files that are only in the cloud
func (h *SmartSyncHandler) ListCloudOnlyFiles(w http.ResponseWriter, r *http.Request) {
	shareID := r.URL.Query().Get("share_id")
	if shareID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "share_id is required", 0)
		return
	}

	files := h.manager.GetCloudOnlyFiles(shareID)
	writeJSON(w, files)
}

// ListLocalFiles returns files available locally
func (h *SmartSyncHandler) ListLocalFiles(w http.ResponseWriter, r *http.Request) {
	shareID := r.URL.Query().Get("share_id")
	if shareID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "share_id is required", 0)
		return
	}

	files := h.manager.GetLocalFiles(shareID)
	writeJSON(w, files)
}

// Shutdown gracefully shuts down the smart sync handler
func (h *SmartSyncHandler) Shutdown() {
	h.manager.Stop()
}

// GetManager returns the smart sync manager
func (h *SmartSyncHandler) GetManager() *smartsync.SmartSyncManager {
	return h.manager
}

// RegisterPlaceholder registers a new placeholder file
func (h *SmartSyncHandler) RegisterPlaceholder(shareID, path, name string, size int64, hash string, modifiedAt time.Time) error {
	placeholder := &smartsync.PlaceholderFile{
		ShareID:    shareID,
		Path:       path,
		Name:       name,
		Size:       size,
		Hash:       hash,
		ModifiedAt: modifiedAt,
		State:      smartsync.StateCloud,
	}
	return h.manager.RegisterPlaceholder(placeholder)
}

