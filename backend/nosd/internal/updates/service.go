package updates

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"nos/backend/nosd/internal/fsatomic"
	"nos/backend/nosd/internal/version"
)

// Agent is the part of the nos-agent client the service uses.
type Agent interface {
	GetJSON(ctx context.Context, path string, out any) error
	PostJSON(ctx context.Context, path string, body any, out any) error
}

var (
	ErrBusy     = errors.New("an update or rollback is already running")
	ErrNotFound = errors.New("snapshot not found")
)

// ConfigDir is what the pre-update snapshot covers: nOS's users, shares,
// groups, settings and secrets. Package updates don't touch data pools; a
// whole-system rollback needs a Btrfs root and booting from a snapshot,
// which nOS doesn't do yet.
const ConfigDir = "/etc/nos"

// Record is one update: its snapshot and what it changed.
type Record struct {
	ID        string     `json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	Reason    string     `json:"reason"` // update, rollback
	Packages  []Package  `json:"packages"`
	Targets   []Target   `json:"targets"`
	Success   *bool      `json:"success,omitempty"`
	Finished  *time.Time `json:"finished_at,omitempty"`
	Notes     string     `json:"notes,omitempty"`
}

// Target is one snapshot taken by the agent.
type Target struct {
	Path     string `json:"path"`
	ID       string `json:"id"` // the agent's snapshot id
	Type     string `json:"type"`
	Location string `json:"location"`
}

type persisted struct {
	Progress  Progress       `json:"progress"`
	Packages  []string       `json:"packages"`
	RecordID  string         `json:"record_id,omitempty"`
	LastCheck *CheckResponse `json:"last_check,omitempty"`
	LogsSeen  int            `json:"logs_seen"`
}

// Service runs updates. It is safe for concurrent use.
type Service struct {
	agent       Agent
	dir         string // state.json, records.json
	channelFile string
	pollEvery   time.Duration

	mu      sync.Mutex
	st      persisted
	running bool
	subs    map[chan Progress]struct{}
}

// New loads the state from dir (normally /var/lib/nos/updates) and the
// channel from channelFile (/etc/nos/update/channel).
func New(agent Agent, dir, channelFile string) (*Service, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	s := &Service{agent: agent, dir: dir, channelFile: channelFile, pollEvery: 3 * time.Second, subs: map[chan Progress]struct{}{}}
	if _, err := fsatomic.LoadJSON(s.statePath(), &s.st); err != nil {
		return nil, fmt.Errorf("load %s: %w", s.statePath(), err)
	}
	if s.st.Progress.State == "" {
		s.st.Progress = Progress{State: StateIdle, Logs: []LogEntry{}}
	}
	return s, nil
}

func (s *Service) statePath() string   { return filepath.Join(s.dir, "state.json") }
func (s *Service) recordsPath() string { return filepath.Join(s.dir, "records.json") }

// Resume continues an update that was installing when nosd stopped (the
// upgrade restarts nosd); one interrupted earlier is marked failed.
func (s *Service) Resume() {
	s.mu.Lock()
	p := s.st.Progress
	if p.State != StateApplying && p.State != StateVerifying && p.State != StateRollingBack {
		s.mu.Unlock()
		return
	}
	if p.Phase == PhaseInstall || p.State == StateVerifying {
		s.running = true
		s.mu.Unlock()
		s.log("info", PhaseInstall, "nosd restarted during the update; following it")
		go s.follow(context.Background())
		return
	}
	s.mu.Unlock()
	s.fail("The update was interrupted (nosd stopped) before packages were installed; nothing was changed")
}

// --- version, channel, check -------------------------------------------------

// Version describes what is installed.
func (s *Service) Version() SystemVersion {
	v := SystemVersion{NosdVersion: version.Version, Channel: s.Channel()}
	v.AgentVersion = dpkgVersion("nos-agent")
	v.WebUIVersion = dpkgVersion("nos-web")
	if b, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		v.Kernel = strings.TrimSpace(string(b))
	}
	if b, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if val, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
				v.OSVersion = strings.Trim(val, `"`)
			}
		}
	}
	return v
}

