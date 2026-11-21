package backup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Restorer handles backup restore operations
type Restorer struct {
	logger      zerolog.Logger
	agentClient AgentClient
	jobManager  *JobManager
	scheduler   *Scheduler
	replicator  *Replicator
}

// NewRestorer creates a new restorer
func NewRestorer(logger zerolog.Logger, agentClient AgentClient, jobManager *JobManager, scheduler *Scheduler, replicator *Replicator) *Restorer {
	return &Restorer{
		logger:      logger.With().Str("component", "restorer").Logger(),
		agentClient: agentClient,
		jobManager:  jobManager,
		scheduler:   scheduler,
		replicator:  replicator,
	}
}

// CreateRestorePlan creates a plan for restore operation
func (r *Restorer) CreateRestorePlan(sourceType string, sourceID string, restoreType string, targetPath string, dryRun bool) (*RestorePlan, error) {
	plan := &RestorePlan{
		SourceType:  sourceType,
		SourceID:    sourceID,
		RestoreType: restoreType,
		TargetPath:  targetPath,
		DryRun:      dryRun,
		Actions:     []RestoreAction{},
	}

	// Validate source
	var snapshot *Snapshot
	var sourceSnapshot string

	switch sourceType {
	case "local":
		// Get local snapshot
		snapshots := r.scheduler.ListSnapshots()
		for _, s := range snapshots {
			if s.ID == sourceID {
				snapshot = s
				sourceSnapshot = s.Path
				break
			}
		}
		if snapshot == nil {
			return nil, fmt.Errorf("snapshot not found: %s", sourceID)
		}

	case "ssh":
		// For SSH restore, sourceID should be "destination:snapshot"
		parts := strings.SplitN(sourceID, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid SSH source format, expected destination:snapshot")
		}

		dest, err := r.replicator.GetDestination(parts[0])
		if err != nil {
			return nil, fmt.Errorf("destination not found: %w", err)
		}

		if dest.Type != "ssh" {
			return nil, fmt.Errorf("destination is not SSH type")
		}

		sourceSnapshot = parts[1]

	case "rclone":
		// Similar to SSH
		parts := strings.SplitN(sourceID, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid rclone source format")
		}

		dest, err := r.replicator.GetDestination(parts[0])
		if err != nil {
			return nil, fmt.Errorf("destination not found: %w", err)
		}

		if dest.Type != "rclone" {
			return nil, fmt.Errorf("destination is not rclone type")
		}

		sourceSnapshot = parts[1]

	default:
		return nil, fmt.Errorf("unsupported source type: %s", sourceType)
	}

	// Build restore actions based on type
	switch restoreType {
	case "full":
		// Full subvolume restore
		plan.Actions = append(plan.Actions, RestoreAction{
			Type:        "snapshot",
			Target:      targetPath,
			Description: fmt.Sprintf("Create safety snapshot of %s", targetPath),
		})

		// Determine services that need to be stopped
		services := r.getAffectedServices(targetPath)
		for _, service := range services {
			plan.RequiresStop = append(plan.RequiresStop, service)
			plan.Actions = append(plan.Actions, RestoreAction{
				Type:        "stop_service",
				Target:      service,
				Description: fmt.Sprintf("Stop service %s", service),
			})
		}

		// Main restore action
		plan.Actions = append(plan.Actions, RestoreAction{
			Type:        "rollback",
			Target:      targetPath,
			Description: fmt.Sprintf("Replace %s with snapshot %s", targetPath, sourceSnapshot),
		})

		// Restart services
		for _, service := range services {
			plan.Actions = append(plan.Actions, RestoreAction{
				Type:        "start_service",
				Target:      service,
				Description: fmt.Sprintf("Start service %s", service),
			})
		}

		plan.EstimatedTime = 60 + len(services)*10

	case "files":
		// File-level restore
		plan.Actions = append(plan.Actions, RestoreAction{
			Type:        "mount",
			Target:      sourceSnapshot,
			Description: fmt.Sprintf("Mount snapshot %s read-only", sourceSnapshot),
		})

		plan.Actions = append(plan.Actions, RestoreAction{
			Type:        "copy",
			Target:      targetPath,
			Description: fmt.Sprintf("Copy files to %s", targetPath),
		})

		plan.Actions = append(plan.Actions, RestoreAction{
			Type:        "unmount",
			Target:      sourceSnapshot,
			Description: fmt.Sprintf("Unmount snapshot %s", sourceSnapshot),
		})

		plan.EstimatedTime = 30

	default:
		return nil, fmt.Errorf("unsupported restore type: %s", restoreType)
	}

	return plan, nil
}

