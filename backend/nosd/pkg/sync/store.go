package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"nithronos/backend/nosd/internal/fsatomic"
)

// Store handles persistence of sync-related data
type Store struct {
	basePath    string
	devicesPath string
	statePath   string
	configPath  string

	mu      sync.RWMutex
	devices map[string]*DeviceToken
	configs map[string]*SyncConfig
	states  map[string]*SyncState // key: deviceID:shareID
}

// devicesFile is the on-disk structure for devices
type devicesFile struct {
	Version int                     `json:"version"`
	Devices map[string]*DeviceToken `json:"devices"`
}

// configsFile is the on-disk structure for sync configs
type configsFile struct {
	Version int                    `json:"version"`
	Configs map[string]*SyncConfig `json:"configs"`
}

// NewStore creates a new sync store
func NewStore(basePath string) (*Store, error) {
	s := &Store{
		basePath:    basePath,
		devicesPath: filepath.Join(basePath, "devices.json"),
		statePath:   filepath.Join(basePath, "state"),
		configPath:  filepath.Join(basePath, "configs.json"),
		devices:     make(map[string]*DeviceToken),
		configs:     make(map[string]*SyncConfig),
		states:      make(map[string]*SyncState),
	}

	// Ensure directories exist
	if err := os.MkdirAll(basePath, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create sync directory: %w", err)
	}
	if err := os.MkdirAll(s.statePath, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	// Load existing data
	if err := s.loadDevices(); err != nil {
		return nil, fmt.Errorf("failed to load devices: %w", err)
	}
	if err := s.loadConfigs(); err != nil {
		return nil, fmt.Errorf("failed to load configs: %w", err)
	}

	return s, nil
}

// loadDevices loads devices from disk
func (s *Store) loadDevices() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var df devicesFile
	ok, err := fsatomic.LoadJSON(s.devicesPath, &df)
	if err != nil {
		return err
	}
	if !ok {
		s.devices = make(map[string]*DeviceToken)
		return nil
	}
	if df.Devices == nil {
		df.Devices = make(map[string]*DeviceToken)
	}
	s.devices = df.Devices
	return nil
}

// loadConfigs loads sync configs from disk
func (s *Store) loadConfigs() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var cf configsFile
	ok, err := fsatomic.LoadJSON(s.configPath, &cf)
	if err != nil {
		return err
	}
	if !ok {
		s.configs = make(map[string]*SyncConfig)
		return nil
	}
	if cf.Configs == nil {
		cf.Configs = make(map[string]*SyncConfig)
	}
	s.configs = cf.Configs
	return nil
}

// SaveDevice persists a device token
func (s *Store) SaveDevice(device *DeviceToken) error {
	s.mu.Lock()
	s.devices[device.ID] = device
	devicesCopy := make(map[string]*DeviceToken, len(s.devices))
	for k, v := range s.devices {
		devicesCopy[k] = v
	}
	s.mu.Unlock()

	return fsatomic.WithLock(s.devicesPath, func() error {
		return fsatomic.SaveJSON(context.TODO(), s.devicesPath, devicesFile{
			Version: 1,
			Devices: devicesCopy,
		}, 0o600)
	})
}

// GetDevice retrieves a device by ID
func (s *Store) GetDevice(deviceID string) (*DeviceToken, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	device, ok := s.devices[deviceID]
	if !ok {
		return nil, false
	}
	// Return a copy
	copy := *device
	return &copy, true
}

// GetDeviceByTokenHash finds a device by its token hash
func (s *Store) GetDeviceByTokenHash(hash string) (*DeviceToken, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, device := range s.devices {
		if device.TokenHash == hash {
			copy := *device
			return &copy, true
		}
	}
	return nil, false
}

// ListDevicesByUser returns all devices for a user
func (s *Store) ListDevicesByUser(userID string) []*DeviceToken {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var devices []*DeviceToken
	for _, device := range s.devices {
		if device.UserID == userID && device.RevokedAt == nil {
			copy := *device
			devices = append(devices, &copy)
		}
	}
	return devices
}

// DeleteDevice removes a device (soft delete by setting RevokedAt)
func (s *Store) DeleteDevice(deviceID string) error {
	s.mu.Lock()
	device, ok := s.devices[deviceID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("device not found")
	}
	now := time.Now()
	device.RevokedAt = &now
	s.devices[deviceID] = device
	devicesCopy := make(map[string]*DeviceToken, len(s.devices))
	for k, v := range s.devices {
		devicesCopy[k] = v
	}
	s.mu.Unlock()

	return fsatomic.WithLock(s.devicesPath, func() error {
		return fsatomic.SaveJSON(context.TODO(), s.devicesPath, devicesFile{
			Version: 1,
			Devices: devicesCopy,
		}, 0o600)
	})
}

// PermanentlyDeleteDevice removes a device completely
func (s *Store) PermanentlyDeleteDevice(deviceID string) error {
	s.mu.Lock()
	delete(s.devices, deviceID)
	devicesCopy := make(map[string]*DeviceToken, len(s.devices))
	for k, v := range s.devices {
		devicesCopy[k] = v
	}
	s.mu.Unlock()

	// Also delete associated state files
	statePattern := filepath.Join(s.statePath, deviceID+"_*.json")
	matches, _ := filepath.Glob(statePattern)
	for _, m := range matches {
		_ = os.Remove(m)
	}

	return fsatomic.WithLock(s.devicesPath, func() error {
		return fsatomic.SaveJSON(context.TODO(), s.devicesPath, devicesFile{
			Version: 1,
			Devices: devicesCopy,
		}, 0o600)
	})
}