var dpkgVersion = func(pkg string) string {
	out, err := exec.Command("dpkg-query", "-W", "-f=${Version}", pkg).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Channel is the release channel the admin chose (stable by default). nOS
// has no package repository of its own yet, so it is recorded for when it
// does; it doesn't change the apt sources.
func (s *Service) Channel() Channel {
	b, err := os.ReadFile(s.channelFile)
	if err == nil && Channel(strings.TrimSpace(string(b))) == ChannelBeta {
		return ChannelBeta
	}
	return ChannelStable
}

// SetChannel records the channel.
func (s *Service) SetChannel(c Channel) error {
	if c != ChannelStable && c != ChannelBeta {
		return fmt.Errorf("channel must be stable or beta")
	}
	if err := os.MkdirAll(filepath.Dir(s.channelFile), 0o750); err != nil {
		return err
	}
	return os.WriteFile(s.channelFile, []byte(string(c)+"\n"), 0o640)
}

type agentPlan struct {
	Updates []struct {
		Name      string `json:"name"`
		Current   string `json:"current"`
		Candidate string `json:"candidate"`
	} `json:"updates"`
	RebootRequired bool   `json:"reboot_required"`
	Note           string `json:"note"`
}

// Check refreshes the package lists and lists the upgradable packages.
func (s *Service) Check(ctx context.Context) (CheckResponse, error) {
	var plan agentPlan
	if err := s.agent.PostJSON(ctx, "/v1/updates/plan", map[string]any{}, &plan); err != nil {
		return CheckResponse{}, err
	}
	res := CheckResponse{CurrentVersion: s.Version(), LastCheck: time.Now().UTC()}
	if len(plan.Updates) > 0 {
		up := &AvailableUpdate{Version: "system", Channel: s.Channel(), ReleaseDate: res.LastCheck, Packages: []Package{}, RequiresReboot: plan.RebootRequired}
		for _, u := range plan.Updates {
			up.Packages = append(up.Packages, Package{Name: u.Name, CurrentVersion: u.Current, NewVersion: u.Candidate})
			if u.Name == "nosd" {
				up.Version = u.Candidate
			}
			if strings.HasPrefix(u.Name, "linux-image") {
				up.RequiresReboot = true
			}
		}
		res.UpdateAvailable, res.LatestVersion = true, up
	}
	s.mu.Lock()
	s.st.LastCheck = &res
	s.saveLocked()
	s.mu.Unlock()
	return res, nil
}

// --- apply ------------------------------------------------------------------

// Apply starts an update in the background; Progress and Subscribe follow it.
func (s *Service) Apply(req ApplyRequest) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return ErrBusy
	}
	s.running = true
	now := time.Now().UTC()
	s.st.Progress = Progress{State: StateApplying, Phase: PhasePreflight, Progress: 0, Message: "Starting the update", StartedAt: now, Logs: []LogEntry{}}
	s.st.Packages, s.st.RecordID, s.st.LogsSeen = slices.Clone(req.Packages), "", 0
	s.saveLocked()
	s.mu.Unlock()
	go s.run(req)
	return nil
}

