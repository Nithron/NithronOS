package server

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"nithronos/backend/nosd/internal/config"
	"nithronos/backend/nosd/internal/shares"
	"nithronos/backend/nosd/pkg/httpx"
	nosync "nithronos/backend/nosd/pkg/sync"
)

// SyncHandler handles sync-related API endpoints
type SyncHandler struct {
	deviceMgr         *nosync.DeviceManager
	changeTracker     *nosync.ChangeTracker
	deltaSync         *nosync.DeltaSync
	shareStore        *shares.Store
	syncStore         *nosync.Store
	conflictStore     *nosync.ConflictStore
	activityStore     *nosync.ActivityStore
	collaborationStore *nosync.CollaborationStore
	logger            zerolog.Logger
	cfg               config.Config
}

// NewSyncHandler creates a new sync handler
func NewSyncHandler(cfg config.Config, shareStore *shares.Store, logger zerolog.Logger) (*SyncHandler, error) {
	// Initialize sync store
	syncBasePath := filepath.Join(filepath.Dir(cfg.UsersPath), "sync")
	syncStore, err := nosync.NewStore(syncBasePath)
	if err != nil {
		return nil, err
	}

	// Initialize device manager
	deviceMgr := nosync.NewDeviceManager(
		syncStore,
		logger,
		nosync.DefaultDeviceManagerConfig(),
	)

	// Initialize change tracker
	changeTracker := nosync.NewChangeTracker(
		logger,
		nosync.DefaultChangeTrackerConfig(),
	)

	// Initialize delta sync
	deltaSync := nosync.NewDeltaSync(nosync.DefaultBlockSize)

	// Initialize conflict store
	conflictStore, err := nosync.NewConflictStore(syncBasePath)
	if err != nil {
		return nil, err
	}

	// Initialize activity store
	activityStore, err := nosync.NewActivityStore(syncBasePath, 10000)
	if err != nil {
		return nil, err
	}

	// Initialize collaboration store
	collaborationStore, err := nosync.NewCollaborationStore(syncBasePath)
	if err != nil {
		return nil, err
	}

	return &SyncHandler{
		deviceMgr:          deviceMgr,
		changeTracker:      changeTracker,
		deltaSync:          deltaSync,
		shareStore:         shareStore,
		syncStore:          syncStore,
		conflictStore:      conflictStore,
		activityStore:      activityStore,
		collaborationStore: collaborationStore,
		logger:             logger.With().Str("component", "sync-handler").Logger(),
		cfg:                cfg,
	}, nil
}

// Routes returns the chi router for sync endpoints
func (h *SyncHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Device management (requires user auth initially)
	r.Post("/devices/register", h.RegisterDevice)
	r.Post("/devices/refresh", h.RefreshDeviceToken)

	// Protected endpoints (require device token)
	r.Group(func(pr chi.Router) {
		pr.Use(h.DeviceTokenAuthMiddleware)

		// Device management
		pr.Get("/devices", h.ListDevices)
		pr.Get("/devices/{device_id}", h.GetDevice)
		pr.Delete("/devices/{device_id}", h.RevokeDevice)
		pr.Patch("/devices/{device_id}", h.UpdateDevice)

		// Sync configuration
		pr.Get("/shares", h.ListSyncShares)
		pr.Get("/config", h.GetSyncConfig)
		pr.Put("/config", h.UpdateSyncConfig)

		// File operations
		pr.Get("/changes", h.GetChanges)
		pr.Get("/files/{share_id}/metadata", h.GetFileMetadata)
		pr.Post("/files/{share_id}/metadata", h.GetFilesMetadata)
		pr.Post("/files/{share_id}/hash", h.GetBlockHashes)

		// Sync state
		pr.Get("/state/{share_id}", h.GetSyncState)
		pr.Put("/state/{share_id}", h.UpdateSyncState)

		// Conflicts
		pr.Get("/conflicts", h.ListConflicts)
		pr.Get("/conflicts/{conflict_id}", h.GetConflict)
		pr.Put("/conflicts/{conflict_id}", h.ResolveConflict)

		// Activity history
		pr.Get("/activity", h.ListActivity)
		pr.Get("/activity/recent", h.GetRecentActivity)
		pr.Get("/activity/stats", h.GetActivityStats)

		// Shared folder collaboration
		pr.Get("/shared-folders", h.ListSharedFolders)
		pr.Post("/shared-folders", h.CreateSharedFolder)
		pr.Get("/shared-folders/{folder_id}", h.GetSharedFolder)
		pr.Delete("/shared-folders/{folder_id}", h.DeleteSharedFolder)
		pr.Post("/shared-folders/{folder_id}/members", h.AddFolderMember)
		pr.Delete("/shared-folders/{folder_id}/members/{user_id}", h.RemoveFolderMember)
		pr.Put("/shared-folders/{folder_id}/members/{user_id}", h.UpdateFolderMember)

		// Invitations
		pr.Get("/invites", h.ListPendingInvites)
		pr.Post("/invites", h.CreateInvite)
		pr.Put("/invites/{invite_id}/accept", h.AcceptInvite)
		pr.Put("/invites/{invite_id}/decline", h.DeclineInvite)
	})

	return r
}

