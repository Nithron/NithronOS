package backup

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// KnownHostsFile holds the SSH host keys of backup destinations (nosd tests
// connections as nos, nos-agent sends as root; both use this file).
const KnownHostsFile = "/var/lib/nos/backup/known_hosts"

// Replicator handles backup replication to remote destinations
type Replicator struct {
	logger       zerolog.Logger
	destinations map[string]*Destination
	stateFile    string
	keysDir      string
	mu           sync.RWMutex
	jobManager   *JobManager

	// Set by Connect: the agent sends snapshots, snapshot finds them.
	agent     AgentClient
	snapshot  func(id string) (*Snapshot, bool)
	pollEvery time.Duration
}

// Connect gives the replicator the agent and the scheduler's snapshots.
func (r *Replicator) Connect(agent AgentClient, snapshots func() []*Snapshot) {
	r.agent = agent
	r.pollEvery = 3 * time.Second
	r.snapshot = func(id string) (*Snapshot, bool) {
		for _, s := range snapshots() {
			if s.ID == id {
				return s, true
			}
		}
		return nil, false
	}
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
	if err := r.saveStateLocked(); err != nil {
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
	if err := r.saveStateLocked(); err != nil {
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
	if err := r.saveStateLocked(); err != nil {
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

// The same rules as nos-agent's /v1/backup/send: these values reach ssh and
// the remote shell, so only plain names and paths are accepted.
var (
	reDestHost = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.:-]{0,252}$`)
	reDestUser = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
	reDestPath = regexp.MustCompile(`^/[A-Za-z0-9._/-]{0,254}$`)
)

func (r *Replicator) validateDestination(dest *Destination) error {
	if dest.Name == "" {
		return fmt.Errorf("destination name is required")
	}
	pathOK := reDestPath.MatchString(dest.Path) && !strings.Contains(dest.Path, "..")
	switch dest.Type {
	case "ssh":
		if !reDestHost.MatchString(dest.Host) {
			return fmt.Errorf("SSH host: a host name or IP address")
		}
		if !reDestUser.MatchString(dest.User) {
			return fmt.Errorf("SSH user: a plain user name")
		}
		if !pathOK {
			return fmt.Errorf("SSH path: an absolute path of letters, digits, '.', '_', '-' and '/'")
		}
		if dest.Port < 0 || dest.Port > 65535 {
			return fmt.Errorf("SSH port out of range")
		}
	case "local":
		if !pathOK {
			return fmt.Errorf("local path: an absolute path of letters, digits, '.', '_', '-' and '/'")
		}
	case "rclone":
		return fmt.Errorf("rclone destinations are not supported; use ssh or local")
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
		"-o", "UserKnownHostsFile=" + KnownHostsFile,
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
	sshArgs = append(sshArgs, "--", fmt.Sprintf("%s@%s", dest.User, dest.Host))

	// Test command
	sshArgs = append(sshArgs, "echo", "test")

	// Execute
	cmd := exec.Command("ssh", sshArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w\nOutput: %s", err, string(output))
	}

	// Check if path exists
	// dest.Path is validated (plain characters only); the remote shell runs it.
	sshArgs[len(sshArgs)-2] = fmt.Sprintf("test -d %s || mkdir -p %s", dest.Path, dest.Path)
	sshArgs[len(sshArgs)-1] = ""
	cmd = exec.Command("ssh", sshArgs[:len(sshArgs)-1]...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to access/create remote path: %w", err)
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
	job.State = JobStateRunning
	r.jobManager.UpdateJob(job)
	r.jobManager.AddLogEntry(job.ID, "info", fmt.Sprintf("Starting replication to %s", dest.Name))

	err := r.send(job, dest, snapshotID, baseSnapshotID)

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

// send has nos-agent stream the snapshot with btrfs send (incrementally
// from baseSnapshotID when given) and follows it until it ends.
func (r *Replicator) send(job *BackupJob, dest *Destination, snapshotID, baseSnapshotID string) error {
	if r.agent == nil {
		return fmt.Errorf("replication is not connected to nos-agent")
	}
	snap, ok := r.snapshot(snapshotID)
	if !ok {
		return fmt.Errorf("snapshot not found: %s", snapshotID)
	}
	req := SendRequest{Job: job.ID, Snapshot: snap.Path, Path: dest.Path}
	if baseSnapshotID != "" {
		base, ok := r.snapshot(baseSnapshotID)
		if !ok {
			return fmt.Errorf("base snapshot not found: %s", baseSnapshotID)
		}
		req.Parent = base.Path
	}
	switch dest.Type {
	case "ssh":
		req.Target, req.Host, req.Port, req.User, req.Key = "ssh", dest.Host, dest.Port, dest.User, dest.KeyRef
	case "local":
		req.Target = "local"
	default:
		return fmt.Errorf("%s destinations are not supported; use ssh or local", dest.Type)
	}
	if err := r.agent.Send(req); err != nil {
		return err
	}
	r.jobManager.AddLogEntry(job.ID, "info", "Sending "+snap.Path)
	seen := 0
	start := time.Now()
	for {
		st, err := r.agent.SendStatus(job.ID)
		if err == nil {
			for _, line := range st.Log[min(seen, len(st.Log)):] {
				r.jobManager.AddLogEntry(job.ID, "info", line)
			}
			seen = len(st.Log)
			switch st.State {
			case "succeeded":
				return nil
			case "failed":
				return fmt.Errorf("btrfs send exited with status %d; see the job log", st.ExitStatus)
			case "none":
				if time.Since(start) > time.Minute {
					return fmt.Errorf("the replication did not start")
				}
			}
		}
		time.Sleep(r.pollEvery)
	}
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

// saveState writes the state file; the caller must not hold mu.
func (r *Replicator) saveState() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.saveStateLocked()
}

// saveStateLocked writes the state file; the caller holds mu. (saveState
// used to take mu again while callers held it, which deadlocked.)
func (r *Replicator) saveStateLocked() error {
	state := struct {
		Destinations map[string]*Destination `json:"destinations"`
	}{
		Destinations: r.destinations,
	}

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
	return r.saveStateLocked()
}