func (s *Service) run(req ApplyRequest) {
	ctx := context.Background()
	rec := Record{ID: uuid.NewString(), CreatedAt: time.Now().UTC(), Reason: "update", Packages: s.pendingPackages(req.Packages), Targets: []Target{}}

	s.step(PhasePreflight, 5, "Checking the agent")
	var st agentStatus
	if err := s.agent.GetJSON(ctx, "/v1/updates/status", &st); err != nil {
		s.fail("nos-agent is not reachable: " + err.Error())
		return
	}
	if st.State == "running" {
		s.fail("Another update is already running on this system")
		return
	}

	if !req.SkipSnapshot {
		s.step(PhaseSnapshot, 10, "Saving nOS's configuration ("+ConfigDir+")")
		var snap struct {
			ID, Type, Location string
		}
		err := s.agent.PostJSON(ctx, "/v1/snapshot/create", map[string]any{"path": ConfigDir, "mode": "tar", "reason": "pre-update"}, &snap)
		switch {
		case err == nil:
			rec.Targets = append(rec.Targets, Target{Path: ConfigDir, ID: snap.ID, Type: snap.Type, Location: snap.Location})
			s.log("info", PhaseSnapshot, "Saved "+ConfigDir+" as "+snap.ID)
		case req.Force:
			s.log("warn", PhaseSnapshot, "Snapshot failed, continuing (force): "+err.Error())
		default:
			s.fail("Snapshot of " + ConfigDir + " failed: " + err.Error())
			return
		}
	}
	if err := s.putRecord(rec); err != nil {
		s.log("warn", PhaseSnapshot, "Could not record the snapshot: "+err.Error())
	}
	s.mu.Lock()
	s.st.RecordID = rec.ID
	if len(rec.Targets) > 0 {
		s.st.Progress.SnapshotID = rec.ID
	}
	s.mu.Unlock()

	s.step(PhaseInstall, 20, "Installing packages")
	if err := s.agent.PostJSON(ctx, "/v1/updates/apply", map[string]any{"packages": req.Packages}, nil); err != nil {
		s.fail("Could not start the upgrade: " + err.Error())
		return
	}
	s.follow(ctx)
}

// agentStatus is the agent's view of the nos-update unit.
type agentStatus struct {
	State          string   `json:"state"` // none, running, succeeded, failed
	ExitStatus     int      `json:"exit_status"`
	Log            []string `json:"log"`
	RebootRequired bool     `json:"reboot_required"`
}

// follow polls the agent until the upgrade unit finishes. nosd may be
// restarted meanwhile (Resume calls this again).
func (s *Service) follow(ctx context.Context) {
	start := time.Now()
	for {
		var st agentStatus
		if err := s.agent.GetJSON(ctx, "/v1/updates/status", &st); err != nil {
			// nos-agent restarts when it is upgraded; keep asking.
			if time.Since(start) > 2*time.Hour {
				s.fail("Lost track of the upgrade: " + err.Error())
				return
			}
			time.Sleep(s.pollEvery)
			continue
		}
		s.addAgentLog(st.Log)
		switch st.State {
		case "succeeded":
			s.finish(st.RebootRequired)
			return
		case "failed":
			s.fail(fmt.Sprintf("apt-get failed (exit status %d); see the log. The configuration snapshot can be restored with a rollback.", st.ExitStatus))
			return
		case "none":
			// systemd-run returned before the unit showed up; give it a moment.
			if time.Since(start) > time.Minute {
				s.fail("The upgrade did not start")
				return
			}
		}
		time.Sleep(s.pollEvery)
	}
}

func (s *Service) addAgentLog(lines []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.st.LogsSeen > len(lines) { // a new run, or the journal rotated
		s.st.LogsSeen = 0
	}
	for _, l := range lines[s.st.LogsSeen:] {
		s.st.Progress.Logs = append(s.st.Progress.Logs, LogEntry{Timestamp: time.Now().UTC(), Level: "info", Message: l, Phase: PhaseInstall})
		// apt prints one "Setting up" per package: move the bar with it.
		if strings.HasPrefix(l, "Setting up ") && s.st.Progress.Progress < 90 {
			s.st.Progress.Progress += max(1, 70/max(1, len(s.st.Packages)+5))
		}
	}
	s.st.LogsSeen = len(lines)
	s.trimLogsLocked()
	s.saveLocked()
	s.publishLocked()
}

func (s *Service) finish(reboot bool) {
	// The record first: once the state says success, it is complete.
	s.mu.Lock()
	recID := s.st.RecordID
	s.mu.Unlock()
	s.markRecord(recID, true, "")

	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	p := &s.st.Progress
	p.State, p.Phase, p.Progress, p.CompletedAt, p.RebootRequired = StateSuccess, PhasePostflight, 100, &now, reboot
	p.Message = "Update installed"
	if reboot {
		p.Message = "Update installed; restart the system to finish"
	}
	s.running = false
	s.saveLocked()
	s.publishLocked()
}

