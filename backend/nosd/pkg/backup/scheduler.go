package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"
)

// Scheduler manages backup schedules and retention
type Scheduler struct {
	logger      zerolog.Logger
	stateFile   string
	schedules   map[string]*Schedule
	snapshots   map[string][]*Snapshot
	cron        *cron.Cron
	cronEntries map[string]cron.EntryID
	mu          sync.RWMutex
	agentClient AgentClient
	jobManager  *JobManager
}

// AgentClient is what backups need from nos-agent (nosd can't snapshot,
// send or roll back Btrfs subvolumes itself). There are no pre/post hooks:
// they would run any shell command as root.
type AgentClient interface {
	// CreateSnapshot makes a read-only snapshot of subvolume in its
	// .snapshots folder; the agent names it <time>-<tag>.
	CreateSnapshot(subvolume, tag string) (SnapshotInfo, error)
	DeleteSnapshot(snap *Snapshot) error
	RollbackSnapshot(snap *Snapshot, stopServices []string) error
	Send(req SendRequest) error
	SendStatus(job string) (SendStatus, error)
}

// SnapshotInfo is a snapshot the agent made.
type SnapshotInfo struct {
	ID        string // the agent's name for it
	Path      string
	Subvolume string
	CreatedAt time.Time
}

// SendRequest replicates one snapshot (nos-agent /v1/backup/send).
type SendRequest struct {
	Job      string `json:"job"`
	Snapshot string `json:"snapshot"`
	Parent   string `json:"parent,omitempty"`
	Target   string `json:"target"` // ssh, local
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	User     string `json:"user,omitempty"`
	Path     string `json:"path"`
	Key      string `json:"key,omitempty"`
}

// SendStatus is the state of a replication (nos-agent /v1/backup/status).
type SendStatus struct {
	State      string   `json:"state"` // none, running, succeeded, failed
	ExitStatus int      `json:"exit_status"`
	Log        []string `json:"log"`
}

// NewScheduler creates a new backup scheduler
func NewScheduler(logger zerolog.Logger, stateFile string, agentClient AgentClient) *Scheduler {
	return &Scheduler{
		logger:      logger.With().Str("component", "backup-scheduler").Logger(),
		stateFile:   stateFile,
		schedules:   make(map[string]*Schedule),
		snapshots:   make(map[string][]*Snapshot),
		cron:        cron.New(cron.WithSeconds()),
		cronEntries: make(map[string]cron.EntryID),
		agentClient: agentClient,
		jobManager:  NewJobManager(logger),
	}
}

// Start begins the scheduler
func (s *Scheduler) Start(ctx context.Context) error {
	s.logger.Info().Msg("Starting backup scheduler")

	// Load state
	if err := s.loadState(); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to load scheduler state")
	}

	// Start cron scheduler
	s.cron.Start()

	// Schedule all enabled schedules
	s.mu.RLock()
	for _, schedule := range s.schedules {
		if schedule.Enabled {
			if err := s.scheduleJob(schedule); err != nil {
				s.logger.Error().Err(err).Str("schedule", schedule.ID).Msg("Failed to schedule job")
			}
		}
	}
	s.mu.RUnlock()

	// Start retention cleanup goroutine
	go s.retentionLoop(ctx)

	return nil
}

// Stop stops the scheduler
func (s *Scheduler) Stop() error {
	s.logger.Info().Msg("Stopping backup scheduler")

	// Stop cron
	ctx := s.cron.Stop()
	<-ctx.Done()

	// Save state
	return s.saveState()
}