// DeviceTokenAuthMiddleware validates device tokens
func (h *SyncHandler) DeviceTokenAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.missing", "Authorization header required", 0)
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.invalid", "Bearer token required", 0)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if !strings.HasPrefix(token, "nos_dt_") {
			httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.invalid", "Invalid token format", 0)
			return
		}

		// Get client info
		ip := r.RemoteAddr
		if i := strings.LastIndex(ip, ":"); i >= 0 {
			ip = ip[:i]
		}
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			ip = strings.Split(fwd, ",")[0]
		}
		userAgent := r.Header.Get("User-Agent")

		// Validate token
		device, err := h.deviceMgr.ValidateDeviceToken(token, ip, userAgent)
		if err != nil {
			h.logger.Warn().Err(err).Str("ip", ip).Msg("Device token validation failed")
			httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.invalid", "Invalid or expired token", 0)
			return
		}

		// Add device info to request context via headers
		r.Header.Set("X-Device-ID", device.ID)
		r.Header.Set("X-Device-User-ID", device.UserID)
		r.Header.Set("X-Device-Type", string(device.DeviceType))

		next.ServeHTTP(w, r)
	})
}

// RegisterDevice handles POST /sync/devices/register
func (h *SyncHandler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	// This endpoint requires user session auth, not device token
	userID := r.Header.Get("X-UID")
	if userID == "" {
		httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.required", "User authentication required", 0)
		return
	}

	var req nosync.DeviceRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body", 0)
		return
	}

	// Validate
	if req.DeviceName == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Device name is required", 0)
		return
	}
	if req.DeviceType == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Device type is required", 0)
		return
	}

	response, err := h.deviceMgr.RegisterDevice(userID, req)
	if err != nil {
		h.logger.Error().Err(err).Str("user_id", userID).Msg("Failed to register device")
		if strings.Contains(err.Error(), "maximum device limit") {
			httpx.WriteTypedError(w, http.StatusConflict, "device.limit", err.Error(), 0)
			return
		}
		httpx.WriteTypedError(w, http.StatusInternalServerError, "device.create_failed", err.Error(), 0)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, response)
}

// RefreshDeviceToken handles POST /sync/devices/refresh
func (h *SyncHandler) RefreshDeviceToken(w http.ResponseWriter, r *http.Request) {
	var req nosync.DeviceRefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body", 0)
		return
	}

	if req.RefreshToken == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Refresh token is required", 0)
		return
	}

	response, err := h.deviceMgr.RefreshDeviceToken(req.RefreshToken)
	if err != nil {
		h.logger.Warn().Err(err).Msg("Token refresh failed")
		httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.invalid", "Invalid refresh token", 0)
		return
	}

	writeJSON(w, response)
}

// ListDevices handles GET /sync/devices
func (h *SyncHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-Device-User-ID")
	devices := h.deviceMgr.ListDevices(userID)

	writeJSON(w, map[string]interface{}{
		"devices": devices,
		"count":   len(devices),
	})
}