func (s *Service) fail(msg string) {
	s.mu.Lock()
	recID := s.st.RecordID
	s.mu.Unlock()
	s.markRecord(recID, false, msg)

	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	p := &s.st.Progress
	p.State, p.Error, p.Message, p.CompletedAt = StateFailed, msg, "Update failed", &now
	p.Logs = append(p.Logs, LogEntry{Timestamp: now, Level: "error", Message: msg, Phase: p.Phase})
	s.running = false
	s.saveLocked()
	s.publishLocked()
}

func (s *Service) step(phase Phase, pct int, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st.Progress.Phase, s.st.Progress.Progress, s.st.Progress.Message = phase, pct, msg
	s.st.Progress.Logs = append(s.st.Progress.Logs, LogEntry{Timestamp: time.Now().UTC(), Level: "info", Message: msg, Phase: phase})
	s.saveLocked()
	s.publishLocked()
}

func (s *Service) log(level string, phase Phase, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st.Progress.Logs = append(s.st.Progress.Logs, LogEntry{Timestamp: time.Now().UTC(), Level: level, Message: msg, Phase: phase})
	s.trimLogsLocked()
	s.saveLocked()
	s.publishLocked()
}

const maxLogs = 500

func (s *Service) trimLogsLocked() {
	if n := len(s.st.Progress.Logs); n > maxLogs {
		s.st.Progress.Logs = slices.Clone(s.st.Progress.Logs[n-maxLogs:])
	}
}

// pendingPackages is what the last check said would change (for the record).
func (s *Service) pendingPackages(only []string) []Package {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Package{}
	if s.st.LastCheck == nil || s.st.LastCheck.LatestVersion == nil {
		return out
	}
	for _, p := range s.st.LastCheck.LatestVersion.Packages {
		if len(only) == 0 || slices.Contains(only, p.Name) {
			out = append(out, p)
		}
	}
	return out
}

// --- progress ------------------------------------------------------------------

// Progress returns the current or last update.
func (s *Service) Progress() Progress {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.st.Progress
	p.Logs = slices.Clone(p.Logs)
	return p
}

// Subscribe returns a channel that gets the progress after every change, and
// a function that ends the subscription.
func (s *Service) Subscribe() (<-chan Progress, func()) {
	ch := make(chan Progress, 16)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		delete(s.subs, ch)
		s.mu.Unlock()
	}
}

func (s *Service) publishLocked() {
	p := s.st.Progress
	p.Logs = slices.Clone(p.Logs)
	for ch := range s.subs {
		select {
		case ch <- p:
		default: // a slow reader misses intermediate states
		}
	}
}

func (s *Service) saveLocked() {
	_ = fsatomic.SaveJSON(s.statePath(), s.st, 0o640)
}

// --- records, snapshots, rollback ---------------------------------------------

func (s *Service) records() ([]Record, error) {
	var list []Record
	if _, err := fsatomic.LoadJSON(s.recordsPath(), &list); err != nil {
		return nil, err
	}
	slices.SortFunc(list, func(a, b Record) int { return b.CreatedAt.Compare(a.CreatedAt) })
	return list, nil
}

func (s *Service) changeRecords(change func([]Record) []Record) error {
	return fsatomic.WithLock(s.recordsPath(), func() error {
		var list []Record
		if _, err := fsatomic.LoadJSON(s.recordsPath(), &list); err != nil {
			return err
		}
		return fsatomic.SaveJSON(s.recordsPath(), change(list), 0o640)
	})
}

func (s *Service) putRecord(rec Record) error {
	return s.changeRecords(func(list []Record) []Record {
		for i := range list {
			if list[i].ID == rec.ID {
				list[i] = rec
				return list
			}
		}
		return append(list, rec)
	})
}

func (s *Service) markRecord(id string, ok bool, notes string) {
	if id == "" {
		return
	}
	now := time.Now().UTC()
	_ = s.changeRecords(func(list []Record) []Record {
		for i := range list {
			if list[i].ID == id {
				list[i].Success, list[i].Finished, list[i].Notes = &ok, &now, notes
			}
		}
		return list
	})
}

// Records lists the updates and rollbacks, newest first.
func (s *Service) Records() ([]Record, error) { return s.records() }

