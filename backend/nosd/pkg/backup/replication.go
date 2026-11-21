package backup

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Replicator handles backup replication to remote destinations
type Replicator struct {
	logger       zerolog.Logger
	destinations map[string]*Destination
	stateFile    string
	keysDir      string
	mu           sync.RWMutex
	jobManager   *JobManager
}

// NewReplicator creates a new replicator
func NewReplicator(logger zerolog.Logger, stateFile string, keysDir string, jobManager *JobManager) *Replicator {
	return &Replicator{
		logger:       logger.With().Str("component", "replicator").Logger(),
		destinations: make(map[string]*Destination),
		stateFile:    stateFile,
		keysDir:      keysDir,
		jobManager:   jobManager,
	}
}

// Start initializes the replicator
func (r *Replicator) Start() error {
	// Create keys directory if it doesn't exist
	if err := os.MkdirAll(r.keysDir, 0700); err != nil {
		return fmt.Errorf("failed to create keys directory: %w", err)
	}

	// Load state
	if err := r.loadState(); err != nil {
		r.logger.Warn().Err(err).Msg("Failed to load replicator state")
	}

	return nil
}

// Stop shuts down the replicator
func (r *Replicator) Stop() error {
	return r.saveState()
}

// CreateDestination creates a new replication destination
func (r *Replicator) CreateDestination(dest *Destination) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Generate ID if not provided
	if dest.ID == "" {
		dest.ID = uuid.New().String()
	}

	// Set timestamps
	now := time.Now()
	dest.CreatedAt = now
	dest.UpdatedAt = now

	// Validate destination
	if err := r.validateDestination(dest); err != nil {
		return fmt.Errorf("invalid destination: %w", err)
	}

	// Set defaults
	if dest.Type == "ssh" && dest.Port == 0 {
		dest.Port = 22
	}
	if dest.RetryCount == 0 {
		dest.RetryCount = 3
	}
	if dest.Concurrency == 0 {
		dest.Concurrency = 1
	}

	// Store destination
	r.destinations[dest.ID] = dest

	// Save state
	if err := r.saveState(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	r.logger.Info().Str("id", dest.ID).Str("name", dest.Name).Msg("Created replication destination")
	return nil
}

// UpdateDestination updates an existing destination
func (r *Replicator) UpdateDestination(id string, update *Destination) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.destinations[id]
	if !ok {
		return fmt.Errorf("destination not found: %s", id)
	}

	// Preserve immutable fields
	update.ID = existing.ID
	update.CreatedAt = existing.CreatedAt
	update.UpdatedAt = time.Now()

	// Validate
	if err := r.validateDestination(update); err != nil {
		return fmt.Errorf("invalid destination: %w", err)
	}

	// Update destination
	r.destinations[id] = update

	// Save state
	if err := r.saveState(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	r.logger.Info().Str("id", id).Msg("Updated replication destination")
	return nil
}

// DeleteDestination deletes a destination
func (r *Replicator) DeleteDestination(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	dest, ok := r.destinations[id]
	if !ok {
		return fmt.Errorf("destination not found: %s", id)
	}

	// Delete associated SSH key if exists
	if dest.Type == "ssh" && dest.KeyRef != "" {
		keyPath := filepath.Join(r.keysDir, dest.KeyRef)
		if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
			r.logger.Warn().Err(err).Str("key", keyPath).Msg("Failed to delete SSH key")
		}
	}

	// Delete destination
	delete(r.destinations, id)

	// Save state
	if err := r.saveState(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	r.logger.Info().Str("id", id).Msg("Deleted replication destination")
	return nil
}

// GetDestination returns a destination by ID
func (r *Replicator) GetDestination(id string) (*Destination, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	dest, ok := r.destinations[id]
	if !ok {
		return nil, fmt.Errorf("destination not found: %s", id)
	}

	return dest, nil
}

// ListDestinations returns all destinations
func (r *Replicator) ListDestinations() []*Destination {
	r.mu.RLock()
	defer r.mu.RUnlock()

	destinations := make([]*Destination, 0, len(r.destinations))
	for _, dest := range r.destinations {
		destinations = append(destinations, dest)
	}

	return destinations
}

// TestDestination tests connectivity to a destination
func (r *Replicator) TestDestination(id string) error {
	r.mu.RLock()
	dest, ok := r.destinations[id]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("destination not found: %s", id)
	}

	var err error
	switch dest.Type {
	case "ssh":
		err = r.testSSHDestination(dest)
	case "rclone":
		err = r.testRcloneDestination(dest)
	case "local":
		err = r.testLocalDestination(dest)
	default:
		err = fmt.Errorf("unsupported destination type: %s", dest.Type)
	}

	// Update test status
	r.mu.Lock()
	now := time.Now()
	dest.LastTest = &now
	if err != nil {
		dest.LastTestStatus = fmt.Sprintf("failed: %v", err)
	} else {
		dest.LastTestStatus = "success"
	}
	r.mu.Unlock()

	_ = r.saveState()

	return err
}

