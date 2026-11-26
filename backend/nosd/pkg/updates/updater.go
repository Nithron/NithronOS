package updates

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	stateFilePath = "/var/lib/nos-update/state.json"
	lockFilePath  = "/var/run/nos-update.lock"
	maxFailures   = 3
)

// Updater manages system updates
type Updater struct {
	mu          sync.RWMutex
	config      *UpdateConfig
	state       *UpdateStateMachine
	aptManager  *APTManager
	snapManager *SnapshotManager
	lockFile    *os.File

	// Channels for progress updates
	progressChan chan UpdateProgress
	stopChan     chan struct{}
}

// GetConfig returns the update configuration
func (u *Updater) GetConfig() *UpdateConfig {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.config
}

// GetProgressChannel returns a channel for receiving progress updates
func (u *Updater) GetProgressChannel() <-chan UpdateProgress {
	return u.progressChan
}

// ReleaseProgressChannel releases a progress channel
func (u *Updater) ReleaseProgressChannel(ch <-chan UpdateProgress) {
	// No-op for now, but could be used for cleanup
}

// DeleteSnapshot deletes a snapshot by ID
func (u *Updater) DeleteSnapshot(id string) error {
	return u.snapManager.DeleteSnapshot(id)
}

// NewUpdater creates a new updater instance
func NewUpdater(config *UpdateConfig) (*Updater, error) {
	// Create state directory
	stateDir := filepath.Dir(stateFilePath)
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	// Load or initialize state
	state, err := loadState(stateFilePath)
	if err != nil {
		// Initialize new state
		state = &UpdateStateMachine{
			CurrentState: UpdateStateIdle,
			Snapshots:    []UpdateSnapshot{},
		}
	}

	// Create managers
	aptManager := NewAPTManager(config.RepoURL, config.GPGKeyID)
	snapManager := NewSnapshotManager(config.SnapshotRetention)

	// Set channel
	if err := aptManager.SetChannel(config.Channel); err != nil {
		return nil, fmt.Errorf("failed to set channel: %w", err)
	}

	updater := &Updater{
		config:       config,
		state:        state,
		aptManager:   aptManager,
		snapManager:  snapManager,
		progressChan: make(chan UpdateProgress, 100),
		stopChan:     make(chan struct{}),
	}

	// Resume if update was interrupted
	if state.CurrentState != UpdateStateIdle && state.CurrentState != UpdateStateSuccess {
		go updater.resumeUpdate()
	}

	return updater, nil
}

// AcquireLock acquires an exclusive lock for updates
func (u *Updater) AcquireLock() error {
	lockFile, err := os.OpenFile(lockFilePath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open lock file: %w", err)
	}

	// Try to acquire exclusive lock (platform-specific)
	if err := acquireFileLock(lockFile); err != nil {
		lockFile.Close()
		return fmt.Errorf("another update is already in progress")
	}

	u.lockFile = lockFile
	return nil
}

// ReleaseLock releases the update lock
func (u *Updater) ReleaseLock() {
	if u.lockFile != nil {
		if err := releaseFileLock(u.lockFile); err != nil {
			fmt.Printf("Failed to release file lock: %v\n", err)
		}
		u.lockFile.Close()
		u.lockFile = nil
	}
}

// GetVersion returns the current system version
func (u *Updater) GetVersion() (*SystemVersion, error) {
	// Get package versions
	nosdVersion, _ := u.aptManager.GetPackageVersion("nosd")
	agentVersion, _ := u.aptManager.GetPackageVersion("nos-agent")
	webVersion, _ := u.aptManager.GetPackageVersion("nos-web")

	// Get kernel version
	kernel := fmt.Sprintf("%s %s", runtime.GOOS, runtime.Version())
	if data, err := os.ReadFile("/proc/version"); err == nil {
		parts := strings.Fields(string(data))
		if len(parts) >= 3 {
			kernel = parts[2]
		}
	}

	// Get OS version
	osVersion := "nOS"
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				osVersion = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
				break
			}
		}
	}

	// Get current channel
	channel, _ := u.aptManager.GetChannel()

	version := &SystemVersion{
		OSVersion:    osVersion,
		Kernel:       kernel,
		NosdVersion:  nosdVersion,
		AgentVersion: agentVersion,
		WebUIVersion: webVersion,
		Channel:      channel,
		BuildDate:    time.Now(), // This would come from package metadata
	}

	return version, nil
}