// CreateSchedule creates a new backup schedule
func (s *Scheduler) CreateSchedule(schedule *Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate ID if not provided
	if schedule.ID == "" {
		schedule.ID = uuid.New().String()
	}

	// Set timestamps
	now := time.Now()
	schedule.CreatedAt = now
	schedule.UpdatedAt = now

	// Validate schedule
	if err := s.validateSchedule(schedule); err != nil {
		return fmt.Errorf("invalid schedule: %w", err)
	}

	// Calculate next run time
	nextRun := s.calculateNextRun(schedule)
	schedule.NextRun = &nextRun

	// Store schedule
	s.schedules[schedule.ID] = schedule

	// Schedule if enabled
	if schedule.Enabled {
		if err := s.scheduleJob(schedule); err != nil {
			return fmt.Errorf("failed to schedule job: %w", err)
		}
	}

	// Save state
	if err := s.saveStateLocked(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	s.logger.Info().Str("id", schedule.ID).Str("name", schedule.Name).Msg("Created backup schedule")
	return nil
}

// UpdateSchedule updates an existing schedule
func (s *Scheduler) UpdateSchedule(id string, update *Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.schedules[id]
	if !ok {
		return fmt.Errorf("schedule not found: %s", id)
	}

	// Preserve immutable fields
	update.ID = existing.ID
	update.CreatedAt = existing.CreatedAt
	update.UpdatedAt = time.Now()

	// Validate
	if err := s.validateSchedule(update); err != nil {
		return fmt.Errorf("invalid schedule: %w", err)
	}

	// Remove old cron entry
	if entryID, ok := s.cronEntries[id]; ok {
		s.cron.Remove(entryID)
		delete(s.cronEntries, id)
	}

	// Calculate next run
	nextRun := s.calculateNextRun(update)
	update.NextRun = &nextRun

	// Update schedule
	s.schedules[id] = update

	// Reschedule if enabled
	if update.Enabled {
		if err := s.scheduleJob(update); err != nil {
			return fmt.Errorf("failed to schedule job: %w", err)
		}
	}

	// Save state
	if err := s.saveStateLocked(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	s.logger.Info().Str("id", id).Msg("Updated backup schedule")
	return nil
}

// DeleteSchedule deletes a schedule
func (s *Scheduler) DeleteSchedule(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.schedules[id]; !ok {
		return fmt.Errorf("schedule not found: %s", id)
	}

	// Remove cron entry
	if entryID, ok := s.cronEntries[id]; ok {
		s.cron.Remove(entryID)
		delete(s.cronEntries, id)
	}

	// Delete schedule
	delete(s.schedules, id)

	// Save state
	if err := s.saveStateLocked(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	s.logger.Info().Str("id", id).Msg("Deleted backup schedule")
	return nil
}

// GetSchedule returns a schedule by ID
func (s *Scheduler) GetSchedule(id string) (*Schedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	schedule, ok := s.schedules[id]
	if !ok {
		return nil, fmt.Errorf("schedule not found: %s", id)
	}

	return schedule, nil
}

// ListSchedules returns all schedules
func (s *Scheduler) ListSchedules() []*Schedule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	schedules := make([]*Schedule, 0, len(s.schedules))
	for _, schedule := range s.schedules {
		schedules = append(schedules, schedule)
	}

	// Sort by name
	sort.Slice(schedules, func(i, j int) bool {
		return schedules[i].Name < schedules[j].Name
	})

	return schedules
}

// CreateSnapshot creates a manual snapshot
func (s *Scheduler) CreateSnapshot(subvolumes []string, tag string) (*BackupJob, error) {
	job := &BackupJob{
		ID:         uuid.New().String(),
		Type:       "snapshot",
		State:      JobStatePending,
		Subvolumes: subvolumes,
		StartedAt:  time.Now(),
	}

	// Add to job manager
	s.jobManager.AddJob(job)

	// Run snapshot creation in background
	go s.runSnapshotJob(job, subvolumes, tag, nil)

	return job, nil
}

// DeleteSnapshot deletes a snapshot
func (s *Scheduler) DeleteSnapshot(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find snapshot
	var snapshot *Snapshot
	for _, snapshots := range s.snapshots {
		for _, snap := range snapshots {
			if snap.ID == id {
				snapshot = snap
				break
			}
		}
		if snapshot != nil {
			break
		}
	}

	if snapshot == nil {
		return fmt.Errorf("snapshot not found: %s", id)
	}

	// Delete via agent
	if err := s.agentClient.DeleteSnapshot(snapshot); err != nil {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}

	// Remove from state
	s.removeSnapshot(snapshot)

	// Save state
	if err := s.saveStateLocked(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	s.logger.Info().Str("id", id).Str("path", snapshot.Path).Msg("Deleted snapshot")
	return nil
}

// ListSnapshots returns all snapshots
func (s *Scheduler) ListSnapshots() []*Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var snapshots []*Snapshot
	for _, subvolSnapshots := range s.snapshots {
		snapshots = append(snapshots, subvolSnapshots...)
	}

	// Sort by creation time (newest first)
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.After(snapshots[j].CreatedAt)
	})

	return snapshots
}

// GetJobManager returns the job manager
func (s *Scheduler) GetJobManager() *JobManager {
	return s.jobManager
}