// GetDevice handles GET /sync/devices/{device_id}
func (h *SyncHandler) GetDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")
	userID := r.Header.Get("X-Device-User-ID")

	device, err := h.deviceMgr.GetDevice(deviceID, userID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			httpx.WriteTypedError(w, http.StatusNotFound, "device.not_found", "Device not found", 0)
			return
		}
		httpx.WriteTypedError(w, http.StatusForbidden, "auth.forbidden", "Not authorized", 0)
		return
	}

	writeJSON(w, device)
}

// RevokeDevice handles DELETE /sync/devices/{device_id}
func (h *SyncHandler) RevokeDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")
	userID := r.Header.Get("X-Device-User-ID")

	if err := h.deviceMgr.RevokeDevice(deviceID, userID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			httpx.WriteTypedError(w, http.StatusNotFound, "device.not_found", "Device not found", 0)
			return
		}
		httpx.WriteTypedError(w, http.StatusForbidden, "auth.forbidden", "Not authorized", 0)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UpdateDevice handles PATCH /sync/devices/{device_id}
func (h *SyncHandler) UpdateDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")
	userID := r.Header.Get("X-Device-User-ID")

	var req struct {
		DeviceName string `json:"device_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body", 0)
		return
	}

	if req.DeviceName != "" {
		if err := h.deviceMgr.UpdateDeviceName(deviceID, userID, req.DeviceName); err != nil {
			httpx.WriteTypedError(w, http.StatusBadRequest, "device.update_failed", err.Error(), 0)
			return
		}
	}

	device, _ := h.deviceMgr.GetDevice(deviceID, userID)
	writeJSON(w, device)
}

// ListSyncShares handles GET /sync/shares
func (h *SyncHandler) ListSyncShares(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-Device-User-ID")

	// Get all shares accessible to this user
	allShares := h.shareStore.List()
	var syncShares []nosync.SyncShare

	for _, share := range allShares {
		// Check if user has access (simplified - in production check ACLs)
		accessible := false
		for _, u := range share.Users {
			if u == userID {
				accessible = true
				break
			}
		}
		if !accessible && len(share.Users) > 0 {
			continue
		}

		// Get share stats
		totalFiles, totalSize, _ := h.changeTracker.ShareStats(share.Path)

		syncShares = append(syncShares, nosync.SyncShare{
			ShareID:     share.ID,
			ShareName:   share.Name,
			SharePath:   share.Path,
			SyncEnabled: true, // All shares are sync-enabled for now
			TotalSize:   totalSize,
			FileCount:   totalFiles,
		})
	}

	writeJSON(w, map[string]interface{}{
		"shares": syncShares,
		"count":  len(syncShares),
	})
}

// GetSyncConfig handles GET /sync/config
func (h *SyncHandler) GetSyncConfig(w http.ResponseWriter, r *http.Request) {
	deviceID := r.Header.Get("X-Device-ID")

	config, ok := h.syncStore.GetConfig(deviceID)
	if !ok {
		// Return default config
		config = &nosync.SyncConfig{
			DeviceID:   deviceID,
			SyncShares: []string{},
			PauseSync:  false,
		}
	}

	writeJSON(w, config)
}

// UpdateSyncConfig handles PUT /sync/config
func (h *SyncHandler) UpdateSyncConfig(w http.ResponseWriter, r *http.Request) {
	deviceID := r.Header.Get("X-Device-ID")

	var config nosync.SyncConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body", 0)
		return
	}

	config.DeviceID = deviceID
	if err := h.syncStore.SaveConfig(&config); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "config.save_failed", err.Error(), 0)
		return
	}

	writeJSON(w, config)
}

// GetChanges handles GET /sync/changes
func (h *SyncHandler) GetChanges(w http.ResponseWriter, r *http.Request) {
	shareID := r.URL.Query().Get("share_id")
	cursor := r.URL.Query().Get("cursor")
	limitStr := r.URL.Query().Get("limit")

	if shareID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "share_id is required", 0)
		return
	}

	// Get share
	share, ok := h.shareStore.GetByID(shareID)
	if !ok {
		httpx.WriteTypedError(w, http.StatusNotFound, "share.not_found", "Share not found", 0)
		return
	}

	limit := 1000
	if limitStr != "" {
		if l, err := parseInt(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	response, err := h.changeTracker.GetChanges(share.Path, cursor, limit)
	if err != nil {
		h.logger.Error().Err(err).Str("share_id", shareID).Msg("Failed to get changes")
		httpx.WriteTypedError(w, http.StatusInternalServerError, "sync.changes_failed", err.Error(), 0)
		return
	}

	writeJSON(w, response)
}

// GetFileMetadata handles GET /sync/files/{share_id}/metadata
func (h *SyncHandler) GetFileMetadata(w http.ResponseWriter, r *http.Request) {
	shareID := chi.URLParam(r, "share_id")
	filePath := r.URL.Query().Get("path")

	if filePath == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "path is required", 0)
		return
	}

	share, ok := h.shareStore.GetByID(shareID)
	if !ok {
		httpx.WriteTypedError(w, http.StatusNotFound, "share.not_found", "Share not found", 0)
		return
	}

	meta, err := h.changeTracker.GetFileMetadata(share.Path, filePath)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			httpx.WriteTypedError(w, http.StatusNotFound, "file.not_found", "File not found", 0)
			return
		}
		httpx.WriteTypedError(w, http.StatusInternalServerError, "sync.metadata_failed", err.Error(), 0)
		return
	}

	writeJSON(w, meta)
}

// GetFilesMetadata handles POST /sync/files/{share_id}/metadata
func (h *SyncHandler) GetFilesMetadata(w http.ResponseWriter, r *http.Request) {
	shareID := chi.URLParam(r, "share_id")

	var req struct {
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body", 0)
		return
	}

	share, ok := h.shareStore.GetByID(shareID)
	if !ok {
		httpx.WriteTypedError(w, http.StatusNotFound, "share.not_found", "Share not found", 0)
		return
	}

	files, err := h.changeTracker.GetFilesMetadata(share.Path, req.Paths)
	if err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "sync.metadata_failed", err.Error(), 0)
		return
	}

	writeJSON(w, map[string]interface{}{
		"files": files,
	})
}

// GetBlockHashes handles POST /sync/files/{share_id}/hash
func (h *SyncHandler) GetBlockHashes(w http.ResponseWriter, r *http.Request) {
	shareID := chi.URLParam(r, "share_id")

	var req nosync.BlockHashRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body", 0)
		return
	}

	if req.Path == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "path is required", 0)
		return
	}

	share, ok := h.shareStore.GetByID(shareID)
	if !ok {
		httpx.WriteTypedError(w, http.StatusNotFound, "share.not_found", "Share not found", 0)
		return
	}

	fullPath := filepath.Join(share.Path, req.Path)
	response, err := h.deltaSync.ComputeBlockHashes(fullPath)
	if err != nil {
		h.logger.Error().Err(err).Str("path", req.Path).Msg("Failed to compute block hashes")
		httpx.WriteTypedError(w, http.StatusInternalServerError, "sync.hash_failed", err.Error(), 0)
		return
	}

	response.Path = req.Path // Return relative path
	writeJSON(w, response)
}

// GetSyncState handles GET /sync/state/{share_id}
func (h *SyncHandler) GetSyncState(w http.ResponseWriter, r *http.Request) {
	shareID := chi.URLParam(r, "share_id")
	deviceID := r.Header.Get("X-Device-ID")

	state, ok := h.syncStore.GetState(deviceID, shareID)
	if !ok {
		// Return empty state
		state = &nosync.SyncState{
			DeviceID: deviceID,
			ShareID:  shareID,
			LastSync: time.Time{},
			Files:    make(map[string]nosync.FileState),
		}
	}

	writeJSON(w, state)
}

// UpdateSyncState handles PUT /sync/state/{share_id}
func (h *SyncHandler) UpdateSyncState(w http.ResponseWriter, r *http.Request) {
	shareID := chi.URLParam(r, "share_id")
	deviceID := r.Header.Get("X-Device-ID")

	var state nosync.SyncState
	if err := json.NewDecoder(r.Body).Decode(&state); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body", 0)
		return
	}

	state.DeviceID = deviceID
	state.ShareID = shareID
	state.LastSync = time.Now()

	if err := h.syncStore.SaveState(&state); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "state.save_failed", err.Error(), 0)
		return
	}

	// Record sync for stats
	_ = h.deviceMgr.RecordSync(deviceID, state.TotalBytes)

	writeJSON(w, state)
}

// Stats returns sync handler statistics
func (h *SyncHandler) Stats() map[string]interface{} {
	return h.deviceMgr.Stats()
}

// DeviceManager returns the device manager for use by other handlers
func (h *SyncHandler) DeviceManager() *nosync.DeviceManager {
	return h.deviceMgr
}

// ==================== Conflict Handlers ====================

// ListConflicts returns all sync conflicts
func (h *SyncHandler) ListConflicts(w http.ResponseWriter, r *http.Request) {
	deviceID := r.Header.Get("X-Device-ID")
	if deviceID == "" {
		httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.required", "Authentication required", 0)
		return
	}

	shareID := r.URL.Query().Get("share_id")
	unresolvedOnly := r.URL.Query().Get("unresolved_only") == "true"

	conflicts := h.conflictStore.ListConflicts(shareID, deviceID, unresolvedOnly)
	writeJSON(w, conflicts)
}

// GetConflict returns a specific conflict
func (h *SyncHandler) GetConflict(w http.ResponseWriter, r *http.Request) {
	conflictID := chi.URLParam(r, "conflict_id")
	if conflictID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "Conflict ID required", 0)
		return
	}

	conflict, err := h.conflictStore.GetConflict(conflictID)
	if err != nil {
		httpx.WriteTypedError(w, http.StatusNotFound, "conflict.not_found", "Conflict not found", 0)
		return
	}

	writeJSON(w, conflict)
}

// ResolveConflict resolves a sync conflict
func (h *SyncHandler) ResolveConflict(w http.ResponseWriter, r *http.Request) {
	deviceID := r.Header.Get("X-Device-ID")
	if deviceID == "" {
		httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.required", "Authentication required", 0)
		return
	}

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

	resolution := nosync.ConflictResolution(req.Resolution)
	if resolution != nosync.ResolutionKeepLocal && 
	   resolution != nosync.ResolutionKeepRemote && 
	   resolution != nosync.ResolutionKeepBoth {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid resolution type", 0)
		return
	}

	conflict, err := h.conflictStore.ResolveConflict(conflictID, resolution, deviceID)
	if err != nil {
		httpx.WriteTypedError(w, http.StatusNotFound, "conflict.not_found", "Conflict not found", 0)
		return
	}

	// Record activity
	h.activityStore.RecordActivity(deviceID, conflict.ShareID, nosync.ActivityConflict, conflict.Path, 0)

	writeJSON(w, conflict)
}

// ==================== Activity Handlers ====================

// ListActivity returns paginated sync activity
func (h *SyncHandler) ListActivity(w http.ResponseWriter, r *http.Request) {
	deviceID := r.Header.Get("X-Device-ID")
	if deviceID == "" {
		httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.required", "Authentication required", 0)
		return
	}

	shareID := r.URL.Query().Get("share_id")
	page := parseIntWithDefault(r.URL.Query().Get("page"), 1)
	pageSize := parseIntWithDefault(r.URL.Query().Get("page_size"), 50)

	result := h.activityStore.ListActivities(deviceID, shareID, page, pageSize)
	writeJSON(w, result)
}

// GetRecentActivity returns recent sync activity
func (h *SyncHandler) GetRecentActivity(w http.ResponseWriter, r *http.Request) {
	deviceID := r.Header.Get("X-Device-ID")
	if deviceID == "" {
		httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.required", "Authentication required", 0)
		return
	}

	limit := parseIntWithDefault(r.URL.Query().Get("limit"), 20)
	activities := h.activityStore.GetRecentActivities(deviceID, limit)
	writeJSON(w, activities)
}

// GetActivityStats returns activity statistics
func (h *SyncHandler) GetActivityStats(w http.ResponseWriter, r *http.Request) {
	deviceID := r.Header.Get("X-Device-ID")
	if deviceID == "" {
		httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.required", "Authentication required", 0)
		return
	}

	shareID := r.URL.Query().Get("share_id")
	stats := h.activityStore.GetActivityStats(deviceID, shareID)
	writeJSON(w, stats)
}

// ==================== Collaboration Handlers ====================

// ListSharedFolders returns shared folders accessible to the user
func (h *SyncHandler) ListSharedFolders(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-Device-User-ID")
	if userID == "" {
		httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.required", "Authentication required", 0)
		return
	}

	folders := h.collaborationStore.ListSharedFolders(userID)
	writeJSON(w, map[string]interface{}{
		"folders": folders,
		"count":   len(folders),
	})
}

// CreateSharedFolder creates a new shared folder
func (h *SyncHandler) CreateSharedFolder(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-Device-User-ID")
	if userID == "" {
		httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.required", "Authentication required", 0)
		return
	}

	var req struct {
		ShareID   string `json:"share_id"`
		Path      string `json:"path"`
		Name      string `json:"name"`
		OwnerName string `json:"owner_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body", 0)
		return
	}

	if req.ShareID == "" || req.Path == "" || req.Name == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "share_id, path, and name are required", 0)
		return
	}

	folder, err := h.collaborationStore.CreateSharedFolder(req.ShareID, req.Path, req.Name, userID, req.OwnerName)
	if err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "folder.create_failed", err.Error(), 0)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, folder)
}