// Replicate starts a replication job
func (r *Replicator) Replicate(destID string, snapshotID string, baseSnapshotID string) (*BackupJob, error) {
	r.mu.RLock()
	dest, ok := r.destinations[destID]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("destination not found: %s", destID)
	}

	if !dest.Enabled {
		return nil, fmt.Errorf("destination is disabled")
	}

	// Create job
	job := &BackupJob{
		ID:            uuid.New().String(),
		Type:          "replicate",
		State:         JobStatePending,
		DestinationID: destID,
		SnapshotID:    snapshotID,
		Incremental:   baseSnapshotID != "",
		BaseSnapshot:  baseSnapshotID,
		StartedAt:     time.Now(),
	}

	// Add to job manager
	r.jobManager.AddJob(job)

	// Run replication in background
	go r.runReplication(job, dest, snapshotID, baseSnapshotID)

	return job, nil
}

// Private methods

func (r *Replicator) validateDestination(dest *Destination) error {
	if dest.Name == "" {
		return fmt.Errorf("destination name is required")
	}

	switch dest.Type {
	case "ssh":
		if dest.Host == "" {
			return fmt.Errorf("SSH host is required")
		}
		if dest.User == "" {
			return fmt.Errorf("SSH user is required")
		}
		if dest.Path == "" {
			return fmt.Errorf("SSH path is required")
		}
	case "rclone":
		if dest.RemoteName == "" {
			return fmt.Errorf("rclone remote name is required")
		}
		if dest.RemotePath == "" {
			return fmt.Errorf("rclone remote path is required")
		}
	case "local":
		if dest.Path == "" {
			return fmt.Errorf("local path is required")
		}
	default:
		return fmt.Errorf("invalid destination type: %s", dest.Type)
	}

	return nil
}

func (r *Replicator) testSSHDestination(dest *Destination) error {
	// Build SSH command
	sshArgs := []string{
		"-o", "ConnectTimeout=10",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "UserKnownHostsFile=/var/lib/nos/backup/known_hosts",
		"-o", "BatchMode=yes",
		"-p", fmt.Sprintf("%d", dest.Port),
	}

	// Add key if specified
	if dest.KeyRef != "" {
		keyPath := filepath.Join(r.keysDir, dest.KeyRef)
		if _, err := os.Stat(keyPath); err != nil {
			return fmt.Errorf("SSH key not found: %w", err)
		}
		sshArgs = append(sshArgs, "-i", keyPath)
	}

	// Add user@host
	sshArgs = append(sshArgs, fmt.Sprintf("%s@%s", dest.User, dest.Host))

	// Test command
	sshArgs = append(sshArgs, "echo", "test")

	// Execute
	cmd := exec.Command("ssh", sshArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w\nOutput: %s", err, string(output))
	}

	// Check if path exists
	sshArgs[len(sshArgs)-2] = fmt.Sprintf("test -d %s || mkdir -p %s", dest.Path, dest.Path)
	sshArgs[len(sshArgs)-1] = ""
	cmd = exec.Command("ssh", sshArgs[:len(sshArgs)-1]...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to access/create remote path: %w", err)
	}

	return nil
}

