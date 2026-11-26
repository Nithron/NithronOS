package backup

import (
	"fmt"
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
	_, err := r.agentClient.CreateSnapshot(targetPath, "pre-restore")
	return err
}

// Services are stopped and started by the agent around the rollback.
func (r *Restorer) stopService(service string) error  { return nil }
func (r *Restorer) startService(service string) error { return nil }

// rollbackSubvolume puts a local snapshot back in place of its subvolume
// (nos-agent keeps the current state in .rollback first). Restoring from an
// SSH destination is not supported yet: receive the snapshot on this system
// and restore it from there.
func (r *Restorer) rollbackSubvolume(plan *RestorePlan, targetPath string) error {
	if plan.SourceType != "local" {
		return fmt.Errorf("restoring from a remote destination is not supported; restore a local snapshot")
	}
	for _, snap := range r.scheduler.ListSnapshots() {
		if snap.ID == plan.SourceID {
			if snap.Subvolume != targetPath {
				return fmt.Errorf("snapshot %s is of %s, not %s", snap.ID, snap.Subvolume, targetPath)
			}
			return r.agentClient.RollbackSnapshot(snap, r.getAffectedServices(targetPath))
		}
	}
	return fmt.Errorf("snapshot not found: %s", plan.SourceID)
}

// File-level restore isn't done by nOS: snapshots of a share are in its
// .snapshots folder, so files can be copied back over SMB or NFS.
var errFileRestore = fmt.Errorf("restoring single files is not supported; copy them from the share's .snapshots folder")

func (r *Restorer) mountSnapshot(string) error           { return errFileRestore }
func (r *Restorer) copyFiles(*RestorePlan, string) error { return errFileRestore }
func (r *Restorer) unmountSnapshot(string) error         { return nil }

// RestorePoint represents an available restore point
type RestorePoint struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // "local", "ssh", "rclone"
	Subvolume string    `json:"subvolume"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"` // Source name (local, destination name)
	Path      string    `json:"path"`
}
