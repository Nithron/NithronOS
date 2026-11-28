package updates

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeAgent answers like nos-agent; statuses are returned in turn by
// /v1/updates/status, reverts by /v1/updates/revert/status and health by
// /v1/updates/health (the last of each repeats).
type fakeAgent struct {
	mu       sync.Mutex
	statuses []agentStatus
	reverts  []agentStatus
	health   []healthReply
	missing  bool // /v1/updates/keep can't keep curl
	calls    []string
	bodies   map[string]any
	failOn   map[string]error
}

func next[T any](list *[]T, fallback T) T {
	if len(*list) == 0 {
		return fallback
	}
	v := (*list)[0]
	if len(*list) > 1 {
		*list = (*list)[1:]
	}
	return v
}

func (f *fakeAgent) record(path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, path)
	return f.failOn[path]
}

func (f *fakeAgent) GetJSON(_ context.Context, path string, out any) error {
	if err := f.record(path); err != nil {
		return err
	}
	f.mu.Lock()
	var v any
	switch path {
	case "/v1/updates/revert/status":
		v = next(&f.reverts, agentStatus{State: "succeeded"})
	case "/v1/updates/health":
		v = next(&f.health, healthReply{OK: true})
	default:
		v = next(&f.statuses, agentStatus{State: "none"})
	}
	f.mu.Unlock()
	b, _ := json.Marshal(v)
	return json.Unmarshal(b, out)
}

func (f *fakeAgent) PostJSON(_ context.Context, path string, body any, out any) error {
	if err := f.record(path); err != nil {
		return err
	}
	f.mu.Lock()
	if f.bodies == nil {
		f.bodies = map[string]any{}
	}
	f.bodies[path] = body
	missing := f.missing
	f.mu.Unlock()
	var resp string
	switch path {
	case "/v1/updates/keep":
		resp = `{"kept":[{"name":"nosd","version":"0.9.5~pre-alpha-1"},{"name":"curl","version":"1"}],"missing":[]}`
		if missing {
			resp = `{"kept":[{"name":"nosd","version":"0.9.5~pre-alpha-1"}],"missing":[{"name":"curl","version":"1","error":"dpkg-repack: not found"}]}`
		}
	case "/v1/updates/plan":
		resp = `{"updates":[{"name":"nosd","current":"0.9.5~pre-alpha-1","candidate":"0.9.6~pre-alpha-1"},{"name":"curl","current":"1","candidate":"2"}]}`
	case "/v1/snapshot/create":
		resp = `{"ok":true,"id":"20261126-120000-pre-update","type":"tar","location":"/var/lib/nos/snapshots/etc-nos/20261126-120000-pre-update.tar.gz"}`
	default:
		resp = `{"ok":true}`
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal([]byte(resp), out)
}

func (f *fakeAgent) called(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if c == path {
			n++
		}
	}
	return n
}

func newTestService(t *testing.T, agent *fakeAgent) *Service {
	t.Helper()
	dir := t.TempDir()
	s, err := New(agent, filepath.Join(dir, "updates"), filepath.Join(dir, "etc", "update", "channel"))
	if err != nil {
		t.Fatal(err)
	}
	s.pollEvery = time.Millisecond
	s.postflightWait = 20 * time.Millisecond
	s.minFree = 0
	return s
}