func (r *Replicator) testRcloneDestination(dest *Destination) error {
	// Check if rclone is installed
	if _, err := exec.LookPath("rclone"); err != nil {
		return fmt.Errorf("rclone not found: %w", err)
	}

	// Test remote
	cmd := exec.Command("rclone", "lsd", fmt.Sprintf("%s:%s", dest.RemoteName, dest.RemotePath))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rclone test failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func (r *Replicator) testLocalDestination(dest *Destination) error {
	// Check if path exists or can be created
	if err := os.MkdirAll(dest.Path, 0755); err != nil {
		return fmt.Errorf("cannot access local path: %w", err)
	}

	// Check if writable
	testFile := filepath.Join(dest.Path, ".test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("path is not writable: %w", err)
	}
	os.Remove(testFile)

	return nil
}

func (r *Replicator) runReplication(job *BackupJob, dest *Destination, snapshotID string, baseSnapshotID string) {
	// Update job state
	job.State = JobStateRunning
	r.jobManager.UpdateJob(job)

	// Log start
	r.jobManager.AddLogEntry(job.ID, "info", fmt.Sprintf("Starting replication to %s", dest.Name))

	// TODO: Get snapshot details from snapshot manager
	// For now, use placeholder paths
	snapshotPath := fmt.Sprintf("@snapshots/test/%s", snapshotID)

	var err error
	switch dest.Type {
	case "ssh":
		err = r.replicateSSH(job, dest, snapshotPath, baseSnapshotID)
	case "rclone":
		err = r.replicateRclone(job, dest, snapshotPath)
	case "local":
		err = r.replicateLocal(job, dest, snapshotPath, baseSnapshotID)
	default:
		err = fmt.Errorf("unsupported destination type: %s", dest.Type)
	}

	// Update job state
	now := time.Now()
	job.FinishedAt = &now

	if err != nil {
		job.State = JobStateFailed
		job.Error = err.Error()
		r.jobManager.AddLogEntry(job.ID, "error", fmt.Sprintf("Replication failed: %v", err))
		r.logger.Error().Err(err).Str("job", job.ID).Msg("Replication failed")
	} else {
		job.State = JobStateSucceeded
		job.Progress = 100
		r.jobManager.AddLogEntry(job.ID, "info", "Replication completed successfully")
		r.logger.Info().Str("job", job.ID).Msg("Replication completed")
	}

	r.jobManager.UpdateJob(job)
}

func (r *Replicator) replicateSSH(job *BackupJob, dest *Destination, snapshotPath string, baseSnapshotID string) error {
	// Build btrfs send command
	sendArgs := []string{"send"}

	// Add parent for incremental
	if baseSnapshotID != "" {
		parentPath := fmt.Sprintf("@snapshots/test/%s", baseSnapshotID)
		sendArgs = append(sendArgs, "-p", parentPath)
	}

	sendArgs = append(sendArgs, snapshotPath)

	// Build SSH command
	sshArgs := []string{
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "UserKnownHostsFile=/var/lib/nos/backup/known_hosts",
		"-o", "BatchMode=yes",
		"-p", fmt.Sprintf("%d", dest.Port),
	}

	// Add key if specified
	if dest.KeyRef != "" {
		keyPath := filepath.Join(r.keysDir, dest.KeyRef)
		sshArgs = append(sshArgs, "-i", keyPath)
	}

	// Add user@host and receive command
	// remotePath := filepath.Join(dest.Path, filepath.Base(snapshotPath)) // TODO: use for validation
	sshArgs = append(sshArgs,
		fmt.Sprintf("%s@%s", dest.User, dest.Host),
		fmt.Sprintf("btrfs receive %s", dest.Path),
	)

	// Create send command
	sendCmd := exec.Command("btrfs", sendArgs...)

	// Create SSH command
	sshCmd := exec.Command("ssh", sshArgs...)

	// Create pipe
	pipe, err := sendCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}
	sshCmd.Stdin = pipe

	// Handle bandwidth limiting if specified
	if dest.BandwidthLimit > 0 {
		// Use pv for bandwidth limiting
		if _, err := exec.LookPath("pv"); err == nil {
			pvCmd := exec.Command("pv", "-L", fmt.Sprintf("%dk", dest.BandwidthLimit))
			pvCmd.Stdin = pipe

			pvPipe, err := pvCmd.StdoutPipe()
			if err != nil {
				return fmt.Errorf("failed to create pv pipe: %w", err)
			}
			sshCmd.Stdin = pvPipe

			// Start pv
			if err := pvCmd.Start(); err != nil {
				return fmt.Errorf("failed to start pv: %w", err)
			}
			defer func() { _ = pvCmd.Wait() }()
		}
	}

	// Capture SSH stderr for logging
	sshStderr, err := sshCmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start SSH command
	if err := sshCmd.Start(); err != nil {
		return fmt.Errorf("failed to start SSH: %w", err)
	}

	// Start send command
	if err := sendCmd.Start(); err != nil {
		return fmt.Errorf("failed to start btrfs send: %w", err)
	}

	// Read SSH stderr for progress
	go func() {
		scanner := bufio.NewScanner(sshStderr)
		for scanner.Scan() {
			line := scanner.Text()
			r.jobManager.AddLogEntry(job.ID, "info", line)
		}
	}()

	// Wait for send to complete
	if err := sendCmd.Wait(); err != nil {
		return fmt.Errorf("btrfs send failed: %w", err)
	}

	// Close pipe
	pipe.Close()

	// Wait for SSH to complete
	if err := sshCmd.Wait(); err != nil {
		return fmt.Errorf("SSH receive failed: %w", err)
	}

	return nil
}

