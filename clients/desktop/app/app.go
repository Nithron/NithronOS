// Package app provides the main application logic for NithronSync desktop.
package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	gruntime "runtime"

	"github.com/rs/zerolog"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"nithronos/clients/sync-core/config"
	"nithronos/clients/sync-core/db"
	"nithronos/clients/sync-core/engine"
)

// App represents the main application.
type App struct {
	ctx       context.Context
	logger    zerolog.Logger
	cfg       *config.Config
	engine    *engine.Engine
	trayMgr   *TrayManager
}

// New creates a new App.
func New(logger zerolog.Logger) *App {
	return &App{
		logger: logger.With().Str("component", "app").Logger(),
	}
}

// Startup is called when the app starts.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		a.logger.Error().Err(err).Msg("Failed to load configuration")
		cfg = config.DefaultConfig()
	}
	a.cfg = cfg

	// Initialize sync engine if configured
	if cfg.IsConfigured() {
		a.initEngine()
	}

	// Initialize tray icon
	a.trayMgr = NewTrayManager(a)
	go a.trayMgr.Run()

	a.logger.Info().Msg("Application started")
}

// DomReady is called after the frontend is loaded.
func (a *App) DomReady(ctx context.Context) {
	a.logger.Debug().Msg("DOM ready")
}

// BeforeClose is called before the app closes.
func (a *App) BeforeClose(ctx context.Context) (prevent bool) {
	// Hide window instead of closing
	wailsruntime.WindowHide(ctx)
	return true
}

// Shutdown is called when the app is shutting down.
func (a *App) Shutdown(ctx context.Context) {
	if a.engine != nil {
		a.engine.Stop()
	}
	if a.trayMgr != nil {
		a.trayMgr.Stop()
	}
	a.logger.Info().Msg("Application shutdown")
}

// initEngine initializes the sync engine.
func (a *App) initEngine() error {
	eng, err := engine.New(a.cfg, a.logger)
	if err != nil {
		return err
	}
	a.engine = eng

	// Set up callbacks
	eng.SetCallbacks(
		func(state engine.State) {
			a.onStateChange(state)
		},
		func(progress engine.Progress) {
			a.onProgress(progress)
		},
		func(err error) {
			a.onError(err)
		},
		func(conflict engine.Conflict) {
			a.onConflict(conflict)
		},
	)

	// Start engine
	if a.cfg.SyncEnabled {
		go func() {
			if err := eng.Start(); err != nil {
				a.logger.Error().Err(err).Msg("Failed to start sync engine")
			}
		}()
	}

	return nil
}

// Frontend-callable methods

// GetConfig returns the current configuration.
func (a *App) GetConfig() *ConfigResponse {
	return &ConfigResponse{
		ServerURL:          a.cfg.ServerURL,
		SyncFolder:         a.cfg.SyncFolder,
		SyncEnabled:        a.cfg.SyncEnabled,
		SyncOnMetered:      a.cfg.SyncOnMetered,
		BandwidthLimitKBps: a.cfg.BandwidthLimitKBps,
		PollIntervalSecs:   a.cfg.PollIntervalSecs,
		IsConfigured:       a.cfg.IsConfigured(),
	}
}

// ConfigResponse is the response for GetConfig.
type ConfigResponse struct {
	ServerURL          string `json:"server_url"`
	SyncFolder         string `json:"sync_folder"`
	SyncEnabled        bool   `json:"sync_enabled"`
	SyncOnMetered      bool   `json:"sync_on_metered"`
	BandwidthLimitKBps int    `json:"bandwidth_limit_kbps"`
	PollIntervalSecs   int    `json:"poll_interval_secs"`
	IsConfigured       bool   `json:"is_configured"`
}

// SetupRequest contains setup information.
type SetupRequest struct {
	ServerURL   string `json:"server_url"`
	DeviceToken string `json:"device_token"`
	DeviceName  string `json:"device_name"`
	SyncFolder  string `json:"sync_folder"`
}

