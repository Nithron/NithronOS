package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// LifecycleManager handles app lifecycle operations. nosd runs as the nos
// user: it writes the app's compose and env files into the config folder
// (which belongs to nos) and asks nos-agent for every privileged step
// (folders and ownership, the systemd unit, snapshots, the Caddy proxy).
// Those steps used to run here directly and failed on installed systems.
type LifecycleManager struct {
	catalogMgr  *CatalogManager
	stateStore  *StateStore
	renderer    *TemplateRenderer
	appsRoot    string
	host        HostOps
	eventLogger EventLogger
}

// AppOp is one privileged step, run by nos-agent's /v1/apps/op.
type AppOp struct {
	Op       string `json:"op"`
	AppID    string `json:"app_id"`
	UID      int    `json:"uid,omitempty"`
	GID      int    `json:"gid,omitempty"`
	Name     string `json:"name,omitempty"`
	Snapshot string `json:"snapshot,omitempty"`
	KeepData bool   `json:"keep_data,omitempty"`
	Port     int    `json:"port,omitempty"`
}

// HostOps runs AppOps (nos-agent; a fake in tests).
type HostOps interface {
	AppOp(ctx context.Context, op AppOp) (output string, err error)
}

// EventLogger interface for logging events
type EventLogger interface {
	LogEvent(event Event) error
}

// NewLifecycleManager creates a new lifecycle manager
func NewLifecycleManager(
	catalogMgr *CatalogManager,
	stateStore *StateStore,
	renderer *TemplateRenderer,
	appsRoot string,
	host HostOps,
	eventLogger EventLogger,
) *LifecycleManager {
	return &LifecycleManager{
		catalogMgr:  catalogMgr,
		stateStore:  stateStore,
		renderer:    renderer,
		appsRoot:    appsRoot,
		host:        host,
		eventLogger: eventLogger,
	}
}

// InstallApp installs a new application
func (lm *LifecycleManager) InstallApp(ctx context.Context, req InstallRequest, userID string) error {
	// Get catalog entry
	entry, err := lm.catalogMgr.GetEntry(req.ID)
	if err != nil {
		return fmt.Errorf("app not found in catalog: %w", err)
	}

	// Check if already installed
	if _, err := lm.stateStore.GetApp(req.ID); err == nil {
		return fmt.Errorf("app already installed: %s", req.ID)
	}

	// Validate parameters
	if err := lm.renderer.ValidateParams(entry, req.Params); err != nil {
		return fmt.Errorf("parameter validation failed: %w", err)
	}

	// Log installation start event
	lm.logEvent("app.install.start", req.ID, userID, map[string]interface{}{
		"version": entry.Version,
		"params":  req.Params,
	})

	// The app's folders: config (nos writes it) and data (a Btrfs subvolume
	// when possible, owned by the user the containers run as).
	appDir := filepath.Join(lm.appsRoot, req.ID)
	configDir := filepath.Join(appDir, "config")
	uid, gid := containerIDs(req.Params)
	if _, err := lm.host.AppOp(ctx, AppOp{Op: "prepare", AppID: req.ID, UID: uid, GID: gid}); err != nil {
		return fmt.Errorf("failed to prepare the app folders: %w", err)
	}
	cleanup := func() { _, _ = lm.host.AppOp(context.Background(), AppOp{Op: "remove", AppID: req.ID}) }

	params, err := lm.withSystemParams(ctx, req.ID, req.Params)
	if err != nil {
		cleanup()
		return err
	}

	// Render compose file
	composeContent, err := lm.renderer.RenderComposeFile(entry, params)
	if err != nil {
		cleanup()
		return fmt.Errorf("failed to render compose file: %w", err)
	}

	composePath := filepath.Join(configDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, composeContent, 0600); err != nil {
		cleanup()
		return fmt.Errorf("failed to write compose file: %w", err)
	}

	// Render environment file
	envContent, err := lm.renderer.RenderEnvFile(params)
	if err != nil {
		cleanup()
		return fmt.Errorf("failed to render env file: %w", err)
	}

	envPath := filepath.Join(configDir, ".env")
	if err := os.WriteFile(envPath, envContent, 0600); err != nil {
		cleanup()
		return fmt.Errorf("failed to write env file: %w", err)
	}

	// Create initial snapshot
	snapshotID, err := lm.createSnapshot(ctx, req.ID, "post-install")
	if err != nil {
		// Log warning but continue
		fmt.Fprintf(os.Stderr, "Warning: failed to create post-install snapshot: %v\n", err)
	}

	// Start the app
	if err := lm.startApp(ctx, req.ID); err != nil {
		// Rollback on failure
		cleanup()
		return fmt.Errorf("failed to start app: %w", err)
	}

	// Setup reverse proxy if ports are exposed
	if len(entry.Defaults.Ports) > 0 {
		if err := lm.setupReverseProxy(ctx, req.ID, entry.Defaults.Ports); err != nil {
			// Log warning but continue
			fmt.Fprintf(os.Stderr, "Warning: failed to setup reverse proxy: %v\n", err)
		}
	}

	// Save app state
	app := InstalledApp{
		ID:      req.ID,
		Name:    entry.Name,
		Version: entry.Version,
		Status:  StatusRunning,
		Params:  req.Params,
		Ports:   entry.Defaults.Ports,
		URLs:    lm.generateAppURLs(req.ID, entry.Defaults.Ports),
		Health: HealthStatus{
			Status:    "unknown",
			CheckedAt: time.Now(),
		},
		Snapshots: []AppSnapshot{},
	}

	if snapshotID != "" {
		app.Snapshots = append(app.Snapshots, AppSnapshot{
			ID:        snapshotID,
			Timestamp: time.Now(),
			Type:      "btrfs",
			Name:      "post-install",
			Path:      filepath.Join(lm.appsRoot, ".snapshots", req.ID, snapshotID),
		})
	}

	if err := lm.stateStore.AddApp(app); err != nil {
		// Try to clean up
		if stopErr := lm.stopApp(ctx, req.ID); stopErr != nil {
			fmt.Printf("Failed to stop app during cleanup: %v\n", stopErr)
		}
		cleanup()
		return fmt.Errorf("failed to save app state: %w", err)
	}

	// Log installation complete event
	lm.logEvent("app.install.complete", req.ID, userID, map[string]interface{}{
		"version": entry.Version,
	})
	return nil
}

