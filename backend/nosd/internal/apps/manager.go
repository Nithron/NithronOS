package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"nos/backend/nosd/pkg/apps"
)

// Manager integrates all app management components
type Manager struct {
	catalogMgr    *apps.CatalogManager
	stateStore    *apps.StateStore
	lifecycleMgr  *apps.LifecycleManager
	healthMonitor *apps.HealthMonitor
	renderer      *apps.TemplateRenderer
	eventLogger   *EventLogger
	config        *Config
}

// Config holds app manager configuration
type Config struct {
	AppsRoot      string
	StateFile     string
	CatalogPath   string
	CachePath     string
	SourcesPath   string
	TemplatesPath string
	AgentPath     string
	CaddyPath     string
}

// EventLogger implements the apps.EventLogger interface
type EventLogger struct {
	mu     sync.Mutex
	events []apps.Event
	file   *os.File
}

// NewEventLogger creates a new event logger
func NewEventLogger(logPath string) (*EventLogger, error) {
	// If no path provided, operate in-memory only (tests)
	if strings.TrimSpace(logPath) == "" {
		return &EventLogger{events: []apps.Event{}}, nil
	}
	// Ensure directory exists
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &EventLogger{
		events: []apps.Event{},
		file:   file,
	}, nil
}

// LogEvent logs an app event
func (el *EventLogger) LogEvent(event apps.Event) error {
	el.mu.Lock()
	defer el.mu.Unlock()

	// Add to memory buffer
	el.events = append(el.events, event)

	// Keep only last 1000 events in memory
	if len(el.events) > 1000 {
		el.events = el.events[len(el.events)-1000:]
	}

	// Write to file if configured
	if el.file != nil {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal event: %w", err)
		}
		if _, err := el.file.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("failed to write event: %w", err)
		}
		return el.file.Sync()
	}
	return nil
}

// GetEvents returns recent events
func (el *EventLogger) GetEvents(appID string, limit int) []apps.Event {
	el.mu.Lock()
	defer el.mu.Unlock()

	result := []apps.Event{}
	for i := len(el.events) - 1; i >= 0 && len(result) < limit; i-- {
		if appID == "" || el.events[i].AppID == appID {
			result = append(result, el.events[i])
		}
	}

	return result
}

// Close closes the event logger
func (el *EventLogger) Close() error {
	if el.file != nil {
		return el.file.Close()
	}
	return nil
}

// NewManager creates a new app manager
func NewManager(config *Config) (*Manager, error) {
	// Create event logger
	logPath := filepath.Join(filepath.Dir(config.StateFile), "events.jsonl")
	if strings.EqualFold(os.Getenv("NOS_DISABLE_APP_EVENTS"), "1") || runtime.GOOS == "windows" {
		logPath = ""
	}
	eventLogger, err := NewEventLogger(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create event logger: %w", err)
	}

	// Create catalog manager
	catalogMgr := apps.NewCatalogManager(
		config.CatalogPath,
		config.CachePath,
		config.SourcesPath,
	)

	// Load catalog sources
	if err := catalogMgr.LoadSources(); err != nil {
		return nil, fmt.Errorf("failed to load catalog sources: %w", err)
	}

	// Create state store
	stateStore, err := apps.NewStateStore(config.StateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create state store: %w", err)
	}

	// Create template renderer
	renderer := apps.NewTemplateRenderer(config.TemplatesPath)

	// Create lifecycle manager
	lifecycleMgr := apps.NewLifecycleManager(
		catalogMgr,
		stateStore,
		renderer,
		config.AppsRoot,
		config.AgentPath,
		eventLogger,
	)

	// Create health monitor
	healthMonitor := apps.NewHealthMonitor(stateStore, catalogMgr)

	return &Manager{
		catalogMgr:    catalogMgr,
		stateStore:    stateStore,
		lifecycleMgr:  lifecycleMgr,
		healthMonitor: healthMonitor,
		renderer:      renderer,
		eventLogger:   eventLogger,
		config:        config,
	}, nil
}