// GetSnapshotStats returns snapshot statistics
func (s *Scheduler) GetSnapshotStats() *SnapshotStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &SnapshotStats{
		BySubvolume: make(map[string]SubvolumeStats),
	}

	var oldest, newest time.Time

	for subvol, snapshots := range s.snapshots {
		subStats := SubvolumeStats{}

		for _, snap := range snapshots {
			stats.TotalCount++
			subStats.Count++

			stats.TotalSizeBytes += snap.SizeBytes
			subStats.SizeBytes += snap.SizeBytes

			if subStats.LastBackup.IsZero() || snap.CreatedAt.After(subStats.LastBackup) {
				subStats.LastBackup = snap.CreatedAt
			}

			if oldest.IsZero() || snap.CreatedAt.Before(oldest) {
				oldest = snap.CreatedAt
			}

			if newest.IsZero() || snap.CreatedAt.After(newest) {
				newest = snap.CreatedAt
			}
		}

		stats.BySubvolume[subvol] = subStats
	}

	stats.OldestSnapshot = oldest
	stats.NewestSnapshot = newest

	return stats
}

// Private methods

func (s *Scheduler) validateSchedule(schedule *Schedule) error {
	if schedule.Name == "" {
		return fmt.Errorf("schedule name is required")
	}

	if len(schedule.Subvolumes) == 0 {
		return fmt.Errorf("at least one subvolume is required")
	}
	if len(schedule.PreHooks) > 0 || len(schedule.PostHooks) > 0 {
		return fmt.Errorf("pre- and post-hooks are not supported")
	}

	// Validate frequency
	switch schedule.Frequency.Type {
	case "cron":
		if schedule.Frequency.Cron == "" {
			return fmt.Errorf("cron expression is required")
		}
		// Validate cron expression
		if _, err := cron.ParseStandard(schedule.Frequency.Cron); err != nil {
			return fmt.Errorf("invalid cron expression: %w", err)
		}
	case "hourly", "daily", "weekly", "monthly":
		// These are valid
	default:
		return fmt.Errorf("invalid frequency type: %s", schedule.Frequency.Type)
	}

	// Validate retention
	if schedule.Retention.MinKeep < 0 {
		return fmt.Errorf("min_keep cannot be negative")
	}

	return nil
}

func (s *Scheduler) scheduleJob(schedule *Schedule) error {
	// Build cron expression
	var cronExpr string
	switch schedule.Frequency.Type {
	case "cron":
		cronExpr = schedule.Frequency.Cron
	case "hourly":
		cronExpr = fmt.Sprintf("%d * * * *", schedule.Frequency.Minute)
	case "daily":
		cronExpr = fmt.Sprintf("%d %d * * *", schedule.Frequency.Minute, schedule.Frequency.Hour)
	case "weekly":
		cronExpr = fmt.Sprintf("%d %d * * %d", schedule.Frequency.Minute, schedule.Frequency.Hour, schedule.Frequency.Weekday)
	case "monthly":
		cronExpr = fmt.Sprintf("%d %d %d * *", schedule.Frequency.Minute, schedule.Frequency.Hour, schedule.Frequency.Day)
	default:
		return fmt.Errorf("unsupported frequency type: %s", schedule.Frequency.Type)
	}

	// Add cron job
	entryID, err := s.cron.AddFunc(cronExpr, func() {
		s.runScheduledBackup(schedule.ID)
	})
	if err != nil {
		return err
	}

	s.cronEntries[schedule.ID] = entryID
	return nil
}

func (s *Scheduler) calculateNextRun(schedule *Schedule) time.Time {
	// Parse cron expression based on frequency
	var cronExpr string
	switch schedule.Frequency.Type {
	case "cron":
		cronExpr = schedule.Frequency.Cron
	case "hourly":
		cronExpr = fmt.Sprintf("%d * * * *", schedule.Frequency.Minute)
	case "daily":
		cronExpr = fmt.Sprintf("%d %d * * *", schedule.Frequency.Minute, schedule.Frequency.Hour)
	case "weekly":
		cronExpr = fmt.Sprintf("%d %d * * %d", schedule.Frequency.Minute, schedule.Frequency.Hour, schedule.Frequency.Weekday)
	case "monthly":
		cronExpr = fmt.Sprintf("%d %d %d * *", schedule.Frequency.Minute, schedule.Frequency.Hour, schedule.Frequency.Day)
	}

	// Parse and calculate next run
	if sched, err := cron.ParseStandard(cronExpr); err == nil {
		return sched.Next(time.Now())
	}

	// Fallback to 1 hour from now
	return time.Now().Add(time.Hour)
}