// ExecuteRestore executes a restore plan
func (r *Restorer) ExecuteRestore(plan *RestorePlan) (*BackupJob, error) {
	if plan.DryRun {
		// Don't actually execute, just return the plan
		return nil, nil
	}

	// Create job
	job := &BackupJob{
		ID:          uuid.New().String(),
		Type:        "restore",
		State:       JobStatePending,
		SourceType:  plan.SourceType,
		RestoreType: plan.RestoreType,
		RestorePath: plan.TargetPath,
		StartedAt:   time.Now(),
	}

	// Add to job manager
	r.jobManager.AddJob(job)

	// Execute restore in background
	go r.runRestore(job, plan)

	return job, nil
}

// ListRestorePoints returns available restore points
func (r *Restorer) ListRestorePoints() ([]RestorePoint, error) {
	var points []RestorePoint

	// Add local snapshots
	snapshots := r.scheduler.ListSnapshots()
	for _, snap := range snapshots {
		points = append(points, RestorePoint{
			ID:        snap.ID,
			Type:      "local",
			Subvolume: snap.Subvolume,
			Timestamp: snap.CreatedAt,
			Source:    "local",
			Path:      snap.Path,
		})
	}

	// Add remote destinations as potential sources
	destinations := r.replicator.ListDestinations()
	for _, dest := range destinations {
		if !dest.Enabled {
			continue
		}

		// For each destination, we would list available snapshots
		// This would require querying the remote destination
		// For now, we'll add a placeholder
		points = append(points, RestorePoint{
			ID:        dest.ID,
			Type:      dest.Type,
			Source:    dest.Name,
			Timestamp: time.Now(), // Would be actual snapshot time
		})
	}

	return points, nil
}

// Private methods

func (r *Restorer) runRestore(job *BackupJob, plan *RestorePlan) {
	// Update job state
	job.State = JobStateRunning
	r.jobManager.UpdateJob(job)

	// Execute each action
	for i, action := range plan.Actions {
		// Update progress
		job.Progress = (i * 100) / len(plan.Actions)
		r.jobManager.UpdateJob(job)

		// Log action
		r.jobManager.AddLogEntry(job.ID, "info", fmt.Sprintf("Executing: %s", action.Description))

		// Execute action
		var err error
		switch action.Type {
		case "snapshot":
			err = r.createSafetySnapshot(action.Target)
		case "stop_service":
			err = r.stopService(action.Target)
		case "start_service":
			err = r.startService(action.Target)
		case "rollback":
			err = r.rollbackSubvolume(plan, action.Target)
		case "mount":
			err = r.mountSnapshot(action.Target)
		case "copy":
			err = r.copyFiles(plan, action.Target)
		case "unmount":
			err = r.unmountSnapshot(action.Target)
		default:
			err = fmt.Errorf("unknown action type: %s", action.Type)
		}

		if err != nil {
			job.State = JobStateFailed
			job.Error = fmt.Sprintf("Action failed: %v", err)
			r.jobManager.AddLogEntry(job.ID, "error", job.Error)
			now := time.Now()
			job.FinishedAt = &now
			r.jobManager.UpdateJob(job)
			return
		}
	}

	// Mark as succeeded
	job.State = JobStateSucceeded
	job.Progress = 100
	now := time.Now()
	job.FinishedAt = &now
	r.jobManager.UpdateJob(job)

	r.jobManager.AddLogEntry(job.ID, "info", "Restore completed successfully")
	r.logger.Info().Str("job", job.ID).Msg("Restore completed")
}

