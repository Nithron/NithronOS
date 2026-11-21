package updates

import (
	"time"
)

// Channel represents an update channel
type Channel string

const (
	ChannelStable Channel = "stable"
	ChannelBeta   Channel = "beta"
)

// UpdateState represents the current state of an update operation
type UpdateState string

const (
	UpdateStateIdle        UpdateState = "idle"
	UpdateStateChecking    UpdateState = "checking"
	UpdateStateDownloading UpdateState = "downloading"
	UpdateStateApplying    UpdateState = "applying"
	UpdateStateVerifying   UpdateState = "verifying"
	UpdateStateSuccess     UpdateState = "success"
	UpdateStateFailed      UpdateState = "failed"
	UpdateStateRollingBack UpdateState = "rolling_back"
	UpdateStateRolledBack  UpdateState = "rolled_back"
)

// UpdatePhase represents a phase in the update process
type UpdatePhase string

const (
	UpdatePhasePreflight  UpdatePhase = "preflight"
	UpdatePhaseSnapshot   UpdatePhase = "snapshot"
	UpdatePhaseDownload   UpdatePhase = "download"
	UpdatePhaseInstall    UpdatePhase = "install"
	UpdatePhasePostflight UpdatePhase = "postflight"
	UpdatePhaseCleanup    UpdatePhase = "cleanup"
)

// SystemVersion represents the current system version information
type SystemVersion struct {
	OSVersion    string    `json:"os_version"`
	Kernel       string    `json:"kernel"`
	NosdVersion  string    `json:"nosd_version"`
	AgentVersion string    `json:"agent_version"`
	WebUIVersion string    `json:"webui_version"`
	Channel      Channel   `json:"channel"`
	Commit       string    `json:"commit,omitempty"`
	BuildDate    time.Time `json:"build_date,omitempty"`
}

// AvailableUpdate represents an available system update
type AvailableUpdate struct {
	Version        string    `json:"version"`
	Channel        Channel   `json:"channel"`
	ReleaseDate    time.Time `json:"release_date"`
	Size           int64     `json:"size"`
	ChangelogURL   string    `json:"changelog_url,omitempty"`
	Packages       []Package `json:"packages"`
	Critical       bool      `json:"critical"`
	RequiresReboot bool      `json:"requires_reboot"`
}

// Package represents a package in an update
type Package struct {
	Name           string `json:"name"`
	CurrentVersion string `json:"current_version"`
	NewVersion     string `json:"new_version"`
	Size           int64  `json:"size"`
	Signature      string `json:"signature,omitempty"`
}

// UpdateProgress represents the progress of an ongoing update
type UpdateProgress struct {
	State                  UpdateState    `json:"state"`
	Phase                  UpdatePhase    `json:"phase"`
	Progress               int            `json:"progress"` // 0-100
	Message                string         `json:"message"`
	StartedAt              time.Time      `json:"started_at"`
	CompletedAt            *time.Time     `json:"completed_at,omitempty"`
	EstimatedTimeRemaining *time.Duration `json:"estimated_time_remaining,omitempty"`
	Logs                   []LogEntry     `json:"logs"`
	Error                  string         `json:"error,omitempty"`
	SnapshotID             string         `json:"snapshot_id,omitempty"`
}

// LogEntry represents a log entry during update
type LogEntry struct {
	Timestamp time.Time   `json:"timestamp"`
	Level     string      `json:"level"` // info, warn, error
	Message   string      `json:"message"`
	Phase     UpdatePhase `json:"phase,omitempty"`
}

// UpdateSnapshot represents a system snapshot taken before an update
type UpdateSnapshot struct {
	ID          string        `json:"id"`
	Version     SystemVersion `json:"version"`
	CreatedAt   time.Time     `json:"created_at"`
	Reason      string        `json:"reason"` // update, manual, automatic
	Size        int64         `json:"size"`
	Subvolumes  []string      `json:"subvolumes"`
	CanRollback bool          `json:"can_rollback"`
	Description string        `json:"description,omitempty"`
}

// UpdateConfig represents the update system configuration
type UpdateConfig struct {
	Channel           Channel       `json:"channel"`
	AutoCheck         bool          `json:"auto_check"`
	AutoCheckInterval time.Duration `json:"auto_check_interval"`
	AutoApply         bool          `json:"auto_apply"`
	SnapshotRetention int           `json:"snapshot_retention"` // Number of snapshots to keep
	RepoURL           string        `json:"repo_url"`
	GPGKeyID          string        `json:"gpg_key_id"`
	ProxyURL          string        `json:"proxy_url,omitempty"`
	Telemetry         bool          `json:"telemetry"`
}

// UpdateState represents the persistent state of the update system
type UpdateStateMachine struct {
	CurrentState    UpdateState      `json:"current_state"`
	LastCheck       *time.Time       `json:"last_check,omitempty"`
	LastUpdate      *time.Time       `json:"last_update,omitempty"`
	LastVersion     string           `json:"last_version,omitempty"`
	PendingUpdate   *AvailableUpdate `json:"pending_update,omitempty"`
	CurrentProgress *UpdateProgress  `json:"current_progress,omitempty"`
	Snapshots       []UpdateSnapshot `json:"snapshots"`
	FailureCount    int              `json:"failure_count"`
	LastError       string           `json:"last_error,omitempty"`
}

// RollbackRequest represents a request to rollback to a previous version
type RollbackRequest struct {
	SnapshotID string `json:"snapshot_id" validate:"required"`
	Force      bool   `json:"force"`
}

// UpdateCheckResponse represents the response from checking for updates
type UpdateCheckResponse struct {
	UpdateAvailable bool             `json:"update_available"`
	CurrentVersion  SystemVersion    `json:"current_version"`
	LatestVersion   *AvailableUpdate `json:"latest_version,omitempty"`
	LastCheck       time.Time        `json:"last_check"`
}

// UpdateApplyRequest represents a request to apply an update
type UpdateApplyRequest struct {
	Version      string `json:"version,omitempty"` // If empty, apply latest
	SkipSnapshot bool   `json:"skip_snapshot"`
	Force        bool   `json:"force"`
}

// ChannelChangeRequest represents a request to change the update channel
type ChannelChangeRequest struct {
	Channel Channel `json:"channel" validate:"required,oneof=stable beta"`
}

// PreflightCheck represents the result of a preflight check
type PreflightCheck struct {
	CheckType string `json:"check_type"` // disk_space, network, repo, signature
	Status    string `json:"status"`     // pass, fail, warning
	Message   string `json:"message"`
	Required  bool   `json:"required"` // If true, failure blocks update
}

// PostflightCheck represents the result of a postflight check
type PostflightCheck struct {
	Service  string `json:"service"`
	Status   string `json:"status"` // running, stopped, degraded
	Healthy  bool   `json:"healthy"`
	Message  string `json:"message"`
	Critical bool   `json:"critical"` // If true, triggers rollback
}

// TelemetryData represents anonymous telemetry data
type TelemetryData struct {
	InstallID      string        `json:"install_id"`
	Version        SystemVersion `json:"version"`
	UpdateSuccess  bool          `json:"update_success"`
	UpdateDuration time.Duration `json:"update_duration"`
	RollbackCount  int           `json:"rollback_count"`
	ErrorCode      string        `json:"error_code,omitempty"`
	Timestamp      time.Time     `json:"timestamp"`
}