// CheckForUpdates checks for available updates
func (u *Updater) CheckForUpdates() (*UpdateCheckResponse, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	// Update state
	u.state.CurrentState = UpdateStateChecking
	u.saveState()

	// Get current version
	currentVersion, err := u.GetVersion()
	if err != nil {
		u.state.CurrentState = UpdateStateIdle
		u.saveState()
		return nil, fmt.Errorf("failed to get current version: %w", err)
	}

	// Check for updates
	packages, err := u.aptManager.CheckForUpdates()
	if err != nil {
		u.state.CurrentState = UpdateStateIdle
		u.saveState()
		return nil, fmt.Errorf("failed to check for updates: %w", err)
	}

	// Update last check time
	now := time.Now()
	u.state.LastCheck = &now

	response := &UpdateCheckResponse{
		UpdateAvailable: len(packages) > 0,
		CurrentVersion:  *currentVersion,
		LastCheck:       now,
	}

	if len(packages) > 0 {
		// Create available update info
		update := &AvailableUpdate{
			Version:        u.getUpdateVersion(packages),
			Channel:        currentVersion.Channel,
			ReleaseDate:    time.Now(), // Would come from repository metadata
			Packages:       packages,
			RequiresReboot: u.requiresReboot(packages),
		}

		response.LatestVersion = update
		u.state.PendingUpdate = update
	}

	u.state.CurrentState = UpdateStateIdle
	u.saveState()

	return response, nil
}

// ApplyUpdate applies an available update
func (u *Updater) ApplyUpdate(request *UpdateApplyRequest) error {
	// Acquire lock
	if err := u.AcquireLock(); err != nil {
		return err
	}
	defer u.ReleaseLock()

	u.mu.Lock()
	defer u.mu.Unlock()

	// Check if an update is already in progress
	if u.state.CurrentState != UpdateStateIdle {
		return fmt.Errorf("update already in progress")
	}

	// Start update process
	u.state.CurrentState = UpdateStateApplying
	u.state.CurrentProgress = &UpdateProgress{
		State:     UpdateStateApplying,
		Phase:     UpdatePhasePreflight,
		Progress:  0,
		StartedAt: time.Now(),
		Logs:      []LogEntry{},
	}
	u.saveState()

	// Run update in background
	go u.runUpdate(request)

	return nil
}

// runUpdate performs the actual update process
func (u *Updater) runUpdate(request *UpdateApplyRequest) {
	defer func() {
		if r := recover(); r != nil {
			u.handleUpdateFailure(fmt.Sprintf("panic during update: %v", r))
		}
	}()

	// Phase 1: Preflight checks
	if err := u.runPreflight(); err != nil {
		u.handleUpdateFailure(fmt.Sprintf("preflight failed: %v", err))
		return
	}

	// Phase 2: Create snapshot
	if !request.SkipSnapshot {
		if err := u.createUpdateSnapshot(); err != nil {
			u.handleUpdateFailure(fmt.Sprintf("snapshot creation failed: %v", err))
			return
		}
	}

	// Phase 3: Download packages
	if err := u.downloadPackages(); err != nil {
		u.handleUpdateFailure(fmt.Sprintf("package download failed: %v", err))
		return
	}

	// Phase 4: Install updates
	if err := u.installUpdates(); err != nil {
		u.handleUpdateFailure(fmt.Sprintf("installation failed: %v", err))
		if !request.SkipSnapshot {
			u.triggerRollback()
		}
		return
	}

	// Phase 5: Postflight checks
	if err := u.runPostflight(); err != nil {
		u.handleUpdateFailure(fmt.Sprintf("postflight failed: %v", err))
		if !request.SkipSnapshot {
			u.triggerRollback()
		}
		return
	}

	// Phase 6: Cleanup
	u.cleanup()

	// Update successful
	u.mu.Lock()
	u.state.CurrentState = UpdateStateSuccess
	u.state.LastUpdate = &time.Time{}
	*u.state.LastUpdate = time.Now()
	u.state.FailureCount = 0
	if u.state.CurrentProgress != nil {
		now := time.Now()
		u.state.CurrentProgress.CompletedAt = &now
		u.state.CurrentProgress.State = UpdateStateSuccess
		u.state.CurrentProgress.Progress = 100
	}
	u.saveState()
	u.mu.Unlock()

	// Prune old snapshots
	if err := u.snapManager.PruneSnapshots(); err != nil {
		fmt.Printf("Failed to prune snapshots: %v\n", err)
	}
}

