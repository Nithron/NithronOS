package sync

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
)

// DeviceManager manages sync device registration and authentication
type DeviceManager struct {
	store    *Store
	logger   zerolog.Logger
	mu       sync.RWMutex
	
	// In-memory token cache for fast validation
	tokenCache     map[string]*DeviceToken // tokenHash -> device
	tokenCacheMu   sync.RWMutex
	
	// Configuration
	maxDevicesPerUser int
	deviceTokenTTL    time.Duration
	refreshTokenTTL   time.Duration
}

// DeviceManagerConfig holds configuration for the device manager
type DeviceManagerConfig struct {
	MaxDevicesPerUser int
	DeviceTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration
}

// DefaultDeviceManagerConfig returns the default configuration
func DefaultDeviceManagerConfig() DeviceManagerConfig {
	return DeviceManagerConfig{
		MaxDevicesPerUser: 20,
		DeviceTokenTTL:    DeviceTokenTTL,
		RefreshTokenTTL:   RefreshTokenTTL,
	}
}

// NewDeviceManager creates a new device manager
func NewDeviceManager(store *Store, logger zerolog.Logger, config DeviceManagerConfig) *DeviceManager {
	dm := &DeviceManager{
		store:             store,
		logger:            logger.With().Str("component", "device-manager").Logger(),
		tokenCache:        make(map[string]*DeviceToken),
		maxDevicesPerUser: config.MaxDevicesPerUser,
		deviceTokenTTL:    config.DeviceTokenTTL,
		refreshTokenTTL:   config.RefreshTokenTTL,
	}
	
	// Populate token cache from stored devices
	dm.populateTokenCache()
	
	// Start background cleanup routine
	go dm.cleanupRoutine()
	
	return dm
}

// populateTokenCache loads all device tokens into the cache
func (dm *DeviceManager) populateTokenCache() {
	dm.tokenCacheMu.Lock()
	defer dm.tokenCacheMu.Unlock()
	
	// This is a simplified approach - in production, we'd iterate all devices
	// For now, the cache is populated on first use
}

// RegisterDevice registers a new sync device for a user
func (dm *DeviceManager) RegisterDevice(userID string, req DeviceRegisterRequest) (*DeviceRegisterResponse, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	// Validate device type
	if !isValidDeviceType(req.DeviceType) {
		return nil, fmt.Errorf("invalid device type: %s", req.DeviceType)
	}
	
	// Check device limit per user
	deviceCount := dm.store.DeviceCountByUser(userID)
	if deviceCount >= dm.maxDevicesPerUser {
		return nil, fmt.Errorf("maximum device limit (%d) reached", dm.maxDevicesPerUser)
	}
	
	// Validate device name
	if len(req.DeviceName) == 0 || len(req.DeviceName) > 64 {
		return nil, fmt.Errorf("device name must be 1-64 characters")
	}
	
	// Generate device ID
	deviceID := uuid.New().String()
	
	// Generate tokens
	deviceToken := dm.generateDeviceToken()
	refreshToken := dm.generateRefreshToken()
	
	// Hash tokens for storage
	deviceTokenHash, err := dm.hashToken(deviceToken)
	if err != nil {
		return nil, fmt.Errorf("failed to hash device token: %w", err)
	}
	refreshTokenHash, err := dm.hashToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to hash refresh token: %w", err)
	}
	
	// Calculate expiry times
	now := time.Now()
	deviceExpiresAt := now.Add(dm.deviceTokenTTL)
	
	// Create device record
	device := &DeviceToken{
		ID:            deviceID,
		UserID:        userID,
		DeviceName:    req.DeviceName,
		DeviceType:    req.DeviceType,
		OSVersion:     req.OSVersion,
		ClientVersion: req.ClientVersion,
		TokenHash:     deviceTokenHash,
		RefreshHash:   refreshTokenHash,
		CreatedAt:     now,
		ExpiresAt:     &deviceExpiresAt,
		Scopes:        DefaultDeviceScopes(),
		SyncCount:     0,
		BytesSynced:   0,
	}
	
	// Persist device
	if err := dm.store.SaveDevice(device); err != nil {
		return nil, fmt.Errorf("failed to save device: %w", err)
	}
	
	// Add to token cache
	dm.tokenCacheMu.Lock()
	dm.tokenCache[deviceTokenHash] = device
	dm.tokenCacheMu.Unlock()
	
	dm.logger.Info().
		Str("device_id", deviceID).
		Str("user_id", userID).
		Str("device_name", req.DeviceName).
		Str("device_type", string(req.DeviceType)).
		Msg("Registered new sync device")
	
	return &DeviceRegisterResponse{
		DeviceID:     deviceID,
		DeviceToken:  deviceToken,
		RefreshToken: refreshToken,
		ExpiresAt:    deviceExpiresAt,
	}, nil
}