func waitFor(t *testing.T, s *Service, states ...State) Progress {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		p := s.Progress()
		for _, st := range states {
			if p.State == st {
				return p
			}
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("still %+v", s.Progress())
	return Progress{}
}

func TestUpdateSucceeds(t *testing.T) {
	agent := &fakeAgent{statuses: []agentStatus{
		{State: "none"}, // preflight
		{State: "running", Log: []string{"Reading package lists..."}},
		{State: "running", Log: []string{"Reading package lists...", "Setting up nosd (0.9.6) ..."}},
		{State: "succeeded", Log: []string{"Reading package lists...", "Setting up nosd (0.9.6) ...", "Setting up curl (2) ..."}, RebootRequired: true},
	}}
	s := newTestService(t, agent)
	check, err := s.Check(context.Background())
	if err != nil || !check.UpdateAvailable || check.LatestVersion.Version != "0.9.6~pre-alpha-1" || len(check.LatestVersion.Packages) != 2 {
		t.Fatalf("check: %v %+v", err, check)
	}
	if err := s.Apply(ApplyRequest{}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(ApplyRequest{}); !errors.Is(err, ErrBusy) {
		t.Fatalf("second apply while running: %v", err)
	}
	p := waitFor(t, s, StateSuccess, StateFailed)
	if p.State != StateSuccess || p.Progress != 100 || !p.RebootRequired || p.SnapshotID == "" {
		t.Fatalf("progress: %+v", p)
	}
	// Each agent log line appears once.
	n := 0
	for _, l := range p.Logs {
		if l.Message == "Setting up nosd (0.9.6) ..." {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("log line repeated %d times: %+v", n, p.Logs)
	}
	if agent.called("/v1/snapshot/create") != 1 || agent.called("/v1/updates/keep") != 1 || agent.called("/v1/updates/apply") != 1 || agent.called("/v1/updates/health") != 1 {
		t.Fatalf("calls: %v", agent.calls)
	}
	// The installed versions are kept, by the id of the update's record.
	keep := agent.bodies["/v1/updates/keep"].(map[string]any)
	if want := keep["packages"].([]map[string]string); len(want) != 2 || want[0]["name"] != "nosd" || want[0]["version"] != "0.9.5~pre-alpha-1" || keep["id"] != p.SnapshotID {
		t.Fatalf("keep: %+v", keep)
	}

	snaps, _ := s.Snapshots()
	if len(snaps) != 1 || snaps[0].Subvolumes[0] != ConfigDir || !snaps[0].CanRollback {
		t.Fatalf("snapshots: %+v", snaps)
	}
	rec, _ := s.Record(snaps[0].ID)
	if rec.Success == nil || !*rec.Success || len(rec.Packages) != 2 || len(rec.Kept) != 2 {
		t.Fatalf("record: %+v", rec)
	}

	// Rollback restores the saved configuration and reinstalls the kept
	// versions; the agent's unit restarts nosd.
	if err := s.Rollback(context.Background(), snaps[0].ID); err != nil {
		t.Fatal(err)
	}
	if p := waitFor(t, s, StateRolledBack, StateFailed); p.State != StateRolledBack || p.Error != "" {
		t.Fatalf("after rollback: %+v", p)
	}
	if agent.called("/v1/snapshot/rollback") != 1 || agent.called("/v1/updates/revert") != 1 {
		t.Fatalf("calls: %v", agent.calls)
	}
	if rec, _ := s.Record(snaps[0].ID); rec.Success == nil || !*rec.Success {
		t.Fatalf("a manual rollback leaves the update marked as it was: %+v", rec)
	}
	if err := s.Rollback(context.Background(), "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown snapshot: %v", err)
	}
}

func TestFailedUpgradeIsRolledBack(t *testing.T) {
	agent := &fakeAgent{statuses: []agentStatus{{State: "none"}, {State: "failed", ExitStatus: 100, Log: []string{"E: broken"}}}}
	s := newTestService(t, agent)
	if err := s.Apply(ApplyRequest{}); err != nil {
		t.Fatal(err)
	}
	p := waitFor(t, s, StateSuccess, StateFailed, StateRolledBack)
	if p.State != StateRolledBack || !strings.Contains(p.Error, "exit status 100") {
		t.Fatalf("progress: %+v", p)
	}
	if agent.called("/v1/snapshot/rollback") != 1 || agent.called("/v1/updates/revert") != 1 {
		t.Fatalf("calls: %v", agent.calls)
	}
	rec, _ := s.Record(p.SnapshotID)
	if rec.Success == nil || *rec.Success || !strings.Contains(rec.Notes, "rolled back automatically") {
		t.Fatalf("record: %+v", rec)
	}

	// The rollback failing too is said plainly.
	agent = &fakeAgent{statuses: []agentStatus{{State: "none"}, {State: "failed", ExitStatus: 100}}, reverts: []agentStatus{{State: "failed", ExitStatus: 1}}}
	s = newTestService(t, agent)
	_ = s.Apply(ApplyRequest{})
	p = waitFor(t, s, StateSuccess, StateFailed, StateRolledBack)
	if p.State != StateFailed || !strings.Contains(p.Error, "The rollback failed too") {
		t.Fatalf("failed rollback: %+v", p)
	}
}

func TestPostflight(t *testing.T) {
	// The services come back after a moment: the update stands.
	agent := &fakeAgent{
		statuses: []agentStatus{{State: "none"}, {State: "succeeded"}},
		health:   []healthReply{{Problems: []string{"caddy is activating"}}, {OK: true}},
	}
	s := newTestService(t, agent)
	_ = s.Apply(ApplyRequest{})
	if p := waitFor(t, s, StateSuccess, StateFailed, StateRolledBack); p.State != StateSuccess {
		t.Fatalf("progress: %+v", p)
	}

	// Still broken when the wait is over: rolled back, and it says why.
	agent = &fakeAgent{
		statuses: []agentStatus{{State: "none"}, {State: "succeeded"}},
		health:   []healthReply{{Problems: []string{"caddy is failed"}}},
	}
	s = newTestService(t, agent)
	_ = s.Apply(ApplyRequest{})
	p := waitFor(t, s, StateSuccess, StateFailed, StateRolledBack)
	if p.State != StateRolledBack || p.Error != "After the update, caddy is failed" || agent.called("/v1/updates/revert") != 1 {
		t.Fatalf("progress: %+v %v", p, agent.calls)
	}
}

func TestPreflightStops(t *testing.T) {
	// A version that can't be kept stops the update: it couldn't be undone.
	agent := &fakeAgent{statuses: []agentStatus{{State: "none"}}, missing: true}
	s := newTestService(t, agent)
	_ = s.Apply(ApplyRequest{})
	p := waitFor(t, s, StateSuccess, StateFailed)
	if p.State != StateFailed || !strings.Contains(p.Error, "curl 1 (dpkg-repack: not found)") || agent.called("/v1/updates/apply") != 0 {
		t.Fatalf("progress: %+v %v", p, agent.calls)
	}
	// Forced, it goes ahead with what was kept.
	agent = &fakeAgent{statuses: []agentStatus{{State: "none"}, {State: "succeeded"}}, missing: true}
	s = newTestService(t, agent)
	_ = s.Apply(ApplyRequest{Force: true})
	if p := waitFor(t, s, StateSuccess, StateFailed); p.State != StateSuccess {
		t.Fatalf("forced: %+v", p)
	}

	// Too little space: nothing is done.
	agent = &fakeAgent{statuses: []agentStatus{{State: "none"}}}
	s = newTestService(t, agent)
	s.minFree = ^uint64(0)
	_ = s.Apply(ApplyRequest{})
	p = waitFor(t, s, StateSuccess, StateFailed)
	if p.State != StateFailed || !strings.Contains(p.Error, "an update needs") || agent.called("/v1/snapshot/create") != 0 {
		t.Fatalf("space: %+v %v", p, agent.calls)
	}
}

// A failed snapshot stops the update before anything is installed.
func TestSnapshotFailureStops(t *testing.T) {
	agent := &fakeAgent{statuses: []agentStatus{{State: "none"}}, failOn: map[string]error{"/v1/snapshot/create": errors.New("disk full")}}
	s := newTestService(t, agent)
	if err := s.Apply(ApplyRequest{}); err != nil {
		t.Fatal(err)
	}
	p := waitFor(t, s, StateSuccess, StateFailed)
	if p.State != StateFailed || agent.called("/v1/updates/apply") != 0 {
		t.Fatalf("snapshot failure must stop the update: %+v %v", p, agent.calls)
	}
}

func TestResumeAfterRestart(t *testing.T) {
	agent := &fakeAgent{statuses: []agentStatus{{State: "running"}, {State: "succeeded"}}}
	dir := t.TempDir()
	s, _ := New(agent, dir, filepath.Join(dir, "channel"))
	s.mu.Lock()
	s.st.Progress = Progress{State: StateApplying, Phase: PhaseInstall, Logs: []LogEntry{}}
	s.saveLocked()
	s.mu.Unlock()

	// A new process (nosd restarted by the upgrade) picks it up.
	s2, err := New(agent, dir, filepath.Join(dir, "channel"))
	if err != nil {
		t.Fatal(err)
	}
	s2.pollEvery = time.Millisecond
	s2.Resume()
	if p := waitFor(t, s2, StateSuccess, StateFailed); p.State != StateSuccess {
		t.Fatalf("resumed: %+v", p)
	}

	// Interrupted before installing: marked failed, nothing to follow.
	s3, _ := New(agent, t.TempDir(), filepath.Join(dir, "channel"))
	s3.mu.Lock()
	s3.st.Progress = Progress{State: StateApplying, Phase: PhaseSnapshot, Logs: []LogEntry{}}
	s3.mu.Unlock()
	s3.Resume()
	if p := s3.Progress(); p.State != StateFailed {
		t.Fatalf("interrupted early: %+v", p)
	}
}

func TestResumeDuringRollback(t *testing.T) {
	agent := &fakeAgent{reverts: []agentStatus{{State: "running"}, {State: "succeeded"}}}
	dir := t.TempDir()
	s, _ := New(agent, dir, filepath.Join(dir, "channel"))
	_ = s.putRecord(Record{ID: "u1", CreatedAt: time.Now(), Reason: "update", Kept: []Package{{Name: "nosd", CurrentVersion: "1"}}})
	_ = s.putRecord(Record{ID: "r1", CreatedAt: time.Now(), Reason: "rollback"})
	s.mu.Lock()
	s.st.RecordID = "u1"
	s.st.Progress = Progress{State: StateRollingBack, Phase: PhaseRollback, Logs: []LogEntry{}}
	s.st.Rollback = &rollbackState{Of: "u1", RecordID: "r1", Auto: true, Reason: "apt-get failed", Started: true}
	s.saveLocked()
	s.mu.Unlock()

	// The revert unit restarted nosd; the new process finishes the rollback.
	s2, _ := New(agent, dir, filepath.Join(dir, "channel"))
	s2.pollEvery = time.Millisecond
	s2.Resume()
	if p := waitFor(t, s2, StateRolledBack, StateFailed); p.State != StateRolledBack || p.Error != "apt-get failed" {
		t.Fatalf("resumed: %+v", p)
	}
	if agent.called("/v1/updates/revert") != 0 {
		t.Fatalf("a started revert is followed, not started again: %v", agent.calls)
	}
	if r, _ := s2.Record("r1"); r.Success == nil || !*r.Success {
		t.Fatalf("rollback record: %+v", r)
	}
	if u, _ := s2.Record("u1"); u.Success == nil || *u.Success {
		t.Fatalf("update record: %+v", u)
	}
}

func TestPruneAfterUpdateKeepsRetention(t *testing.T) {
	agent := &fakeAgent{statuses: []agentStatus{{State: "none"}, {State: "succeeded"}}}
	s := newTestService(t, agent)
	if err := s.SetConfig(Config{SnapshotRetention: 0}); err == nil {
		t.Fatal("retention 0 accepted")
	}
	if c := s.Config(); c.SnapshotRetention != DefaultRetention {
		t.Fatalf("default: %+v", c)
	}
	if err := s.SetConfig(Config{SnapshotRetention: 2}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		_ = s.putRecord(Record{ID: string(rune('a' + i)), CreatedAt: time.Now().Add(-time.Hour + time.Duration(i)*time.Minute), Reason: "update",
			Targets: []Target{{Path: ConfigDir, ID: "x", Type: "tar"}}, Kept: []Package{{Name: "nosd", CurrentVersion: "1"}}})
	}
	_ = s.Apply(ApplyRequest{})
	p := waitFor(t, s, StateSuccess, StateFailed)
	snaps, _ := s.Snapshots()
	if p.State != StateSuccess || len(snaps) != 2 || snaps[0].ID != p.SnapshotID || snaps[1].ID != "c" {
		t.Fatalf("kept: %+v %+v", p, snaps)
	}
	// The kept packages go with their snapshot.
	if agent.called("/v1/updates/keep/delete") != 2 {
		t.Fatalf("calls: %v", agent.calls)
	}
}

func TestChannelAndPrune(t *testing.T) {
	agent := &fakeAgent{statuses: []agentStatus{{State: "none"}}}
	s := newTestService(t, agent)
	if s.Channel() != ChannelStable {
		t.Fatal("default channel")
	}
	if err := s.SetChannel("nightly"); err == nil {
		t.Fatal("unknown channel accepted")
	}
	if err := s.SetChannel(ChannelBeta); err != nil || s.Channel() != ChannelBeta {
		t.Fatalf("set channel: %v", err)
	}

	for i := 0; i < 3; i++ {
		_ = s.putRecord(Record{ID: string(rune('a' + i)), CreatedAt: time.Now().Add(time.Duration(i) * time.Minute), Reason: "update",
			Targets: []Target{{Path: ConfigDir, ID: "20261126-12000" + string(rune('0'+i)) + "-pre-update", Type: "tar"}}})
	}
	n, err := s.Prune(context.Background(), 1)
	if err != nil || n != 2 || agent.called("/v1/snapshot/delete") != 2 {
		t.Fatalf("prune: %d %v %v", n, err, agent.calls)
	}
	if snaps, _ := s.Snapshots(); len(snaps) != 1 || snaps[0].ID != "c" {
		t.Fatalf("kept: %+v", snaps)
	}
}