// runPreflight runs preflight checks
func (u *Updater) runPreflight() error {
	u.updateProgress(UpdatePhasePreflight, 10, "Running preflight checks...")

	checks := []PreflightCheck{}

	// Check disk space (platform-specific implementation)
	availableGB := getAvailableDiskSpace("/")
	if availableGB < 2 {
		checks = append(checks, PreflightCheck{
			CheckType: "disk_space",
			Status:    "fail",
			Message:   fmt.Sprintf("Insufficient disk space: %d GB available, 2 GB required", availableGB),
			Required:  true,
		})
	}

	// Check network connectivity
	if err := u.aptManager.Update(); err != nil {
		checks = append(checks, PreflightCheck{
			CheckType: "network",
			Status:    "fail",
			Message:   fmt.Sprintf("Cannot reach update repository: %v", err),
			Required:  true,
		})
	}

	// Verify signatures
	if err := u.aptManager.VerifySignatures(); err != nil {
		checks = append(checks, PreflightCheck{
			CheckType: "signature",
			Status:    "fail",
			Message:   fmt.Sprintf("Signature verification failed: %v", err),
			Required:  true,
		})
	}

	// Check for failures
	for _, check := range checks {
		if check.Status == "fail" && check.Required {
			return fmt.Errorf("%s", check.Message)
		}
	}

	u.updateProgress(UpdatePhasePreflight, 20, "Preflight checks passed")
	return nil
}

// createUpdateSnapshot creates a pre-update snapshot
func (u *Updater) createUpdateSnapshot() error {
	u.updateProgress(UpdatePhaseSnapshot, 30, "Creating system snapshot...")

	snapshotID := fmt.Sprintf("update-%d", time.Now().Unix())
	version, _ := u.GetVersion()
	description := fmt.Sprintf("Pre-update snapshot (from %s)", version.NosdVersion)

	snapshot, err := u.snapManager.CreateSnapshot(snapshotID, description)
	if err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	// Update state
	u.mu.Lock()
	u.state.Snapshots = append(u.state.Snapshots, *snapshot)
	if u.state.CurrentProgress != nil {
		u.state.CurrentProgress.SnapshotID = snapshot.ID
	}
	u.saveState()
	u.mu.Unlock()

	u.updateProgress(UpdatePhaseSnapshot, 40, fmt.Sprintf("Snapshot created: %s", snapshot.ID))
	return nil
}

// downloadPackages downloads update packages
func (u *Updater) downloadPackages() error {
	u.updateProgress(UpdatePhaseDownload, 50, "Downloading packages...")

	// APT will handle the download during dist-upgrade
	// Here we could pre-download if needed

	u.updateProgress(UpdatePhaseDownload, 60, "Packages ready")
	return nil
}

// installUpdates installs the updates
func (u *Updater) installUpdates() error {
	u.updateProgress(UpdatePhaseInstall, 70, "Installing updates...")

	// Perform the actual upgrade
	if err := u.aptManager.DistUpgrade(); err != nil {
		return fmt.Errorf("dist-upgrade failed: %w", err)
	}

	u.updateProgress(UpdatePhaseInstall, 85, "Updates installed")
	return nil
}

// runPostflight runs postflight checks
func (u *Updater) runPostflight() error {
	u.updateProgress(UpdatePhasePostflight, 90, "Running postflight checks...")

	checks := []PostflightCheck{}

	// Check critical services
	services := []string{"nosd", "nos-agent", "caddy"}
	for _, service := range services {
		if !u.isServiceRunning(service) {
			checks = append(checks, PostflightCheck{
				Service:  service,
				Status:   "stopped",
				Healthy:  false,
				Critical: true,
				Message:  fmt.Sprintf("Service %s is not running", service),
			})
		}
	}

	// Check UI reachability
	if !u.checkUIReachability() {
		checks = append(checks, PostflightCheck{
			Service:  "web-ui",
			Status:   "unreachable",
			Healthy:  false,
			Critical: true,
			Message:  "Web UI is not reachable",
		})
	}

	// Check for critical failures
	for _, check := range checks {
		if !check.Healthy && check.Critical {
			return fmt.Errorf("%s", check.Message)
		}
	}

	u.updateProgress(UpdatePhasePostflight, 95, "Postflight checks passed")
	return nil
}

// cleanup performs post-update cleanup
func (u *Updater) cleanup() {
	u.updateProgress(UpdatePhaseCleanup, 98, "Cleaning up...")

	// Clean APT cache
	if err := u.aptManager.CleanCache(); err != nil {
		fmt.Printf("Failed to clean apt cache: %v\n", err)
	}

	u.updateProgress(UpdatePhaseCleanup, 100, "Update complete")
}

// Helper methods