// ValidateDeviceToken validates a device token and returns the associated device
func (dm *DeviceManager) ValidateDeviceToken(tokenValue, ip, userAgent string) (*DeviceToken, error) {
	// Check token format
	if !strings.HasPrefix(tokenValue, "nos_dt_") {
		return nil, fmt.Errorf("invalid token format")
	}
	
	// Check cache first
	tokenHash := dm.fastHash(tokenValue)
	dm.tokenCacheMu.RLock()
	cachedDevice, ok := dm.tokenCache[tokenHash]
	dm.tokenCacheMu.RUnlock()
	
	if ok {
		// Verify the token matches (in case of hash collision)
		if err := bcrypt.CompareHashAndPassword([]byte(cachedDevice.TokenHash), []byte(tokenValue)); err == nil {
			// Check expiration
			if cachedDevice.ExpiresAt != nil && cachedDevice.ExpiresAt.Before(time.Now()) {
				return nil, fmt.Errorf("token expired")
			}
			// Check revocation
			if cachedDevice.RevokedAt != nil {
				return nil, fmt.Errorf("device revoked")
			}
			
			// Update activity asynchronously
			go func() {
				_ = dm.store.UpdateDeviceActivity(cachedDevice.ID, ip, userAgent)
			}()
			
			return cachedDevice, nil
		}
	}
	
	// Fallback to full scan (expensive but necessary for cache misses)
	devices := dm.store.ListDevicesByUser("")
	for _, device := range devices {
		if device.TokenHash == "" {
			continue
		}
		if err := bcrypt.CompareHashAndPassword([]byte(device.TokenHash), []byte(tokenValue)); err == nil {
			// Check expiration
			if device.ExpiresAt != nil && device.ExpiresAt.Before(time.Now()) {
				return nil, fmt.Errorf("token expired")
			}
			// Check revocation
			if device.RevokedAt != nil {
				return nil, fmt.Errorf("device revoked")
			}
			
			// Update cache
			dm.tokenCacheMu.Lock()
			dm.tokenCache[dm.fastHash(tokenValue)] = device
			dm.tokenCacheMu.Unlock()
			
			// Update activity asynchronously
			go func() {
				_ = dm.store.UpdateDeviceActivity(device.ID, ip, userAgent)
			}()
			
			return device, nil
		}
	}
	
	return nil, fmt.Errorf("invalid token")
}

// RefreshDeviceToken refreshes a device's tokens using the refresh token
func (dm *DeviceManager) RefreshDeviceToken(refreshTokenValue string) (*DeviceRefreshResponse, error) {
	// Check token format
	if !strings.HasPrefix(refreshTokenValue, "nos_rt_") {
		return nil, fmt.Errorf("invalid refresh token format")
	}
	
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	// Find device by refresh token (scan all devices)
	var targetDevice *DeviceToken
	devices := dm.store.ListDevicesByUser("")
	for _, device := range devices {
		if device.RefreshHash == "" {
			continue
		}
		if err := bcrypt.CompareHashAndPassword([]byte(device.RefreshHash), []byte(refreshTokenValue)); err == nil {
			targetDevice = device
			break
		}
	}
	
	if targetDevice == nil {
		return nil, fmt.Errorf("invalid refresh token")
	}
	
	// Check if device is revoked
	if targetDevice.RevokedAt != nil {
		return nil, fmt.Errorf("device revoked")
	}
	
	// Generate new tokens
	newDeviceToken := dm.generateDeviceToken()
	newRefreshToken := dm.generateRefreshToken()
	
	// Hash new tokens
	newDeviceTokenHash, err := dm.hashToken(newDeviceToken)
	if err != nil {
		return nil, fmt.Errorf("failed to hash device token: %w", err)
	}
	newRefreshTokenHash, err := dm.hashToken(newRefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to hash refresh token: %w", err)
	}
	
	// Update device with new tokens
	now := time.Now()
	newExpiresAt := now.Add(dm.deviceTokenTTL)
	
	// Remove old token from cache
	dm.tokenCacheMu.Lock()
	for k, v := range dm.tokenCache {
		if v.ID == targetDevice.ID {
			delete(dm.tokenCache, k)
			break
		}
	}
	dm.tokenCacheMu.Unlock()
	
	// Update device
	targetDevice.TokenHash = newDeviceTokenHash
	targetDevice.RefreshHash = newRefreshTokenHash
	targetDevice.ExpiresAt = &newExpiresAt
	targetDevice.LastSeenAt = &now
	
	// Persist
	if err := dm.store.SaveDevice(targetDevice); err != nil {
		return nil, fmt.Errorf("failed to update device: %w", err)
	}
	
	// Add new token to cache
	dm.tokenCacheMu.Lock()
	dm.tokenCache[dm.fastHash(newDeviceToken)] = targetDevice
	dm.tokenCacheMu.Unlock()
	
	dm.logger.Info().
		Str("device_id", targetDevice.ID).
		Str("user_id", targetDevice.UserID).
		Msg("Refreshed device tokens")
	
	return &DeviceRefreshResponse{
		DeviceToken:  newDeviceToken,
		RefreshToken: newRefreshToken,
		ExpiresAt:    newExpiresAt,
	}, nil
}