// GetSharedFolder returns a specific shared folder
func (h *SyncHandler) GetSharedFolder(w http.ResponseWriter, r *http.Request) {
	folderID := chi.URLParam(r, "folder_id")
	if folderID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "Folder ID required", 0)
		return
	}

	folder, err := h.collaborationStore.GetSharedFolder(folderID)
	if err != nil {
		httpx.WriteTypedError(w, http.StatusNotFound, "folder.not_found", "Folder not found", 0)
		return
	}

	writeJSON(w, folder)
}

// DeleteSharedFolder deletes a shared folder
func (h *SyncHandler) DeleteSharedFolder(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-Device-User-ID")
	if userID == "" {
		httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.required", "Authentication required", 0)
		return
	}

	folderID := chi.URLParam(r, "folder_id")
	if folderID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "Folder ID required", 0)
		return
	}

	folder, err := h.collaborationStore.GetSharedFolder(folderID)
	if err != nil {
		httpx.WriteTypedError(w, http.StatusNotFound, "folder.not_found", "Folder not found", 0)
		return
	}

	if folder.OwnerID != userID {
		httpx.WriteTypedError(w, http.StatusForbidden, "permission.denied", "Only owner can delete folder", 0)
		return
	}

	if err := h.collaborationStore.DeleteSharedFolder(folderID); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "folder.delete_failed", err.Error(), 0)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// AddFolderMember adds a member to a shared folder
