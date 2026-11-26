package backup

import (
	"time"
)

// ScheduleFrequency defines how often backups run
type ScheduleFrequency struct {
	Type    string `json:"type"` // "cron", "hourly", "daily", "weekly", "monthly"
	Cron    string `json:"cron,omitempty"`
	Hour    int    `json:"hour,omitempty"`    // For daily/weekly/monthly
	Minute  int    `json:"minute,omitempty"`  // For hourly/daily/weekly/monthly
	Day     int    `json:"day,omitempty"`     // For monthly (1-31)
	Weekday int    `json:"weekday,omitempty"` // For weekly (0-6, 0=Sunday)
}

// RetentionPolicy defines GFS-style retention
type RetentionPolicy struct {
	MinKeep int `json:"min_keep"` // Minimum snapshots to keep
	Days    int `json:"days"`     // Daily snapshots to keep
	Weeks   int `json:"weeks"`    // Weekly snapshots to keep
	Months  int `json:"months"`   // Monthly snapshots to keep
	Years   int `json:"years"`    // Yearly snapshots to keep
}

// Schedule represents a backup schedule
type Schedule struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Enabled    bool              `json:"enabled"`
	Subvolumes []string          `json:"subvolumes"`
	Frequency  ScheduleFrequency `json:"frequency"`
	Retention  RetentionPolicy   `json:"retention"`
	PreHooks   []string          `json:"pre_hooks,omitempty"`
	PostHooks  []string          `json:"post_hooks,omitempty"`
	LastRun    *time.Time        `json:"last_run,omitempty"`
	NextRun    *time.Time        `json:"next_run,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// Snapshot represents a Btrfs snapshot
type Snapshot struct {
	ID         string    `json:"id"`
	AgentID    string    `json:"agent_id,omitempty"` // the agent's name (its .snapshots entry)
	Subvolume  string    `json:"subvolume"`
	Path       string    `json:"path"`
	CreatedAt  time.Time `json:"created_at"`
	SizeBytes  int64     `json:"size_bytes,omitempty"`
	ScheduleID string    `json:"schedule_id,omitempty"`
	Tags       []string  `json:"tags,omitempty"`
	ReadOnly   bool      `json:"read_only"`
	Parent     string    `json:"parent,omitempty"` // For incremental backups
}

// Destination represents a backup destination
type Destination struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"` // "ssh", "rclone", "local"
	Enabled bool   `json:"enabled"`

	// SSH specific
	Host   string `json:"host,omitempty"`
	Port   int    `json:"port,omitempty"`
	User   string `json:"user,omitempty"`
	Path   string `json:"path,omitempty"`
	KeyRef string `json:"key_ref,omitempty"`

	// Rclone specific
	RemoteName string `json:"remote_name,omitempty"`
	RemotePath string `json:"remote_path,omitempty"`

	// Common options
	BandwidthLimit int `json:"bandwidth_limit,omitempty"` // KB/s
	Concurrency    int `json:"concurrency,omitempty"`
	RetryCount     int `json:"retry_count,omitempty"`

	LastTest       *time.Time `json:"last_test,omitempty"`
	LastTestStatus string     `json:"last_test_status,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// BackupJob represents a backup/replication job
type BackupJob struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"` // "snapshot", "replicate", "restore"
	State    JobState `json:"state"`
	Progress int      `json:"progress"` // 0-100

	// For snapshot jobs
	ScheduleID string   `json:"schedule_id,omitempty"`
	Subvolumes []string `json:"subvolumes,omitempty"`

	// For replication jobs
	DestinationID string `json:"destination_id,omitempty"`
	SnapshotID    string `json:"snapshot_id,omitempty"`
	Incremental   bool   `json:"incremental,omitempty"`
	BaseSnapshot  string `json:"base_snapshot,omitempty"`

	// For restore jobs
	SourceType  string `json:"source_type,omitempty"`  // "local", "ssh", "rclone"
	RestoreType string `json:"restore_type,omitempty"` // "full", "files"
	RestorePath string `json:"restore_path,omitempty"`

	// Common fields
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Error      string     `json:"error,omitempty"`
	BytesTotal int64      `json:"bytes_total,omitempty"`
	BytesDone  int64      `json:"bytes_done,omitempty"`

	// Logs
	LogEntries []LogEntry `json:"log_entries,omitempty"`
}

// JobState represents the state of a backup job
type JobState string

const (
	JobStatePending   JobState = "pending"
	JobStateRunning   JobState = "running"
	JobStateSucceeded JobState = "succeeded"
	JobStateFailed    JobState = "failed"
	JobStateCanceled  JobState = "canceled"
)

// LogEntry represents a job log entry
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"` // "info", "warn", "error"
	Message   string    `json:"message"`
}

// RestorePlan represents a restore operation plan
type RestorePlan struct {
	SourceType    string          `json:"source_type"`
	SourceID      string          `json:"source_id"`
	RestoreType   string          `json:"restore_type"`
	TargetPath    string          `json:"target_path"`
	RequiresStop  []string        `json:"requires_stop,omitempty"` // Services to stop
	EstimatedTime int             `json:"estimated_time_seconds"`
	DryRun        bool            `json:"dry_run"`
	Actions       []RestoreAction `json:"actions"`
}

// RestoreAction represents a single restore action
type RestoreAction struct {
	Type        string `json:"type"` // "stop_service", "snapshot", "copy", "rollback"
	Target      string `json:"target"`
	Description string `json:"description"`
}

// SnapshotStats provides statistics about snapshots
type SnapshotStats struct {
	TotalCount     int                       `json:"total_count"`
	TotalSizeBytes int64                     `json:"total_size_bytes"`
	BySubvolume    map[string]SubvolumeStats `json:"by_subvolume"`
	OldestSnapshot time.Time                 `json:"oldest_snapshot,omitempty"`
	NewestSnapshot time.Time                 `json:"newest_snapshot,omitempty"`
}

// SubvolumeStats provides per-subvolume statistics
type SubvolumeStats struct {
	Count      int       `json:"count"`
	SizeBytes  int64     `json:"size_bytes"`
	LastBackup time.Time `json:"last_backup,omitempty"`
}
