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
	deviceMgr     *nosync.DeviceManager
	changeTracker *nosync.ChangeTracker
	deltaSync     *nosync.DeltaSync
	shareStore    *shares.Store
	syncStore     *nosync.Store
	logger        zerolog.Logger
	cfg           config.Config
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

	return &SyncHandler{
		deviceMgr:     deviceMgr,
		changeTracker: changeTracker,
		deltaSync:     deltaSync,
		shareStore:    shareStore,
		syncStore:     syncStore,
		logger:        logger.With().Str("component", "sync-handler").Logger(),
		cfg:           cfg,
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ListDevices handles GET /sync/devices
func (h *SyncHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-Device-User-ID")
	devices := h.deviceMgr.ListDevices(userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(device)
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
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(device)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(meta)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
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
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
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
	h.deviceMgr.RecordSync(deviceID, state.TotalBytes)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

// Stats returns sync handler statistics
func (h *SyncHandler) Stats() map[string]interface{} {
	return h.deviceMgr.Stats()
}

// DeviceManager returns the device manager for use by other handlers
func (h *SyncHandler) DeviceManager() *nosync.DeviceManager {
	return h.deviceMgr
}

// parseInt is a helper to parse integers
func parseInt(s string) (int, error) {
	var i int
	_, err := json.Unmarshal([]byte(s), &i)
	return i, err
}