func (s *Scheduler) runScheduledBackup(scheduleID string) {
	s.mu.RLock()
	schedule, ok := s.schedules[scheduleID]
	s.mu.RUnlock()

	if !ok {
		s.logger.Error().Str("schedule_id", scheduleID).Msg("Schedule not found")
		return
	}

	s.logger.Info().Str("schedule", schedule.Name).Msg("Running scheduled backup")

	// Create job
	job := &BackupJob{
		ID:         uuid.New().String(),
		Type:       "snapshot",
		State:      JobStatePending,
		ScheduleID: scheduleID,
		Subvolumes: schedule.Subvolumes,
		StartedAt:  time.Now(),
	}

	// Add to job manager
	s.jobManager.AddJob(job)

	// Run backup
	s.runSnapshotJob(job, schedule.Subvolumes, "", schedule)

	// Update schedule
	s.mu.Lock()
	now := time.Now()
	schedule.LastRun = &now
	nextRun := s.calculateNextRun(schedule)
	schedule.NextRun = &nextRun
	s.mu.Unlock()

	// Save state
	_ = s.saveState()
}

func (s *Scheduler) runSnapshotJob(job *BackupJob, subvolumes []string, tag string, schedule *Schedule) {
	// Update job state
	job.State = JobStateRunning
	s.jobManager.UpdateJob(job)

	// Create snapshots
	var createdSnapshots []*Snapshot
	for _, subvol := range subvolumes {
		// Create the snapshot via the agent (in the subvolume's .snapshots)
		info, err := s.agentClient.CreateSnapshot(subvol, tag)
		if err != nil {
			s.logger.Error().Err(err).Str("subvolume", subvol).Msg("Failed to create snapshot")
			job.State = JobStateFailed
			job.Error = fmt.Sprintf("failed to create snapshot for %s: %v", subvol, err)
			now := time.Now()
			job.FinishedAt = &now
			s.jobManager.UpdateJob(job)
			return
		}

		// Create snapshot record
		snapshot := &Snapshot{
			ID:        uuid.New().String(),
			AgentID:   info.ID,
			Subvolume: subvol,
			Path:      info.Path,
			CreatedAt: info.CreatedAt,
			ReadOnly:  true,
			Tags:      []string{},
		}

		if tag != "" {
			snapshot.Tags = append(snapshot.Tags, tag)
		}

		if schedule != nil {
			snapshot.ScheduleID = schedule.ID
		}

		createdSnapshots = append(createdSnapshots, snapshot)

		// Add to state
		s.mu.Lock()
		s.snapshots[subvol] = append(s.snapshots[subvol], snapshot)
		s.mu.Unlock()

		// Update progress
		job.Progress = (len(createdSnapshots) * 100) / len(subvolumes)
		s.jobManager.UpdateJob(job)
	}

	// Apply retention if this was a scheduled backup
	if schedule != nil {
		s.applyRetention(schedule)
	}

	// Mark job as succeeded
	job.State = JobStateSucceeded
	job.Progress = 100
	now := time.Now()
	job.FinishedAt = &now
	s.jobManager.UpdateJob(job)

	// Save state
	_ = s.saveState()

	s.logger.Info().Int("count", len(createdSnapshots)).Msg("Snapshots created successfully")
}

func (s *Scheduler) applyRetention(schedule *Schedule) {
	s.mu.Lock()
	defer s.mu.Unlock()

	retention := schedule.Retention

	for _, subvol := range schedule.Subvolumes {
		snapshots := s.snapshots[subvol]

		// Filter snapshots created by this schedule
		var scheduleSnapshots []*Snapshot
		for _, snap := range snapshots {
			if snap.ScheduleID == schedule.ID {
				scheduleSnapshots = append(scheduleSnapshots, snap)
			}
		}

		// Sort by age (newest first)
		sort.Slice(scheduleSnapshots, func(i, j int) bool {
			return scheduleSnapshots[i].CreatedAt.After(scheduleSnapshots[j].CreatedAt)
		})

		// Apply GFS retention
		toKeep := s.selectGFSSnapshots(scheduleSnapshots, retention)

		// Delete snapshots not in toKeep
		for _, snap := range scheduleSnapshots {
			keep := false
			for _, keepSnap := range toKeep {
				if snap.ID == keepSnap.ID {
					keep = true
					break
				}
			}

			if !keep {
				s.logger.Info().Str("id", snap.ID).Str("path", snap.Path).Msg("Deleting snapshot per retention policy")
				if err := s.agentClient.DeleteSnapshot(snap); err != nil {
					s.logger.Error().Err(err).Str("path", snap.Path).Msg("Failed to delete snapshot")
				} else {
					s.removeSnapshot(snap)
				}
			}
		}
	}
}