// Setup configures the sync client.
func (a *App) Setup(req SetupRequest) error {
	// Validate inputs
	if req.ServerURL == "" {
		return fmt.Errorf("server URL is required")
	}
	if req.DeviceToken == "" {
		return fmt.Errorf("device token is required")
	}

	// Set default sync folder if not provided
	if req.SyncFolder == "" {
		var err error
		req.SyncFolder, err = config.GetDefaultSyncFolder()
		if err != nil {
			return fmt.Errorf("failed to get default sync folder: %w", err)
		}
	}

	// Create sync folder
	if err := os.MkdirAll(req.SyncFolder, 0755); err != nil {
		return fmt.Errorf("failed to create sync folder: %w", err)
	}

	// Update configuration
	if err := a.cfg.Update(func(cfg *config.Config) {
		cfg.ServerURL = req.ServerURL
		cfg.DeviceToken = req.DeviceToken
		cfg.SyncFolder = req.SyncFolder
		cfg.SyncEnabled = true
	}); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// Initialize engine
	if err := a.initEngine(); err != nil {
		return fmt.Errorf("failed to initialize sync engine: %w", err)
	}

	return nil
}

// UpdateSettings updates sync settings.
func (a *App) UpdateSettings(settings SettingsUpdate) error {
	return a.cfg.Update(func(cfg *config.Config) {
		if settings.SyncEnabled != nil {
			cfg.SyncEnabled = *settings.SyncEnabled
		}
		if settings.SyncOnMetered != nil {
			cfg.SyncOnMetered = *settings.SyncOnMetered
		}
		if settings.BandwidthLimitKBps != nil {
			cfg.BandwidthLimitKBps = *settings.BandwidthLimitKBps
		}
		if settings.PollIntervalSecs != nil {
			cfg.PollIntervalSecs = *settings.PollIntervalSecs
		}
		if settings.ConflictPolicy != nil {
			cfg.ConflictPolicy = *settings.ConflictPolicy
		}
	})
}

// SettingsUpdate contains settings to update.
type SettingsUpdate struct {
	SyncEnabled        *bool   `json:"sync_enabled,omitempty"`
	SyncOnMetered      *bool   `json:"sync_on_metered,omitempty"`
	BandwidthLimitKBps *int    `json:"bandwidth_limit_kbps,omitempty"`
	PollIntervalSecs   *int    `json:"poll_interval_secs,omitempty"`
	ConflictPolicy     *string `json:"conflict_policy,omitempty"`
}

// GetStatus returns the current sync status.
func (a *App) GetStatus() *StatusResponse {
	status := &StatusResponse{
		State:       "stopped",
		IsConnected: false,
	}

	if a.engine == nil {
		return status
	}

	progress := a.engine.GetProgress()
	status.State = progress.State.String()
	status.IsConnected = progress.State != engine.StateStopped && progress.State != engine.StateError
	status.CurrentFile = progress.CurrentFile
	status.UploadedBytes = progress.UploadedBytes
	status.DownloadedBytes = progress.DownloadedBytes
	status.PendingUploads = progress.PendingUploads
	status.PendingDownloads = progress.PendingDownloads

	return status
}

// StatusResponse contains the sync status.
type StatusResponse struct {
	State            string `json:"state"`
	IsConnected      bool   `json:"is_connected"`
	CurrentFile      string `json:"current_file"`
	UploadedBytes    int64  `json:"uploaded_bytes"`
	DownloadedBytes  int64  `json:"downloaded_bytes"`
	PendingUploads   int64  `json:"pending_uploads"`
	PendingDownloads int64  `json:"pending_downloads"`
}

// GetStats returns sync statistics.
func (a *App) GetStats() map[string]*db.SyncStats {
	if a.engine == nil {
		return nil
	}
	return a.engine.GetStats()
}

// GetRecentActivity returns recent sync activity.
func (a *App) GetRecentActivity(limit int) ([]db.Activity, error) {
	if a.engine == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	return a.engine.GetRecentActivity(limit)
}

// PauseSync pauses synchronization.
func (a *App) PauseSync() {
	if a.engine != nil {
		a.engine.Pause()
	}
}