func (r *Restorer) getAffectedServices(targetPath string) []string {
	var services []string

	// Map of paths to services that use them
	pathServices := map[string][]string{
		"/":         {"nosd", "nos-agent", "caddy"},
		"/home":     {},
		"/var":      {"nosd", "nos-agent"},
		"/var/log":  {"rsyslog"},
		"/srv/apps": {"docker"},
	}

	// Find matching services
	for path, svcs := range pathServices {
		if strings.HasPrefix(targetPath, path) {
			services = append(services, svcs...)
		}
	}

	// Remove duplicates
	seen := make(map[string]bool)
	unique := []string{}
	for _, svc := range services {
		if !seen[svc] {
			seen[svc] = true
			unique = append(unique, svc)
		}
	}

	return unique
}

func (r *Restorer) createSafetySnapshot(targetPath string) error {
	// Generate snapshot name
	timestamp := time.Now().Format("20060102-150405")
	snapshotPath := fmt.Sprintf("@snapshots/restore-safety/%s-%s", filepath.Base(targetPath), timestamp)

	// Create snapshot via agent
	return r.agentClient.CreateSnapshot(targetPath, snapshotPath, true)
}

func (r *Restorer) stopService(service string) error {
	cmd := exec.Command("systemctl", "stop", service)
	return cmd.Run()
}

func (r *Restorer) startService(service string) error {
	cmd := exec.Command("systemctl", "start", service)
	return cmd.Run()
}

func (r *Restorer) rollbackSubvolume(plan *RestorePlan, targetPath string) error {
	switch plan.SourceType {
	case "local":
		// Get snapshot
		snapshots := r.scheduler.ListSnapshots()
		var snapshot *Snapshot
		for _, s := range snapshots {
			if s.ID == plan.SourceID {
				snapshot = s
				break
			}
		}
		if snapshot == nil {
			return fmt.Errorf("snapshot not found")
		}

		// Perform atomic subvolume replacement
		// 1. Move current subvolume to backup
		backupPath := targetPath + ".backup." + time.Now().Format("20060102-150405")
		if err := exec.Command("mv", targetPath, backupPath).Run(); err != nil {
			return fmt.Errorf("failed to move current subvolume: %w", err)
		}

		// 2. Create new subvolume from snapshot
		if err := exec.Command("btrfs", "subvolume", "snapshot", snapshot.Path, targetPath).Run(); err != nil {
			// Rollback on failure
			_ = exec.Command("mv", backupPath, targetPath).Run()
			return fmt.Errorf("failed to create subvolume from snapshot: %w", err)
		}

		// 3. Delete backup after successful restore
		go func() {
			time.Sleep(5 * time.Minute)
			_ = exec.Command("btrfs", "subvolume", "delete", backupPath).Run()
		}()

		return nil

	case "ssh":
		// Restore from SSH requires receiving the snapshot
		parts := strings.SplitN(plan.SourceID, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid source ID")
		}

		_, err := r.replicator.GetDestination(parts[0]) // dest will be used for SSH restore
		if err != nil {
			return err
		}

		// Build SSH receive command
		// This would be the inverse of replication
		// Implementation would be similar to replicateSSH but in reverse

		return fmt.Errorf("SSH restore not yet implemented")

	default:
		return fmt.Errorf("unsupported source type for rollback")
	}
}

func (r *Restorer) mountSnapshot(snapshotPath string) error {
	// Create mount point
	mountPoint := fmt.Sprintf("/tmp/restore-mount-%s", uuid.New().String())
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		return err
	}

	// Mount snapshot read-only
	return exec.Command("mount", "-o", "ro,subvol="+snapshotPath, "/dev/mapper/nos-root", mountPoint).Run()
}

func (r *Restorer) copyFiles(plan *RestorePlan, targetPath string) error {
	// Get mount point from previous mount action
	mountPoint := "/tmp/restore-mount-*"

	// Use rsync to copy files preserving attributes
	cmd := exec.Command("rsync", "-avHAX", "--progress", mountPoint+"/", targetPath+"/")
	return cmd.Run()
}

func (r *Restorer) unmountSnapshot(snapshotPath string) error {
	// Find and unmount
	mountPoint := "/tmp/restore-mount-*"
	cmd := exec.Command("umount", mountPoint)
	if err := cmd.Run(); err != nil {
		return err
	}

	// Clean up mount point
	return os.RemoveAll(mountPoint)
}

// RestorePoint represents an available restore point
type RestorePoint struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // "local", "ssh", "rclone"
	Subvolume string    `json:"subvolume"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"` // Source name (local, destination name)
	Path      string    `json:"path"`
}