// UpgradeApp upgrades an existing application
func (lm *LifecycleManager) UpgradeApp(ctx context.Context, appID string, req UpgradeRequest, userID string) error {
	// Get current app
	app, err := lm.stateStore.GetApp(appID)
	if err != nil {
		return fmt.Errorf("app not found: %w", err)
	}

	// Get catalog entry
	entry, err := lm.catalogMgr.GetEntry(appID)
	if err != nil {
		return fmt.Errorf("app not found in catalog: %w", err)
	}

	// Log upgrade start event
	lm.logEvent("app.upgrade.start", appID, userID, map[string]interface{}{
		"from_version": app.Version,
		"to_version":   req.Version,
	})

	// Create pre-upgrade snapshot
	snapshotID, err := lm.createSnapshot(ctx, appID, "pre-upgrade")
	if err != nil {
		return fmt.Errorf("failed to create pre-upgrade snapshot: %w", err)
	}

	// Update app state to upgrading
	if err := lm.stateStore.UpdateAppStatus(appID, StatusUpgrading); err != nil {
		return fmt.Errorf("failed to update app status: %w", err)
	}

	// Merge params if provided
	params := app.Params
	if req.Params != nil {
		for k, v := range req.Params {
			params[k] = v
		}
	}

	// Validate new parameters
	if err := lm.renderer.ValidateParams(entry, params); err != nil {
		if err := lm.stateStore.UpdateAppStatus(appID, StatusError); err != nil {
			fmt.Printf("Failed to update app status: %v\n", err)
		}
		return fmt.Errorf("parameter validation failed: %w", err)
	}

	// Render new compose file
	configDir := filepath.Join(lm.appsRoot, appID, "config")
	rendered, err := lm.withSystemParams(ctx, appID, params)
	if err != nil {
		if err := lm.stateStore.UpdateAppStatus(appID, StatusError); err != nil {
			fmt.Printf("Failed to update app status: %v\n", err)
		}
		return err
	}
	composeContent, err := lm.renderer.RenderComposeFile(entry, rendered)
	if err != nil {
		if err := lm.stateStore.UpdateAppStatus(appID, StatusError); err != nil {
			fmt.Printf("Failed to update app status: %v\n", err)
		}
		return fmt.Errorf("failed to render compose file: %w", err)
	}

	// Backup current config
	composePath := filepath.Join(configDir, "docker-compose.yml")
	backupPath := filepath.Join(configDir, "docker-compose.yml.backup")
	if err := lm.copyFile(composePath, backupPath); err != nil {
		if err := lm.stateStore.UpdateAppStatus(appID, StatusError); err != nil {
			fmt.Printf("Failed to update app status: %v\n", err)
		}
		return fmt.Errorf("failed to backup compose file: %w", err)
	}

	// Write new compose file
	if err := os.WriteFile(composePath, composeContent, 0600); err != nil {
		if err := lm.stateStore.UpdateAppStatus(appID, StatusError); err != nil {
			fmt.Printf("Failed to update app status: %v\n", err)
		}
		return fmt.Errorf("failed to write compose file: %w", err)
	}

	// Pull new images
	if err := lm.pullImages(ctx, appID); err != nil {
		// Restore backup
		if err := lm.copyFile(backupPath, composePath); err != nil {
			fmt.Printf("Failed to restore compose file: %v\n", err)
		}
		if err := lm.stateStore.UpdateAppStatus(appID, StatusError); err != nil {
			fmt.Printf("Failed to update app status: %v\n", err)
		}
		return fmt.Errorf("failed to pull images: %w", err)
	}

	// Restart app with new configuration
	if err := lm.restartApp(ctx, appID); err != nil {
		// Rollback on failure
		if err := lm.copyFile(backupPath, composePath); err != nil {
			fmt.Printf("Failed to restore compose file: %v\n", err)
		}
		if err := lm.rollbackSnapshot(ctx, appID, snapshotID); err != nil {
			fmt.Printf("Failed to rollback snapshot: %v\n", err)
		}
		if err := lm.stateStore.UpdateAppStatus(appID, StatusError); err != nil {
			fmt.Printf("Failed to update app status: %v\n", err)
		}
		return fmt.Errorf("failed to restart app: %w", err)
	}

	// Wait for health check
	healthy := lm.waitForHealth(ctx, appID, 60*time.Second)
	if !healthy {
		// Rollback if unhealthy
		fmt.Fprintf(os.Stderr, "App unhealthy after upgrade, rolling back...\n")
		if err := lm.copyFile(backupPath, composePath); err != nil {
			fmt.Printf("Failed to restore compose file: %v\n", err)
		}
		if err := lm.rollbackSnapshot(ctx, appID, snapshotID); err != nil {
			fmt.Printf("Failed to rollback snapshot: %v\n", err)
		}
		if err := lm.restartApp(ctx, appID); err != nil {
			fmt.Printf("Failed to restart app after update: %v\n", err)
		}
		if err := lm.stateStore.UpdateAppStatus(appID, StatusError); err != nil {
			fmt.Printf("Failed to update app status: %v\n", err)
		}
		return fmt.Errorf("app unhealthy after upgrade, rolled back")
	}

	// Update app state
	app.Version = req.Version
	app.Params = params
	app.Status = StatusRunning
	if err := lm.stateStore.UpdateApp(*app); err != nil {
		return fmt.Errorf("failed to update app state: %w", err)
	}

	// Log upgrade complete event
	lm.logEvent("app.upgrade.complete", appID, userID, map[string]interface{}{
		"version": req.Version,
	})

	// Clean up backup
	os.Remove(backupPath)

	return nil
}

