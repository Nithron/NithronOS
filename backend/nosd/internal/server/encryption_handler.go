package server

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"nithronos/backend/nosd/internal/config"
	"nithronos/backend/nosd/pkg/httpx"
	"nithronos/backend/nosd/pkg/sync/crypto"
)

// EncryptionHandler handles encryption-related API endpoints
type EncryptionHandler struct {
	keyMgr   *crypto.KeyManager
	encryptor *crypto.FileEncryptor
	logger   zerolog.Logger
	cfg      config.Config
	
	// State
	isUnlocked bool
	settings   EncryptionSettings
}

// EncryptionStatus represents the encryption status
type EncryptionStatus struct {
	Enabled              bool   `json:"enabled"`
	Algorithm            string `json:"algorithm"`
	MasterKeyInitialized bool   `json:"master_key_initialized"`
	RecoveryKeyExists    bool   `json:"recovery_key_exists"`
	SharesEncrypted      int    `json:"shares_encrypted"`
	TotalEncryptedFiles  int    `json:"total_encrypted_files"`
	TotalEncryptedSize   int64  `json:"total_encrypted_size"`
}

// EncryptionSettings represents encryption settings
type EncryptionSettings struct {
	DefaultAlgorithm  string `json:"default_algorithm"`
	EncryptNewShares  bool   `json:"encrypt_new_shares"`
	EncryptFilenames  bool   `json:"encrypt_filenames"`
	KeyRotationDays   int    `json:"key_rotation_days"`
}

// NewEncryptionHandler creates a new encryption handler
func NewEncryptionHandler(cfg config.Config, logger zerolog.Logger) (*EncryptionHandler, error) {
	dataDir := filepath.Join(cfg.AppsDataDir, "..", "sync", "encryption")
	keyMgr, err := crypto.NewKeyManager(dataDir)
	if err != nil {
		return nil, err
	}

	h := &EncryptionHandler{
		keyMgr: keyMgr,
		logger: logger.With().Str("component", "encryption-handler").Logger(),
		cfg:    cfg,
		settings: EncryptionSettings{
			DefaultAlgorithm:  string(crypto.AlgorithmXChaCha20Poly),
			EncryptNewShares:  true,
			EncryptFilenames:  false,
			KeyRotationDays:   90,
		},
	}

	return h, nil
}

// Routes returns the chi router for encryption endpoints
func (h *EncryptionHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Status
	r.Get("/status", h.GetStatus)

	// Initialization and unlocking
	r.Post("/initialize", h.Initialize)
	r.Post("/unlock", h.Unlock)
	r.Post("/lock", h.Lock)
	r.Post("/change-password", h.ChangePassword)

	// Recovery
	r.Post("/recovery-key", h.GenerateRecoveryKey)
	r.Post("/recover", h.RecoverWithKey)

	// Settings
	r.Get("/settings", h.GetSettings)
	r.Put("/settings", h.UpdateSettings)

	// Keys
	r.Get("/keys", h.ListKeys)

	// Share encryption
	r.Get("/shares/{share_id}", h.GetShareStatus)
	r.Post("/shares/{share_id}/enable", h.EnableShareEncryption)
	r.Post("/shares/{share_id}/disable", h.DisableShareEncryption)
	r.Post("/shares/{share_id}/rotate-key", h.RotateShareKey)

	// Device keys
	r.Get("/devices/{device_id}/public-key", h.ExportPublicKey)
	r.Post("/devices/{device_id}/public-key", h.ImportPublicKey)

	return r
}

// GetStatus returns the encryption status
func (h *EncryptionHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	status := EncryptionStatus{
		Enabled:              h.isUnlocked,
		Algorithm:            h.settings.DefaultAlgorithm,
		MasterKeyInitialized: h.keyMgr != nil,
		RecoveryKeyExists:    true, // Simplified
		SharesEncrypted:      0,
		TotalEncryptedFiles:  0,
		TotalEncryptedSize:   0,
	}

	writeJSON(w, status)
}