// Start starts the app manager
func (m *Manager) Start(ctx context.Context) error {
	// Sync catalogs
	if err := m.catalogMgr.SyncRemoteCatalogs(); err != nil {
		// Log error but continue with builtin catalog
		fmt.Fprintf(os.Stderr, "Warning: failed to sync remote catalogs: %v\n", err)
	}

	// Start health monitoring
	if err := m.healthMonitor.Start(ctx); err != nil {
		return fmt.Errorf("failed to start health monitor: %w", err)
	}

	// Start periodic catalog sync
	go m.catalogSyncLoop(ctx)

	return nil
}

// Stop stops the app manager
func (m *Manager) Stop() error {
	m.healthMonitor.Stop()
	return m.eventLogger.Close()
}

// catalogSyncLoop periodically syncs remote catalogs
func (m *Manager) catalogSyncLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.catalogMgr.SyncRemoteCatalogs(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to sync catalogs: %v\n", err)
			}
		}
	}
}

// GetCatalog returns the merged app catalog
func (m *Manager) GetCatalog() (*apps.Catalog, error) {
	return m.catalogMgr.GetCatalog()
}

// GetInstalledApps returns all installed apps with health status
func (m *Manager) GetInstalledApps() []apps.InstalledApp {
	apps := m.stateStore.GetAllApps()

	// Update health status from cache
	healthCache := m.healthMonitor.GetAllHealth()
	for i := range apps {
		if health, ok := healthCache[apps[i].ID]; ok {
			apps[i].Health = health
		}
	}

	return apps
}

// GetApp returns a specific installed app
func (m *Manager) GetApp(appID string) (*apps.InstalledApp, error) {
	app, err := m.stateStore.GetApp(appID)
	if err != nil {
		return nil, err
	}

	// Update health status from cache
	if health, ok := m.healthMonitor.GetHealth(appID); ok {
		app.Health = health
	}

	return app, nil
}

// InstallApp installs a new app
func (m *Manager) InstallApp(ctx context.Context, req apps.InstallRequest, userID string) error {
	return m.lifecycleMgr.InstallApp(ctx, req, userID)
}

// UpgradeApp upgrades an existing app
func (m *Manager) UpgradeApp(ctx context.Context, appID string, req apps.UpgradeRequest, userID string) error {
	return m.lifecycleMgr.UpgradeApp(ctx, appID, req, userID)
}

// StartApp starts an app
func (m *Manager) StartApp(ctx context.Context, appID string, userID string) error {
	return m.lifecycleMgr.StartApp(ctx, appID, userID)
}

// StopApp stops an app
func (m *Manager) StopApp(ctx context.Context, appID string, userID string) error {
	return m.lifecycleMgr.StopApp(ctx, appID, userID)
}

// RestartApp restarts an app
func (m *Manager) RestartApp(ctx context.Context, appID string, userID string) error {
	return m.lifecycleMgr.RestartApp(ctx, appID, userID)
}

// DeleteApp deletes an app
func (m *Manager) DeleteApp(ctx context.Context, appID string, keepData bool, userID string) error {
	return m.lifecycleMgr.DeleteApp(ctx, appID, keepData, userID)
}

// RollbackApp rolls back an app to a snapshot
func (m *Manager) RollbackApp(ctx context.Context, appID string, snapshotTS string, userID string) error {
	return m.lifecycleMgr.RollbackApp(ctx, appID, snapshotTS, userID)
}

// GetAppLogs gets logs for an app
func (m *Manager) GetAppLogs(ctx context.Context, appID string, options apps.LogStreamOptions) ([]byte, error) {
	// TODO: Implement log streaming
	return nil, fmt.Errorf("log streaming not yet implemented")
}

// GetEvents returns recent events for an app
func (m *Manager) GetEvents(appID string, limit int) []apps.Event {
	return m.eventLogger.GetEvents(appID, limit)
}

// ForceHealthCheck forces a health check for an app
func (m *Manager) ForceHealthCheck(ctx context.Context, appID string) error {
	return m.healthMonitor.ForceCheck(ctx, appID)
}

// SyncCatalogs manually triggers catalog sync
func (m *Manager) SyncCatalogs() error {
	return m.catalogMgr.SyncRemoteCatalogs()
}