// StartApp starts an application
func (lm *LifecycleManager) StartApp(ctx context.Context, appID string, userID string) error {
	if err := lm.stateStore.UpdateAppStatus(appID, StatusStarting); err != nil {
		return err
	}

	lm.logEvent("app.start", appID, userID, nil)

	if err := lm.startApp(ctx, appID); err != nil {
		if err := lm.stateStore.UpdateAppStatus(appID, StatusError); err != nil {
			fmt.Printf("Failed to update app status: %v\n", err)
		}
		return err
	}

	return lm.stateStore.UpdateAppStatus(appID, StatusRunning)
}

// StopApp stops an application
func (lm *LifecycleManager) StopApp(ctx context.Context, appID string, userID string) error {
	if err := lm.stateStore.UpdateAppStatus(appID, StatusStopping); err != nil {
		return err
	}

	lm.logEvent("app.stop", appID, userID, nil)

	if err := lm.stopApp(ctx, appID); err != nil {
		if err := lm.stateStore.UpdateAppStatus(appID, StatusError); err != nil {
			fmt.Printf("Failed to update app status: %v\n", err)
		}
		return err
	}

	return lm.stateStore.UpdateAppStatus(appID, StatusStopped)
}

// RestartApp restarts an application
func (lm *LifecycleManager) RestartApp(ctx context.Context, appID string, userID string) error {
	lm.logEvent("app.restart", appID, userID, nil)
	return lm.restartApp(ctx, appID)
}