// ResumeSync resumes synchronization.
func (a *App) ResumeSync() {
	if a.engine != nil {
		a.engine.Resume()
	}
}

// SyncNow triggers an immediate sync.
func (a *App) SyncNow() {
	if a.engine != nil {
		a.engine.SyncNow()
	}
}

// OpenSyncFolder opens the sync folder in the file manager.
func (a *App) OpenSyncFolder() error {
	folder := a.cfg.SyncFolder
	if folder == "" {
		return fmt.Errorf("sync folder not configured")
	}

	var cmd string
	var args []string

	switch gruntime.GOOS {
	case "windows":
		cmd = "explorer"
		args = []string{folder}
	case "darwin":
		cmd = "open"
		args = []string{folder}
	default: // Linux
		cmd = "xdg-open"
		args = []string{folder}
	}

	return exec.Command(cmd, args...).Start()
}

// OpenWebUI opens the NithronOS web UI in the browser.
func (a *App) OpenWebUI() error {
	if a.cfg.ServerURL == "" {
		return fmt.Errorf("server not configured")
	}
	return wailsruntime.BrowserOpenURL(a.ctx, a.cfg.ServerURL)
}

// GetSystemInfo returns system information.
func (a *App) GetSystemInfo() *SystemInfo {
	return &SystemInfo{
		OS:        gruntime.GOOS,
		Arch:      gruntime.GOARCH,
		Version:   "1.0.0",
		GoVersion: gruntime.Version(),
		ConfigDir: getOrEmpty(config.GetConfigDir),
		DataDir:   getOrEmpty(config.GetDataDir),
		LogDir:    getOrEmpty(config.GetLogDir),
	}
}

// SystemInfo contains system information.
type SystemInfo struct {
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	Version   string `json:"version"`
	GoVersion string `json:"go_version"`
	ConfigDir string `json:"config_dir"`
	DataDir   string `json:"data_dir"`
	LogDir    string `json:"log_dir"`
}

// SelectFolder opens a folder selection dialog.
func (a *App) SelectFolder() (string, error) {
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Select Sync Folder",
	})
}

// Quit quits the application.
func (a *App) Quit() {
	if a.engine != nil {
		a.engine.Stop()
	}
	wailsruntime.Quit(a.ctx)
}

// ShowWindow shows the main window.
func (a *App) ShowWindow() {
	wailsruntime.WindowShow(a.ctx)
	wailsruntime.WindowUnminimise(a.ctx)
	wailsruntime.WindowSetAlwaysOnTop(a.ctx, true)
	wailsruntime.WindowSetAlwaysOnTop(a.ctx, false)
}

// HideWindow hides the main window.
func (a *App) HideWindow() {
	wailsruntime.WindowHide(a.ctx)
}

// Event handlers

func (a *App) onStateChange(state engine.State) {
	a.logger.Debug().Str("state", state.String()).Msg("State changed")
	wailsruntime.EventsEmit(a.ctx, "sync:state", state.String())
}

func (a *App) onProgress(progress engine.Progress) {
	wailsruntime.EventsEmit(a.ctx, "sync:progress", map[string]interface{}{
		"state":             progress.State.String(),
		"current_file":      progress.CurrentFile,
		"uploaded_bytes":    progress.UploadedBytes,
		"downloaded_bytes":  progress.DownloadedBytes,
		"pending_uploads":   progress.PendingUploads,
		"pending_downloads": progress.PendingDownloads,
	})
}

func (a *App) onError(err error) {
	a.logger.Error().Err(err).Msg("Sync error")
	wailsruntime.EventsEmit(a.ctx, "sync:error", err.Error())
}

func (a *App) onConflict(conflict engine.Conflict) {
	a.logger.Warn().
		Str("share", conflict.ShareID).
		Str("path", conflict.Path).
		Msg("Conflict detected")
	wailsruntime.EventsEmit(a.ctx, "sync:conflict", map[string]interface{}{
		"share_id": conflict.ShareID,
		"path":     conflict.Path,
	})
}

// Helper functions

func getOrEmpty(fn func() (string, error)) string {
	s, _ := fn()
	return s
}