// Record returns one.
func (s *Service) Record(id string) (Record, error) {
	list, err := s.records()
	if err != nil {
		return Record{}, err
	}
	for _, r := range list {
		if r.ID == id {
			return r, nil
		}
	}
	return Record{}, ErrNotFound
}

// Snapshots lists the update records that have a snapshot to restore.
func (s *Service) Snapshots() ([]Snapshot, error) {
	list, err := s.records()
	if err != nil {
		return nil, err
	}
	out := []Snapshot{}
	for _, r := range list {
		if r.Reason != "update" || len(r.Targets) == 0 {
			continue
		}
		paths := []string{}
		for _, t := range r.Targets {
			paths = append(paths, t.Path)
		}
		desc := fmt.Sprintf("Before updating %d package(s)", len(r.Packages))
		out = append(out, Snapshot{ID: r.ID, CreatedAt: r.CreatedAt, Reason: "update", Subvolumes: paths, CanRollback: true, Description: desc})
	}
	return out, nil
}

// DeleteSnapshot deletes a snapshot's files and its record.
func (s *Service) DeleteSnapshot(ctx context.Context, id string) error {
	rec, err := s.Record(id)
	if err != nil {
		return err
	}
	for _, t := range rec.Targets {
		if err := s.agent.PostJSON(ctx, "/v1/snapshot/delete", map[string]any{"path": t.Path, "snapshot_id": t.ID, "type": t.Type}, nil); err != nil {
			return fmt.Errorf("delete %s: %w", t.Location, err)
		}
	}
	return s.changeRecords(func(list []Record) []Record {
		return slices.DeleteFunc(list, func(r Record) bool { return r.ID == id })
	})
}

// Rollback restores the configuration saved before update id, then restarts
// nosd so it reads it. Packages are not downgraded.
func (s *Service) Rollback(ctx context.Context, id string) error {
	rec, err := s.Record(id)
	if err != nil {
		return err
	}
	if len(rec.Targets) == 0 {
		return fmt.Errorf("update %s has no snapshot", id)
	}
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return ErrBusy
	}
	s.running = true
	now := time.Now().UTC()
	s.st.Progress = Progress{State: StateRollingBack, Phase: PhasePostflight, Progress: 10, Message: "Restoring the configuration", StartedAt: now, Logs: []LogEntry{}, SnapshotID: id}
	s.st.RecordID = ""
	s.saveLocked()
	s.publishLocked()
	s.mu.Unlock()

	roll := Record{ID: uuid.NewString(), CreatedAt: now, Reason: "rollback", Packages: []Package{}, Targets: rec.Targets, Notes: "rollback of " + id}
	for _, t := range rec.Targets {
		if err := s.agent.PostJSON(ctx, "/v1/snapshot/rollback", map[string]any{"path": t.Path, "snapshot_id": t.ID, "type": t.Type}, nil); err != nil {
			s.fail("Restoring " + t.Path + " failed: " + err.Error())
			ok := false
			roll.Success = &ok
			_ = s.putRecord(roll)
			return err
		}
	}
	ok := true
	done := time.Now().UTC()
	roll.Success, roll.Finished = &ok, &done
	_ = s.putRecord(roll)

	s.mu.Lock()
	p := &s.st.Progress
	p.State, p.Progress, p.CompletedAt = StateRolledBack, 100, &done
	p.Message = "Configuration restored; nosd restarts to load it (packages were not downgraded)"
	s.running = false
	s.saveLocked()
	s.publishLocked()
	s.mu.Unlock()

	// nosd holds the old state in memory; restart it (after this reply).
	go func() {
		time.Sleep(time.Second)
		_ = s.agent.PostJSON(context.Background(), "/v1/service/reload", map[string]any{"name": "nosd"}, nil)
	}()
	return nil
}

// Prune deletes all but the newest keep update snapshots.
func (s *Service) Prune(ctx context.Context, keep int) (int, error) {
	snaps, err := s.Snapshots() // newest first
	if err != nil {
		return 0, err
	}
	deleted := 0
	for i, sn := range snaps {
		if i < keep {
			continue
		}
		if err := s.DeleteSnapshot(ctx, sn.ID); err != nil {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}