// DeleteApp deletes an application
func (lm *LifecycleManager) DeleteApp(ctx context.Context, appID string, keepData bool, userID string) error {
	// Stop the app first
	if err := lm.stopApp(ctx, appID); err != nil {
		fmt.Printf("Failed to stop app during uninstall: %v\n", err)
	}

	lm.logEvent("app.delete", appID, userID, map[string]interface{}{
		"keep_data": keepData,
	})

	// Remove from systemd
	if err := lm.disableSystemdService(ctx, appID); err != nil {
		fmt.Printf("Failed to disable systemd service: %v\n", err)
	}

	// Remove the Caddy proxy
	if _, err := lm.host.AppOp(ctx, AppOp{Op: "unproxy", AppID: appID}); err != nil {
		fmt.Printf("Failed to remove the app's proxy: %v\n", err)
	}

	// Remove the config folder, and the data and snapshots unless kept
	if _, err := lm.host.AppOp(ctx, AppOp{Op: "remove", AppID: appID, KeepData: keepData}); err != nil {
		return fmt.Errorf("failed to remove app files: %w", err)
	}

	// Remove from state
	return lm.stateStore.DeleteApp(appID)
}

// RollbackApp rolls back an app to a snapshot
func (lm *LifecycleManager) RollbackApp(ctx context.Context, appID string, snapshotTS string, userID string) error {
	lm.logEvent("app.rollback", appID, userID, map[string]interface{}{
		"snapshot": snapshotTS,
	})

	if err := lm.stateStore.UpdateAppStatus(appID, StatusRollback); err != nil {
		return err
	}

	if err := lm.rollbackSnapshot(ctx, appID, snapshotTS); err != nil {
		if err := lm.stateStore.UpdateAppStatus(appID, StatusError); err != nil {
			fmt.Printf("Failed to update app status: %v\n", err)
		}
		return err
	}

	// Restart app after rollback
	if err := lm.restartApp(ctx, appID); err != nil {
		if err := lm.stateStore.UpdateAppStatus(appID, StatusError); err != nil {
			fmt.Printf("Failed to update app status: %v\n", err)
		}
		return err
	}

	return lm.stateStore.UpdateAppStatus(appID, StatusRunning)
}

// Helper methods

func (lm *LifecycleManager) startApp(ctx context.Context, appID string) error {
	_, err := lm.host.AppOp(ctx, AppOp{Op: "start", AppID: appID})
	return err
}

func (lm *LifecycleManager) stopApp(ctx context.Context, appID string) error {
	_, err := lm.host.AppOp(ctx, AppOp{Op: "stop", AppID: appID})
	return err
}

func (lm *LifecycleManager) restartApp(ctx context.Context, appID string) error {
	_, err := lm.host.AppOp(ctx, AppOp{Op: "restart", AppID: appID})
	return err
}

func (lm *LifecycleManager) disableSystemdService(ctx context.Context, appID string) error {
	_, err := lm.host.AppOp(ctx, AppOp{Op: "disable", AppID: appID})
	return err
}

func (lm *LifecycleManager) pullImages(ctx context.Context, appID string) error {
	_, err := lm.host.AppOp(ctx, AppOp{Op: "pull", AppID: appID})
	return err
}

// createSnapshot snapshots the app's data and returns the snapshot's name
// (the last element of the path the snapshot script prints).
// SnapshotApp takes a snapshot of an installed app's data on request and
// records it, so it can be rolled back to.
func (lm *LifecycleManager) SnapshotApp(ctx context.Context, appID, userID string) (AppSnapshot, error) {
	if _, err := lm.stateStore.GetApp(appID); err != nil {
		return AppSnapshot{}, fmt.Errorf("app not found: %s", appID)
	}
	id, err := lm.createSnapshot(ctx, appID, "manual")
	if err != nil {
		return AppSnapshot{}, fmt.Errorf("failed to snapshot the app: %w", err)
	}
	snap := AppSnapshot{ID: id, Timestamp: time.Now(), Type: "btrfs", Name: "manual",
		Path: filepath.Join(lm.appsRoot, ".snapshots", appID, id)}
	if err := lm.stateStore.AddSnapshot(appID, snap); err != nil {
		return AppSnapshot{}, fmt.Errorf("failed to record the snapshot: %w", err)
	}
	lm.logEvent("app.snapshot", appID, userID, map[string]interface{}{"snapshot": id})
	return snap, nil
}