func (h *SyncHandler) AddFolderMember(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-Device-User-ID")
	if userID == "" {
		httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.required", "Authentication required", 0)
		return
	}

	folderID := chi.URLParam(r, "folder_id")
	if folderID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "Folder ID required", 0)
		return
	}

	// Check permission
	if !h.collaborationStore.HasPermission(folderID, userID, nosync.PermissionAdmin) {
		httpx.WriteTypedError(w, http.StatusForbidden, "permission.denied", "Admin permission required", 0)
		return
	}

	var req struct {
		UserID     string `json:"user_id"`
		Username   string `json:"username"`
		Permission string `json:"permission"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body", 0)
		return
	}

	permission := nosync.SharePermission(req.Permission)
	if permission != nosync.PermissionRead && permission != nosync.PermissionWrite && permission != nosync.PermissionAdmin {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid permission level", 0)
		return
	}

	if err := h.collaborationStore.AddMember(folderID, req.UserID, req.Username, permission, userID); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "member.add_failed", err.Error(), 0)
		return
	}

	folder, _ := h.collaborationStore.GetSharedFolder(folderID)
	writeJSON(w, folder)
}

// RemoveFolderMember removes a member from a shared folder
func (h *SyncHandler) RemoveFolderMember(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-Device-User-ID")
	if userID == "" {
		httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.required", "Authentication required", 0)
		return
	}

	folderID := chi.URLParam(r, "folder_id")
	memberUserID := chi.URLParam(r, "user_id")
	if folderID == "" || memberUserID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "Folder ID and User ID required", 0)
		return
	}

	// Check permission (admins can remove, users can remove themselves)
	if memberUserID != userID && !h.collaborationStore.HasPermission(folderID, userID, nosync.PermissionAdmin) {
		httpx.WriteTypedError(w, http.StatusForbidden, "permission.denied", "Admin permission required", 0)
		return
	}

	if err := h.collaborationStore.RemoveMember(folderID, memberUserID); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "member.remove_failed", err.Error(), 0)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UpdateFolderMember updates a member's permission
func (h *SyncHandler) UpdateFolderMember(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-Device-User-ID")
	if userID == "" {
		httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.required", "Authentication required", 0)
		return
	}

	folderID := chi.URLParam(r, "folder_id")
	memberUserID := chi.URLParam(r, "user_id")
	if folderID == "" || memberUserID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "Folder ID and User ID required", 0)
		return
	}

	if !h.collaborationStore.HasPermission(folderID, userID, nosync.PermissionAdmin) {
		httpx.WriteTypedError(w, http.StatusForbidden, "permission.denied", "Admin permission required", 0)
		return
	}

	var req struct {
		Permission string `json:"permission"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body", 0)
		return
	}

	permission := nosync.SharePermission(req.Permission)
	if err := h.collaborationStore.UpdateMemberPermission(folderID, memberUserID, permission); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "member.update_failed", err.Error(), 0)
		return
	}

	folder, _ := h.collaborationStore.GetSharedFolder(folderID)
	writeJSON(w, folder)
}