func (s *Scheduler) selectGFSSnapshots(snapshots []*Snapshot, retention RetentionPolicy) []*Snapshot {
	if len(snapshots) == 0 {
		return snapshots
	}

	// Always keep minimum number
	if len(snapshots) <= retention.MinKeep {
		return snapshots
	}

	toKeep := make(map[string]*Snapshot)
	now := time.Now()

	// Keep daily snapshots
	for i := 0; i < retention.Days && i < len(snapshots); i++ {
		age := now.Sub(snapshots[i].CreatedAt)
		if age < time.Duration(retention.Days)*24*time.Hour {
			toKeep[snapshots[i].ID] = snapshots[i]
		}
	}

	// Keep weekly snapshots
	weeklyCount := 0
	for _, snap := range snapshots {
		age := now.Sub(snap.CreatedAt)
		if age < time.Duration(retention.Weeks)*7*24*time.Hour {
			// Keep one per week
			week := snap.CreatedAt.Format("2006-W01")
			if _, exists := toKeep["weekly-"+week]; !exists && weeklyCount < retention.Weeks {
				toKeep["weekly-"+week] = snap
				toKeep[snap.ID] = snap
				weeklyCount++
			}
		}
	}

	// Keep monthly snapshots
	monthlyCount := 0
	for _, snap := range snapshots {
		age := now.Sub(snap.CreatedAt)
		if age < time.Duration(retention.Months)*30*24*time.Hour {
			// Keep one per month
			month := snap.CreatedAt.Format("2006-01")
			if _, exists := toKeep["monthly-"+month]; !exists && monthlyCount < retention.Months {
				toKeep["monthly-"+month] = snap
				toKeep[snap.ID] = snap
				monthlyCount++
			}
		}
	}

	// Keep yearly snapshots
	yearlyCount := 0
	for _, snap := range snapshots {
		age := now.Sub(snap.CreatedAt)
		if age < time.Duration(retention.Years)*365*24*time.Hour {
			// Keep one per year
			year := snap.CreatedAt.Format("2006")
			if _, exists := toKeep["yearly-"+year]; !exists && yearlyCount < retention.Years {
				toKeep["yearly-"+year] = snap
				toKeep[snap.ID] = snap
				yearlyCount++
			}
		}
	}

	// Ensure we keep at least MinKeep
	if len(toKeep) < retention.MinKeep {
		for i := 0; i < retention.MinKeep && i < len(snapshots); i++ {
			toKeep[snapshots[i].ID] = snapshots[i]
		}
	}

	// Convert map to slice
	result := make([]*Snapshot, 0, len(toKeep))
	for _, snap := range toKeep {
		result = append(result, snap)
	}

	return result
}

func (s *Scheduler) removeSnapshot(snapshot *Snapshot) {
	snapshots := s.snapshots[snapshot.Subvolume]
	for i, snap := range snapshots {
		if snap.ID == snapshot.ID {
			s.snapshots[snapshot.Subvolume] = append(snapshots[:i], snapshots[i+1:]...)
			break
		}
	}
}

func (s *Scheduler) retentionLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.logger.Debug().Msg("Running retention cleanup")

			s.mu.RLock()
			schedules := make([]*Schedule, 0, len(s.schedules))
			for _, schedule := range s.schedules {
				schedules = append(schedules, schedule)
			}
			s.mu.RUnlock()

			for _, schedule := range schedules {
				if schedule.Enabled {
					s.applyRetention(schedule)
				}
			}

			_ = s.saveState()
		}
	}
}

func (s *Scheduler) loadState() error {
	data, err := os.ReadFile(s.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var state struct {
		Schedules map[string]*Schedule   `json:"schedules"`
		Snapshots map[string][]*Snapshot `json:"snapshots"`
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	s.schedules = state.Schedules
	s.snapshots = state.Snapshots

	if s.schedules == nil {
		s.schedules = make(map[string]*Schedule)
	}
	if s.snapshots == nil {
		s.snapshots = make(map[string][]*Snapshot)
	}

	return nil
}

// saveState writes the state file; the caller must not hold mu.
func (s *Scheduler) saveState() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.saveStateLocked()
}

// saveStateLocked writes the state file; the caller holds mu. (saveState
// used to take mu again while callers held it, which deadlocked.)
func (s *Scheduler) saveStateLocked() error {
	state := struct {
		Schedules map[string]*Schedule   `json:"schedules"`
		Snapshots map[string][]*Snapshot `json:"snapshots"`
	}{
		Schedules: s.schedules,
		Snapshots: s.snapshots,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	// Write atomically
	tmpFile := s.stateFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return err
	}

	return os.Rename(tmpFile, s.stateFile)
}