// SaveConfig persists a sync config
func (s *Store) SaveConfig(config *SyncConfig) error {
	s.mu.Lock()
	s.configs[config.DeviceID] = config
	configsCopy := make(map[string]*SyncConfig, len(s.configs))
	for k, v := range s.configs {
		configsCopy[k] = v
	}
	s.mu.Unlock()

	return fsatomic.WithLock(s.configPath, func() error {
		return fsatomic.SaveJSON(context.TODO(), s.configPath, configsFile{
			Version: 1,
			Configs: configsCopy,
		}, 0o600)
	})
}

// GetConfig retrieves a sync config by device ID
func (s *Store) GetConfig(deviceID string) (*SyncConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	config, ok := s.configs[deviceID]
	if !ok {
		return nil, false
	}
	copy := *config
	return &copy, true
}

// stateKey generates the key for a sync state
func stateKey(deviceID, shareID string) string {
	return deviceID + ":" + shareID
}

// stateFileName generates the filename for a sync state
func stateFileName(deviceID, shareID string) string {
	return deviceID + "_" + shareID + ".json"
}

// SaveState persists a sync state
func (s *Store) SaveState(state *SyncState) error {
	key := stateKey(state.DeviceID, state.ShareID)
	s.mu.Lock()
	s.states[key] = state
	s.mu.Unlock()

	statePath := filepath.Join(s.statePath, stateFileName(state.DeviceID, state.ShareID))
	return fsatomic.WithLock(statePath, func() error {
		return fsatomic.SaveJSON(context.TODO(), statePath, state, 0o600)
	})
}

// GetState retrieves a sync state
func (s *Store) GetState(deviceID, shareID string) (*SyncState, bool) {
	key := stateKey(deviceID, shareID)

	// Check cache first
	s.mu.RLock()
	state, ok := s.states[key]
	s.mu.RUnlock()
	if ok {
		copy := *state
		return &copy, true
	}

	// Load from disk
	statePath := filepath.Join(s.statePath, stateFileName(deviceID, shareID))
	var loaded SyncState
	okLoad, err := fsatomic.LoadJSON(statePath, &loaded)
	if err != nil || !okLoad {
		return nil, false
	}

	// Cache it
	s.mu.Lock()
	s.states[key] = &loaded
	s.mu.Unlock()

	return &loaded, true
}

// DeleteState removes a sync state
func (s *Store) DeleteState(deviceID, shareID string) error {
	key := stateKey(deviceID, shareID)
	s.mu.Lock()
	delete(s.states, key)
	s.mu.Unlock()

	statePath := filepath.Join(s.statePath, stateFileName(deviceID, shareID))
	return os.Remove(statePath)
}

// UpdateDeviceStats updates sync statistics for a device
func (s *Store) UpdateDeviceStats(deviceID string, bytesTransferred int64) error {
	s.mu.Lock()
	device, ok := s.devices[deviceID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("device not found")
	}
	now := time.Now()
	device.SyncCount++
	device.BytesSynced += bytesTransferred
	device.LastSyncAt = &now
	device.LastSeenAt = &now
	s.mu.Unlock()

	return s.SaveDevice(device)
}

// UpdateDeviceActivity updates the last seen time for a device
func (s *Store) UpdateDeviceActivity(deviceID, ip, userAgent string) error {
	s.mu.Lock()
	device, ok := s.devices[deviceID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("device not found")
	}
	now := time.Now()
	device.LastSeenAt = &now
	device.LastIP = ip
	device.LastUserAgent = userAgent
	s.mu.Unlock()

	return s.SaveDevice(device)
}

// CleanupExpiredDevices removes devices that have been revoked for more than 30 days
func (s *Store) CleanupExpiredDevices() (int, error) {
	s.mu.Lock()
	thirtyDaysAgo := time.Now().Add(-30 * 24 * time.Hour)
	var toDelete []string
	for id, device := range s.devices {
		if device.RevokedAt != nil && device.RevokedAt.Before(thirtyDaysAgo) {
			toDelete = append(toDelete, id)
		}
		if device.ExpiresAt != nil && device.ExpiresAt.Before(time.Now()) {
			toDelete = append(toDelete, id)
		}
	}
	for _, id := range toDelete {
		delete(s.devices, id)
	}
	devicesCopy := make(map[string]*DeviceToken, len(s.devices))
	for k, v := range s.devices {
		devicesCopy[k] = v
	}
	s.mu.Unlock()

	if len(toDelete) == 0 {
		return 0, nil
	}

	// Delete associated state files
	for _, id := range toDelete {
		statePattern := filepath.Join(s.statePath, id+"_*.json")
		matches, _ := filepath.Glob(statePattern)
		for _, m := range matches {
			_ = os.Remove(m)
		}
	}

	err := fsatomic.WithLock(s.devicesPath, func() error {
		return fsatomic.SaveJSON(context.TODO(), s.devicesPath, devicesFile{
			Version: 1,
			Devices: devicesCopy,
		}, 0o600)
	})

	return len(toDelete), err
}

// ExportDevices exports all devices as JSON (for backup)
func (s *Store) ExportDevices() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.MarshalIndent(s.devices, "", "  ")
}

// DeviceCount returns the total number of active devices
func (s *Store) DeviceCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, d := range s.devices {
		if d.RevokedAt == nil {
			count++
		}
	}
	return count
}

// DeviceCountByUser returns the number of active devices for a user
func (s *Store) DeviceCountByUser(userID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, d := range s.devices {
		if d.UserID == userID && d.RevokedAt == nil {
			count++
		}
	}
	return count
}