// RevokeDevice revokes a device's access
func (dm *DeviceManager) RevokeDevice(deviceID, userID string) error {
	device, ok := dm.store.GetDevice(deviceID)
	if !ok {
		return fmt.Errorf("device not found")
	}
	
	// Verify ownership
	if device.UserID != userID {
		return fmt.Errorf("unauthorized")
	}
	
	// Remove from cache
	dm.tokenCacheMu.Lock()
	for k, v := range dm.tokenCache {
		if v.ID == deviceID {
			delete(dm.tokenCache, k)
			break
		}
	}
	dm.tokenCacheMu.Unlock()
	
	// Delete device (soft delete)
	if err := dm.store.DeleteDevice(deviceID); err != nil {
		return fmt.Errorf("failed to revoke device: %w", err)
	}
	
	dm.logger.Info().
		Str("device_id", deviceID).
		Str("user_id", userID).
		Msg("Revoked sync device")
	
	return nil
}

// RevokeAllDevices revokes all devices for a user
func (dm *DeviceManager) RevokeAllDevices(userID string) (int, error) {
	devices := dm.store.ListDevicesByUser(userID)
	
	// Remove from cache
	dm.tokenCacheMu.Lock()
	for k, v := range dm.tokenCache {
		if v.UserID == userID {
			delete(dm.tokenCache, k)
		}
	}
	dm.tokenCacheMu.Unlock()
	
	// Revoke each device
	count := 0
	for _, device := range devices {
		if err := dm.store.DeleteDevice(device.ID); err != nil {
			dm.logger.Error().Err(err).Str("device_id", device.ID).Msg("Failed to revoke device")
			continue
		}
		count++
	}
	
	dm.logger.Info().
		Str("user_id", userID).
		Int("count", count).
		Msg("Revoked all sync devices for user")
	
	return count, nil
}

// ListDevices returns all devices for a user (public view)
func (dm *DeviceManager) ListDevices(userID string) []DeviceTokenPublic {
	devices := dm.store.ListDevicesByUser(userID)
	result := make([]DeviceTokenPublic, len(devices))
	for i, d := range devices {
		result[i] = d.ToPublic()
	}
	return result
}

// GetDevice returns a specific device (public view)
func (dm *DeviceManager) GetDevice(deviceID, userID string) (*DeviceTokenPublic, error) {
	device, ok := dm.store.GetDevice(deviceID)
	if !ok {
		return nil, fmt.Errorf("device not found")
	}
	if device.UserID != userID {
		return nil, fmt.Errorf("unauthorized")
	}
	pub := device.ToPublic()
	return &pub, nil
}

// UpdateDeviceName updates a device's name
func (dm *DeviceManager) UpdateDeviceName(deviceID, userID, newName string) error {
	device, ok := dm.store.GetDevice(deviceID)
	if !ok {
		return fmt.Errorf("device not found")
	}
	if device.UserID != userID {
		return fmt.Errorf("unauthorized")
	}
	if len(newName) == 0 || len(newName) > 64 {
		return fmt.Errorf("device name must be 1-64 characters")
	}
	
	device.DeviceName = newName
	return dm.store.SaveDevice(device)
}

// HasScope checks if a device has a specific scope
func (dm *DeviceManager) HasScope(device *DeviceToken, scope string) bool {
	for _, s := range device.Scopes {
		if s == scope || s == string(ScopeSyncAdmin) {
			return true
		}
	}
	return false
}

// RecordSync records a sync operation for a device
func (dm *DeviceManager) RecordSync(deviceID string, bytesTransferred int64) error {
	return dm.store.UpdateDeviceStats(deviceID, bytesTransferred)
}

// Token generation helpers

func (dm *DeviceManager) generateDeviceToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return "nos_dt_" + base64.URLEncoding.EncodeToString(b)
}

func (dm *DeviceManager) generateRefreshToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return "nos_rt_" + base64.URLEncoding.EncodeToString(b)
}

func (dm *DeviceManager) hashToken(token string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// fastHash creates a quick hash for cache lookups (not for storage)
func (dm *DeviceManager) fastHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func isValidDeviceType(dt DeviceType) bool {
	switch dt {
	case DeviceTypeWindows, DeviceTypeLinux, DeviceTypeMacOS, DeviceTypeAndroid, DeviceTypeIOS:
		return true
	}
	return false
}

// cleanupRoutine periodically cleans up expired devices
func (dm *DeviceManager) cleanupRoutine() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	
	for range ticker.C {
		count, err := dm.store.CleanupExpiredDevices()
		if err != nil {
			dm.logger.Error().Err(err).Msg("Failed to cleanup expired devices")
		} else if count > 0 {
			dm.logger.Info().Int("count", count).Msg("Cleaned up expired devices")
		}
	}
}

// Stats returns statistics about registered devices
func (dm *DeviceManager) Stats() map[string]interface{} {
	dm.tokenCacheMu.RLock()
	cacheSize := len(dm.tokenCache)
	dm.tokenCacheMu.RUnlock()
	
	return map[string]interface{}{
		"total_devices":        dm.store.DeviceCount(),
		"token_cache_size":     cacheSize,
		"max_devices_per_user": dm.maxDevicesPerUser,
		"device_token_ttl_sec": int(dm.deviceTokenTTL.Seconds()),
	}
}

