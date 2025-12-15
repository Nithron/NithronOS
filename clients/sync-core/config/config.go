// Package config provides configuration management for the sync client.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Config represents the sync client configuration.
type Config struct {
	// Server settings
	ServerURL   string `json:"server_url"`
	DeviceID    string `json:"device_id"`
	DeviceToken string `json:"device_token"`
	AccessToken string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`

	// Local settings
	SyncFolder      string   `json:"sync_folder"`
	SyncShares      []string `json:"sync_shares"`
	ExcludePatterns []string `json:"exclude_patterns"`

	// Sync behavior
	SyncEnabled       bool  `json:"sync_enabled"`
	SyncOnMetered     bool  `json:"sync_on_metered"`
	BandwidthLimitKBps int   `json:"bandwidth_limit_kbps"`
	PollIntervalSecs  int   `json:"poll_interval_secs"`

	// Advanced
	DebugLogging    bool   `json:"debug_logging"`
	MaxConcurrent   int    `json:"max_concurrent"`
	ConflictPolicy  string `json:"conflict_policy"` // "keep_both", "keep_local", "keep_remote"
	RetryAttempts   int    `json:"retry_attempts"`
	RetryDelaySecs  int    `json:"retry_delay_secs"`

	// Internal
	configPath string
	mu         sync.RWMutex
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		SyncEnabled:       true,
		SyncOnMetered:     false,
		BandwidthLimitKBps: 0, // Unlimited
		PollIntervalSecs:  30,
		DebugLogging:      false,
		MaxConcurrent:     4,
		ConflictPolicy:    "keep_both",
		RetryAttempts:     3,
		RetryDelaySecs:    5,
		ExcludePatterns: []string{
			"*.tmp",
			"*.temp",
			"~$*",
			".DS_Store",
			"Thumbs.db",
			"desktop.ini",
			".git/**",
			".svn/**",
			"node_modules/**",
			"__pycache__/**",
			"*.pyc",
			".sync_*",
		},
	}
}

// GetConfigDir returns the platform-specific configuration directory.
func GetConfigDir() (string, error) {
	var baseDir string

	switch runtime.GOOS {
	case "windows":
		baseDir = os.Getenv("APPDATA")
		if baseDir == "" {
			baseDir = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		baseDir = filepath.Join(home, "Library", "Application Support")
	default: // Linux and others
		baseDir = os.Getenv("XDG_CONFIG_HOME")
		if baseDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			baseDir = filepath.Join(home, ".config")
		}
	}

	configDir := filepath.Join(baseDir, "NithronSync")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	return configDir, nil
}

// GetDataDir returns the platform-specific data directory.
func GetDataDir() (string, error) {
	var baseDir string

	switch runtime.GOOS {
	case "windows":
		baseDir = os.Getenv("LOCALAPPDATA")
		if baseDir == "" {
			baseDir = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		baseDir = filepath.Join(home, "Library", "Application Support")
	default: // Linux and others
		baseDir = os.Getenv("XDG_DATA_HOME")
		if baseDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			baseDir = filepath.Join(home, ".local", "share")
		}
	}

	dataDir := filepath.Join(baseDir, "NithronSync")
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create data directory: %w", err)
	}

	return dataDir, nil
}

// GetLogDir returns the platform-specific log directory.
func GetLogDir() (string, error) {
	var logDir string

	switch runtime.GOOS {
	case "windows":
		baseDir := os.Getenv("LOCALAPPDATA")
		if baseDir == "" {
			baseDir = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		logDir = filepath.Join(baseDir, "NithronSync", "logs")
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		logDir = filepath.Join(home, "Library", "Logs", "NithronSync")
	default: // Linux and others
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		logDir = filepath.Join(home, ".local", "share", "nithron-sync", "logs")
	}

	if err := os.MkdirAll(logDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create log directory: %w", err)
	}

	return logDir, nil
}

// GetDefaultSyncFolder returns the default sync folder path.
func GetDefaultSyncFolder() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "NithronSync"), nil
}

// Load loads the configuration from the default location.
func Load() (*Config, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(configDir, "config.json")
	return LoadFrom(configPath)
}

// LoadFrom loads the configuration from a specific file.
func LoadFrom(path string) (*Config, error) {
	cfg := DefaultConfig()
	cfg.configPath = path

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// No config file yet, use defaults
		syncFolder, _ := GetDefaultSyncFolder()
		cfg.SyncFolder = syncFolder
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

// Save saves the configuration to disk.
func (c *Config) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.configPath == "" {
		configDir, err := GetConfigDir()
		if err != nil {
			return err
		}
		c.configPath = filepath.Join(configDir, "config.json")
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(c.configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Update updates config values and saves.
func (c *Config) Update(fn func(*Config)) error {
	c.mu.Lock()
	fn(c)
	c.mu.Unlock()
	return c.Save()
}

// IsConfigured returns true if the client has been configured with server details.
func (c *Config) IsConfigured() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ServerURL != "" && c.DeviceToken != ""
}

// GetServerURL returns the server URL.
func (c *Config) GetServerURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ServerURL
}

// GetDeviceToken returns the device token.
func (c *Config) GetDeviceToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.DeviceToken
}

// GetAccessToken returns the current access token.
func (c *Config) GetAccessToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.AccessToken
}

// SetTokens updates the access and refresh tokens.
func (c *Config) SetTokens(accessToken, refreshToken string) error {
	return c.Update(func(cfg *Config) {
		cfg.AccessToken = accessToken
		cfg.RefreshToken = refreshToken
	})
}