func (lm *LifecycleManager) createSnapshot(ctx context.Context, appID, name string) (string, error) {
	out, err := lm.host.AppOp(ctx, AppOp{Op: "snapshot", AppID: appID, Name: name})
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "/.snapshots/") {
			return filepath.Base(strings.TrimSpace(line)), nil
		}
	}
	return "", fmt.Errorf("snapshot ID not found in output")
}

func (lm *LifecycleManager) rollbackSnapshot(ctx context.Context, appID, snapshotTS string) error {
	_, err := lm.host.AppOp(ctx, AppOp{Op: "rollback", AppID: appID, Snapshot: snapshotTS})
	return err
}

// containerIDs is the user and group the app's containers run as (PUID and
// PGID, 1000 by default); its data folder belongs to them.
func containerIDs(params map[string]interface{}) (int, int) {
	id := func(key string) int {
		if v, err := strconv.Atoi(fmt.Sprint(params[key])); err == nil && v > 0 {
			return v
		}
		return 1000
	}
	return id("PUID"), id("PGID")
}

// helperScript reads app status with docker (nos is in the docker group).
const helperScript = "/usr/lib/nos/apps/nos-app-helper.sh"

// proxyPort is the host port Caddy forwards /apps/<id>/ to: the one
// published for a usual HTTP container port, or else the first published.
func proxyPort(ports []PortMapping) int {
	for _, p := range ports {
		if p.Host > 0 && (p.Container == 80 || p.Container == 8080 || p.Container == 3000 || p.Container == 8000) {
			return p.Host
		}
	}
	for _, p := range ports {
		if p.Host > 0 {
			return p.Host
		}
	}
	return 0
}

func (lm *LifecycleManager) setupReverseProxy(ctx context.Context, appID string, ports []PortMapping) error {
	port := proxyPort(ports)
	if port == 0 {
		return nil // nothing published
	}
	_, err := lm.host.AppOp(ctx, AppOp{Op: "proxy", AppID: appID, Port: port})
	return err
}

func (lm *LifecycleManager) generateAppURLs(appID string, ports []PortMapping) []string {
	urls := []string{}

	// Add path-based URL
	urls = append(urls, fmt.Sprintf("/apps/%s/", appID))

	// Add port-based URLs if any
	for _, port := range ports {
		if port.Protocol == "tcp" {
			urls = append(urls, fmt.Sprintf("http://localhost:%d", port.Host))
		}
	}

	return urls
}

func (lm *LifecycleManager) waitForHealth(ctx context.Context, appID string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// Check container status
		cmd := exec.CommandContext(ctx, helperScript, "app-status", appID)
		output, err := cmd.Output()
		if err == nil {
			var status map[string]interface{}
			if json.Unmarshal(output, &status) == nil {
				if status["status"] == "running" {
					return true
				}
			}
		}

		select {
		case <-ctx.Done():
			return false
		case <-time.After(2 * time.Second):
			// Continue checking
		}
	}

	return false
}

func (lm *LifecycleManager) copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0600)
}

func (lm *LifecycleManager) logEvent(eventType, appID, userID string, details interface{}) {
	if lm.eventLogger == nil {
		return
	}

	detailsJSON, _ := json.Marshal(details)

	event := Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		AppID:     appID,
		Timestamp: time.Now(),
		User:      userID,
		Details:   detailsJSON,
	}

	if err := lm.eventLogger.LogEvent(event); err != nil {
		fmt.Printf("Failed to log event: %v\n", err)
	}
}

// AppGIDParam is the variable an app's compose template uses to join its
// share-reader group (group_add: ["${NOS_APP_GID}"]). Shares the admin opens
// to the app give that group read access.
const AppGIDParam = "NOS_APP_GID"

// withSystemParams returns a copy of params with the values nOS sets itself
// (they replace anything the user sent under the same name).
func (lm *LifecycleManager) withSystemParams(ctx context.Context, appID string, params map[string]interface{}) (map[string]interface{}, error) {
	gid, err := lm.host.AppOp(ctx, AppOp{Op: "group", AppID: appID})
	if err != nil {
		return nil, fmt.Errorf("failed to set up the app's share group: %w", err)
	}
	gid = strings.TrimSpace(gid)
	if _, err := strconv.Atoi(gid); err != nil {
		return nil, fmt.Errorf("the app's share group has no numeric id: %q", gid)
	}
	out := make(map[string]interface{}, len(params)+1)
	for k, v := range params {
		out[k] = v
	}
	out[AppGIDParam] = gid
	return out, nil
}