// ==================== Invitation Handlers ====================

// ListPendingInvites returns pending invitations for the user
func (h *SyncHandler) ListPendingInvites(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-Device-User-ID")
	if userID == "" {
		httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.required", "Authentication required", 0)
		return
	}

	invites := h.collaborationStore.ListPendingInvites(userID)
	writeJSON(w, map[string]interface{}{
		"invites": invites,
		"count":   len(invites),
	})
}

// CreateInvite creates a new invitation
func (h *SyncHandler) CreateInvite(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-Device-User-ID")
	if userID == "" {
		httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.required", "Authentication required", 0)
		return
	}

	var req struct {
		FolderID     string `json:"folder_id"`
		InviterName  string `json:"inviter_name"`
		InviteeID    string `json:"invitee_id"`
		InviteeEmail string `json:"invitee_email"`
		Permission   string `json:"permission"`
		Message      string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body", 0)
		return
	}

	// Check permission
	if !h.collaborationStore.HasPermission(req.FolderID, userID, nosync.PermissionAdmin) {
		httpx.WriteTypedError(w, http.StatusForbidden, "permission.denied", "Admin permission required", 0)
		return
	}

	permission := nosync.SharePermission(req.Permission)
	if permission == "" {
		permission = nosync.PermissionRead
	}

	invite, err := h.collaborationStore.CreateInvite(
		req.FolderID,
		userID,
		req.InviterName,
		req.InviteeID,
		req.InviteeEmail,
		permission,
		req.Message,
		7*24*time.Hour, // 7 day expiry
	)
	if err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "invite.create_failed", err.Error(), 0)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, invite)
}

// AcceptInvite accepts an invitation
func (h *SyncHandler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-Device-User-ID")
	if userID == "" {
		httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.required", "Authentication required", 0)
		return
	}

	inviteID := chi.URLParam(r, "invite_id")
	if inviteID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "Invite ID required", 0)
		return
	}

	var req struct {
		Username string `json:"username"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := h.collaborationStore.AcceptInvite(inviteID, userID, req.Username); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "invite.accept_failed", err.Error(), 0)
		return
	}

	invite, _ := h.collaborationStore.GetInvite(inviteID)
	writeJSON(w, invite)
}

// DeclineInvite declines an invitation
func (h *SyncHandler) DeclineInvite(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-Device-User-ID")
	if userID == "" {
		httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.required", "Authentication required", 0)
		return
	}

	inviteID := chi.URLParam(r, "invite_id")
	if inviteID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "Invite ID required", 0)
		return
	}

	if err := h.collaborationStore.DeclineInvite(inviteID, userID); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "invite.decline_failed", err.Error(), 0)
		return
	}

	invite, _ := h.collaborationStore.GetInvite(inviteID)
	writeJSON(w, invite)
}