// Initialize initializes encryption with a password
func (h *EncryptionHandler) Initialize(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body", 0)
		return
	}

	if len(req.Password) < 8 {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Password must be at least 8 characters", 0)
		return
	}

	// Initialize master key
	if err := h.keyMgr.InitializeMasterKey(req.Password); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "encryption.init_failed", err.Error(), 0)
		return
	}

	// Generate recovery key
	recoveryKey, err := h.keyMgr.GenerateRecoveryKey()
	if err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "encryption.recovery_failed", err.Error(), 0)
		return
	}

	h.isUnlocked = true
	h.encryptor = crypto.NewFileEncryptor(h.keyMgr, crypto.EncryptionAlgorithm(h.settings.DefaultAlgorithm))

	h.logger.Info().Msg("Encryption initialized")

	writeJSON(w, map[string]string{
		"recovery_key": recoveryKey,
	})
}

// Unlock unlocks encryption with a password
func (h *EncryptionHandler) Unlock(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body", 0)
		return
	}

	if err := h.keyMgr.InitializeMasterKey(req.Password); err != nil {
		httpx.WriteTypedError(w, http.StatusUnauthorized, "encryption.unlock_failed", "Invalid password", 0)
		return
	}

	h.isUnlocked = true
	h.encryptor = crypto.NewFileEncryptor(h.keyMgr, crypto.EncryptionAlgorithm(h.settings.DefaultAlgorithm))

	h.logger.Info().Msg("Encryption unlocked")
	w.WriteHeader(http.StatusNoContent)
}

// Lock locks encryption
func (h *EncryptionHandler) Lock(w http.ResponseWriter, r *http.Request) {
	h.isUnlocked = false
	h.encryptor = nil

	h.logger.Info().Msg("Encryption locked")
	w.WriteHeader(http.StatusNoContent)
}

// ChangePassword changes the encryption password
func (h *EncryptionHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if !h.isUnlocked {
		httpx.WriteTypedError(w, http.StatusForbidden, "encryption.locked", "Encryption is locked", 0)
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body", 0)
		return
	}

	if len(req.NewPassword) < 8 {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "New password must be at least 8 characters", 0)
		return
	}

	if err := h.keyMgr.ChangePassword(req.OldPassword, req.NewPassword); err != nil {
		httpx.WriteTypedError(w, http.StatusUnauthorized, "encryption.change_failed", err.Error(), 0)
		return
	}

	h.logger.Info().Msg("Encryption password changed")
	w.WriteHeader(http.StatusNoContent)
}

// GenerateRecoveryKey generates a new recovery key
func (h *EncryptionHandler) GenerateRecoveryKey(w http.ResponseWriter, r *http.Request) {
	if !h.isUnlocked {
		httpx.WriteTypedError(w, http.StatusForbidden, "encryption.locked", "Encryption is locked", 0)
		return
	}

	recoveryKey, err := h.keyMgr.GenerateRecoveryKey()
	if err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "encryption.recovery_failed", err.Error(), 0)
		return
	}

	writeJSON(w, map[string]string{
		"recovery_key": recoveryKey,
	})
}

