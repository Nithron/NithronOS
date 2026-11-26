// Package updates runs system updates for nosd: check, apply (with a
// snapshot of nOS's configuration first), progress, and rollback of that
// configuration.
//
// Everything privileged goes through nos-agent. The upgrade itself runs as
// the systemd unit nos-update (see the agent), so it survives nosd and
// nos-agent being restarted by the packages it upgrades; the progress is kept
// in /var/lib/nos/updates/state.json and picked up again when nosd starts.
package updates

import "time"

// Channel is the release channel.
type Channel string

const (
	ChannelStable Channel = "stable"
	ChannelBeta   Channel = "beta"
)

// State of the current or last update.
type State string

const (
	StateIdle        State = "idle"
	StateApplying    State = "applying"
	StateVerifying   State = "verifying"
	StateSuccess     State = "success"
	StateFailed      State = "failed"
	StateRollingBack State = "rolling_back"
	StateRolledBack  State = "rolled_back"
)

// Phase of an update.
type Phase string

const (
	PhasePreflight  Phase = "preflight"
	PhaseSnapshot   Phase = "snapshot"
	PhaseInstall    Phase = "install"
	PhasePostflight Phase = "postflight"
)

// SystemVersion is what is installed.
type SystemVersion struct {
	OSVersion    string  `json:"os_version"`
	Kernel       string  `json:"kernel"`
	NosdVersion  string  `json:"nosd_version"`
	AgentVersion string  `json:"agent_version"`
	WebUIVersion string  `json:"webui_version"`
	Channel      Channel `json:"channel"`
}

// Package is one upgradable package.
type Package struct {
	Name           string `json:"name"`
	CurrentVersion string `json:"current_version"`
	NewVersion     string `json:"new_version"`
}

// AvailableUpdate is the set of upgradable packages.
type AvailableUpdate struct {
	Version        string    `json:"version"` // the new nosd version, or "system"
	Channel        Channel   `json:"channel"`
	ReleaseDate    time.Time `json:"release_date"`
	Size           int64     `json:"size"`
	Packages       []Package `json:"packages"`
	Critical       bool      `json:"critical"`
	RequiresReboot bool      `json:"requires_reboot"`
}

// CheckResponse answers "is there an update?".
type CheckResponse struct {
	UpdateAvailable bool             `json:"update_available"`
	CurrentVersion  SystemVersion    `json:"current_version"`
	LatestVersion   *AvailableUpdate `json:"latest_version,omitempty"`
	LastCheck       time.Time        `json:"last_check"`
}

// LogEntry is one line of the update log.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"` // info, warn, error
	Message   string    `json:"message"`
	Phase     Phase     `json:"phase,omitempty"`
}

// Progress is the current or last update.
type Progress struct {
	State       State      `json:"state"`
	Phase       Phase      `json:"phase,omitempty"`
	Progress    int        `json:"progress"` // 0-100
	Message     string     `json:"message"`
	StartedAt   time.Time  `json:"started_at,omitzero"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Logs        []LogEntry `json:"logs"`
	Error       string     `json:"error,omitempty"`
	SnapshotID  string     `json:"snapshot_id,omitempty"`
	// RebootRequired is set when the upgraded packages need a reboot.
	RebootRequired bool `json:"reboot_required,omitempty"`
}

// Snapshot is a pre-update snapshot of nOS's configuration.
type Snapshot struct {
	ID          string    `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	Reason      string    `json:"reason"` // update
	Size        int64     `json:"size"`
	Subvolumes  []string  `json:"subvolumes"` // the paths it covers
	CanRollback bool      `json:"can_rollback"`
	Description string    `json:"description,omitempty"`
}

// ApplyRequest starts an update.
type ApplyRequest struct {
	Packages     []string `json:"packages,omitempty"` // empty: everything upgradable
	SkipSnapshot bool     `json:"skip_snapshot"`
	Force        bool     `json:"force"` // continue when the snapshot fails
}