func (u *Updater) updateProgress(phase UpdatePhase, progress int, message string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.state.CurrentProgress != nil {
		u.state.CurrentProgress.Phase = phase
		u.state.CurrentProgress.Progress = progress
		u.state.CurrentProgress.Message = message
		u.state.CurrentProgress.Logs = append(u.state.CurrentProgress.Logs, LogEntry{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   message,
			Phase:     phase,
		})

		// Send to progress channel
		select {
		case u.progressChan <- *u.state.CurrentProgress:
		default:
		}

		u.saveState()
	}
}

func (u *Updater) handleUpdateFailure(message string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.state.CurrentState = UpdateStateFailed
	u.state.LastError = message
	u.state.FailureCount++

	if u.state.CurrentProgress != nil {
		u.state.CurrentProgress.State = UpdateStateFailed
		u.state.CurrentProgress.Error = message
		now := time.Now()
		u.state.CurrentProgress.CompletedAt = &now
	}

	u.saveState()
}

func (u *Updater) triggerRollback() {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.state.CurrentState = UpdateStateRollingBack
	u.saveState()

	// Find the latest snapshot
	if u.state.CurrentProgress != nil && u.state.CurrentProgress.SnapshotID != "" {
		if err := u.snapManager.Rollback(u.state.CurrentProgress.SnapshotID); err != nil {
			u.state.LastError = fmt.Sprintf("Rollback failed: %v", err)
		} else {
			u.state.CurrentState = UpdateStateRolledBack
		}
	}

	u.saveState()
}

func (u *Updater) isServiceRunning(service string) bool {
	cmd := exec.Command("systemctl", "is-active", service)
	output, _ := cmd.Output()
	return strings.TrimSpace(string(output)) == "active"
}

func (u *Updater) checkUIReachability() bool {
	// Try to reach the UI through localhost
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://localhost/api/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (u *Updater) getUpdateVersion(packages []Package) string {
	// Find the main package version
	for _, pkg := range packages {
		if pkg.Name == "nosd" {
			return pkg.NewVersion
		}
	}
	if len(packages) > 0 {
		return packages[0].NewVersion
	}
	return "unknown"
}

func (u *Updater) requiresReboot(packages []Package) bool {
	// Check if kernel or critical system packages are being updated
	for _, pkg := range packages {
		if strings.HasPrefix(pkg.Name, "linux-") ||
			pkg.Name == "systemd" ||
			pkg.Name == "libc6" {
			return true
		}
	}
	return false
}

func (u *Updater) resumeUpdate() {
	// Resume interrupted update based on state
	// This would check the current state and continue from where it left off
	// For now, we'll just mark it as failed
	u.handleUpdateFailure("Update was interrupted and cannot be resumed")
}

func (u *Updater) saveState() {
	data, _ := json.MarshalIndent(u.state, "", "  ")
	if err := os.WriteFile(stateFilePath, data, 0600); err != nil {
		fmt.Printf("Failed to write state file: %v\n", err)
	}
}

func loadState(path string) (*UpdateStateMachine, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var state UpdateStateMachine
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// GetProgress returns the current update progress
func (u *Updater) GetProgress() *UpdateProgress {
	u.mu.RLock()
	defer u.mu.RUnlock()

	if u.state.CurrentProgress != nil {
		return u.state.CurrentProgress
	}

	return &UpdateProgress{
		State:   u.state.CurrentState,
		Message: "No update in progress",
	}
}

// Rollback performs a manual rollback to a snapshot
func (u *Updater) Rollback(request *RollbackRequest) error {
	// Acquire lock
	if err := u.AcquireLock(); err != nil {
		return err
	}
	defer u.ReleaseLock()

	u.mu.Lock()
	defer u.mu.Unlock()

	// Verify snapshot exists
	if err := u.snapManager.VerifySnapshot(request.SnapshotID); err != nil {
		return fmt.Errorf("invalid snapshot: %w", err)
	}

	// Perform rollback
	u.state.CurrentState = UpdateStateRollingBack
	u.saveState()

	if err := u.snapManager.Rollback(request.SnapshotID); err != nil {
		u.state.CurrentState = UpdateStateFailed
		u.state.LastError = err.Error()
		u.saveState()
		return err
	}

	u.state.CurrentState = UpdateStateRolledBack
	u.saveState()

	return nil
}

// ListSnapshots returns all available snapshots
func (u *Updater) ListSnapshots() ([]UpdateSnapshot, error) {
	return u.snapManager.ListSnapshots()
}

// SetChannel changes the update channel
func (u *Updater) SetChannel(channel Channel) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if err := u.aptManager.SetChannel(channel); err != nil {
		return err
	}

	u.config.Channel = channel
	return nil
}