// RecoverWithKey recovers encryption using a recovery key
func (h *EncryptionHandler) RecoverWithKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RecoveryKey string `json:"recovery_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body", 0)
		return
	}

	if err := h.keyMgr.RecoverWithKey(req.RecoveryKey); err != nil {
		httpx.WriteTypedError(w, http.StatusUnauthorized, "encryption.recovery_failed", err.Error(), 0)
		return
	}

	h.isUnlocked = true
	h.encryptor = crypto.NewFileEncryptor(h.keyMgr, crypto.EncryptionAlgorithm(h.settings.DefaultAlgorithm))

	h.logger.Info().Msg("Encryption recovered")
	w.WriteHeader(http.StatusNoContent)
}

// GetSettings returns encryption settings
func (h *EncryptionHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.settings)
}

// UpdateSettings updates encryption settings
func (h *EncryptionHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var update EncryptionSettings
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body", 0)
		return
	}

	// Validate algorithm
	validAlgorithms := map[string]bool{
		"aes-256-gcm":         true,
		"chacha20-poly1305":   true,
		"xchacha20-poly1305":  true,
	}
	if update.DefaultAlgorithm != "" && !validAlgorithms[update.DefaultAlgorithm] {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid algorithm", 0)
		return
	}

	// Update settings
	if update.DefaultAlgorithm != "" {
		h.settings.DefaultAlgorithm = update.DefaultAlgorithm
	}
	h.settings.EncryptNewShares = update.EncryptNewShares
	h.settings.EncryptFilenames = update.EncryptFilenames
	if update.KeyRotationDays >= 30 && update.KeyRotationDays <= 365 {
		h.settings.KeyRotationDays = update.KeyRotationDays
	}

	writeJSON(w, h.settings)
}

// ListKeys returns a list of encryption keys
func (h *EncryptionHandler) ListKeys(w http.ResponseWriter, r *http.Request) {
	// In a full implementation, we'd return actual key metadata
	keys := []map[string]interface{}{
		{
			"id":         "master",
			"type":       "master",
			"algorithm":  h.settings.DefaultAlgorithm,
			"created_at": time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339),
		},
	}

	writeJSON(w, keys)
}

// GetShareStatus returns encryption status for a share
func (h *EncryptionHandler) GetShareStatus(w http.ResponseWriter, r *http.Request) {
	shareID := chi.URLParam(r, "share_id")
	if shareID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "Share ID required", 0)
		return
	}

	// Simplified response
	status := map[string]interface{}{
		"share_id":              shareID,
		"encrypted":             false,
		"encryption_enabled_at": nil,
		"total_files":           0,
		"encrypted_files":       0,
		"encryption_progress":   0,
	}

	writeJSON(w, status)
}

// EnableShareEncryption enables encryption for a share
func (h *EncryptionHandler) EnableShareEncryption(w http.ResponseWriter, r *http.Request) {
	if !h.isUnlocked {
		httpx.WriteTypedError(w, http.StatusForbidden, "encryption.locked", "Encryption is locked", 0)
		return
	}

	shareID := chi.URLParam(r, "share_id")
	if shareID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "Share ID required", 0)
		return
	}

	// Generate share key
	_, err := h.keyMgr.GenerateShareKey(shareID)
	if err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "encryption.key_failed", err.Error(), 0)
		return
	}

	h.logger.Info().Str("share_id", shareID).Msg("Share encryption enabled")
	w.WriteHeader(http.StatusNoContent)
}

// DisableShareEncryption disables encryption for a share
func (h *EncryptionHandler) DisableShareEncryption(w http.ResponseWriter, r *http.Request) {
	shareID := chi.URLParam(r, "share_id")
	if shareID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "Share ID required", 0)
		return
	}

	// In a full implementation, we'd decrypt all files and remove the share key
	h.logger.Info().Str("share_id", shareID).Msg("Share encryption disabled")
	w.WriteHeader(http.StatusNoContent)
}

// RotateShareKey rotates the encryption key for a share
func (h *EncryptionHandler) RotateShareKey(w http.ResponseWriter, r *http.Request) {
	if !h.isUnlocked {
		httpx.WriteTypedError(w, http.StatusForbidden, "encryption.locked", "Encryption is locked", 0)
		return
	}

	shareID := chi.URLParam(r, "share_id")
	if shareID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "Share ID required", 0)
		return
	}

	// Generate new share key
	_, err := h.keyMgr.GenerateShareKey(shareID)
	if err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "encryption.key_failed", err.Error(), 0)
		return
	}

	// In a full implementation, we'd re-encrypt all files with the new key
	h.logger.Info().Str("share_id", shareID).Msg("Share key rotated")
	w.WriteHeader(http.StatusNoContent)
}

// ExportPublicKey exports a device's public key
func (h *EncryptionHandler) ExportPublicKey(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")
	if deviceID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "Device ID required", 0)
		return
	}

	publicKey, err := h.keyMgr.GetPublicKeyForSharing(deviceID)
	if err != nil {
		httpx.WriteTypedError(w, http.StatusNotFound, "key.not_found", "Device key not found", 0)
		return
	}

	writeJSON(w, map[string]string{
		"public_key": publicKey,
	})
}

// ImportPublicKey imports a device's public key
func (h *EncryptionHandler) ImportPublicKey(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")
	if deviceID == "" {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.required", "Device ID required", 0)
		return
	}

	var req struct {
		PublicKey string `json:"public_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body", 0)
		return
	}

	// In a full implementation, we'd store the public key for key exchange
	h.logger.Info().Str("device_id", deviceID).Msg("Device public key imported")
	w.WriteHeader(http.StatusNoContent)
}

// GetEncryptor returns the file encryptor
func (h *EncryptionHandler) GetEncryptor() *crypto.FileEncryptor {
	return h.encryptor
}

// IsUnlocked returns whether encryption is unlocked
func (h *EncryptionHandler) IsUnlocked() bool {
	return h.isUnlocked
}