func (r *Replicator) replicateRclone(job *BackupJob, dest *Destination, snapshotPath string) error {
	// Check if rclone is installed
	if _, err := exec.LookPath("rclone"); err != nil {
		return fmt.Errorf("rclone not found: %w", err)
	}

	// Create temporary mount point
	mountPoint := fmt.Sprintf("/tmp/backup-mount-%s", job.ID)
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		return fmt.Errorf("failed to create mount point: %w", err)
	}
	defer os.RemoveAll(mountPoint)

	// Mount snapshot read-only
	mountCmd := exec.Command("mount", "-o", "ro,subvol="+snapshotPath, "/dev/mapper/nos-root", mountPoint)
	if err := mountCmd.Run(); err != nil {
		return fmt.Errorf("failed to mount snapshot: %w", err)
	}
	defer func() { _ = exec.Command("umount", mountPoint).Run() }()

	// Build rclone command
	rcloneArgs := []string{
		"sync",
		mountPoint,
		fmt.Sprintf("%s:%s/%s", dest.RemoteName, dest.RemotePath, filepath.Base(snapshotPath)),
		"--progress",
	}

	// Add bandwidth limit if specified
	if dest.BandwidthLimit > 0 {
		rcloneArgs = append(rcloneArgs, "--bwlimit", fmt.Sprintf("%dk", dest.BandwidthLimit))
	}

	// Add transfers limit if specified
	if dest.Concurrency > 0 {
		rcloneArgs = append(rcloneArgs, "--transfers", fmt.Sprintf("%d", dest.Concurrency))
	}

	// Execute rclone
	cmd := exec.Command("rclone", rcloneArgs...)

	// Capture output for progress
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start rclone: %w", err)
	}

	// Read progress
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		r.jobManager.AddLogEntry(job.ID, "info", line)

		// Parse progress if possible
		if strings.Contains(line, "%") {
			// Extract percentage
			// This is simplified; real implementation would parse rclone's progress format
			job.Progress = 50 // Placeholder
			r.jobManager.UpdateJob(job)
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("rclone sync failed: %w", err)
	}

	return nil
}

func (r *Replicator) replicateLocal(job *BackupJob, dest *Destination, snapshotPath string, baseSnapshotID string) error {
	// For local replication, use btrfs send/receive to local path
	sendArgs := []string{"send"}

	// Add parent for incremental
	if baseSnapshotID != "" {
		parentPath := fmt.Sprintf("@snapshots/test/%s", baseSnapshotID)
		sendArgs = append(sendArgs, "-p", parentPath)
	}

	sendArgs = append(sendArgs, snapshotPath)

	// Create send command
	sendCmd := exec.Command("btrfs", sendArgs...)

	// Create receive command
	receiveCmd := exec.Command("btrfs", "receive", dest.Path)

	// Create pipe
	pipe, err := sendCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}
	receiveCmd.Stdin = pipe

	// Start receive
	if err := receiveCmd.Start(); err != nil {
		return fmt.Errorf("failed to start btrfs receive: %w", err)
	}

	// Start send
	if err := sendCmd.Start(); err != nil {
		return fmt.Errorf("failed to start btrfs send: %w", err)
	}

	// Wait for send to complete
	if err := sendCmd.Wait(); err != nil {
		return fmt.Errorf("btrfs send failed: %w", err)
	}

	// Close pipe
	pipe.Close()

	// Wait for receive to complete
	if err := receiveCmd.Wait(); err != nil {
		return fmt.Errorf("btrfs receive failed: %w", err)
	}

	return nil
}

func (r *Replicator) loadState() error {
	data, err := os.ReadFile(r.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var state struct {
		Destinations map[string]*Destination `json:"destinations"`
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	r.destinations = state.Destinations

	if r.destinations == nil {
		r.destinations = make(map[string]*Destination)
	}

	return nil
}

func (r *Replicator) saveState() error {
	r.mu.RLock()
	state := struct {
		Destinations map[string]*Destination `json:"destinations"`
	}{
		Destinations: r.destinations,
	}
	r.mu.RUnlock()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	// Write atomically
	tmpFile := r.stateFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return err
	}

	return os.Rename(tmpFile, r.stateFile)
}

// StoreSSHKey stores an SSH private key for a destination
func (r *Replicator) StoreSSHKey(destID string, keyContent string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	dest, ok := r.destinations[destID]
	if !ok {
		return fmt.Errorf("destination not found: %s", destID)
	}

	if dest.Type != "ssh" {
		return fmt.Errorf("destination is not SSH type")
	}

	// Generate key reference if not set
	if dest.KeyRef == "" {
		dest.KeyRef = fmt.Sprintf("%s.key", dest.ID)
		dest.UpdatedAt = time.Now()
	}

	// Write key to file
	keyPath := filepath.Join(r.keysDir, dest.KeyRef)
	if err := os.WriteFile(keyPath, []byte(keyContent), 0600); err != nil {
		return fmt.Errorf("failed to write SSH key: %w", err)
	}

	// Save state
	return r.saveState()
}
