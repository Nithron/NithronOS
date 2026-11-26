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
// /v1/updates/status (the last one repeats).
type fakeAgent struct {
	mu       sync.Mutex
	statuses []agentStatus
	calls    []string
	failOn   map[string]error
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
	st := f.statuses[0]
	if len(f.statuses) > 1 {
		f.statuses = f.statuses[1:]
	}
	f.mu.Unlock()
	b, _ := json.Marshal(st)
	return json.Unmarshal(b, out)
}

func (f *fakeAgent) PostJSON(_ context.Context, path string, _ any, out any) error {
	if err := f.record(path); err != nil {
		return err
	}
	var resp string
	switch path {
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
	if agent.called("/v1/snapshot/create") != 1 || agent.called("/v1/updates/apply") != 1 {
		t.Fatalf("calls: %v", agent.calls)
	}

	snaps, _ := s.Snapshots()
	if len(snaps) != 1 || snaps[0].Subvolumes[0] != ConfigDir || !snaps[0].CanRollback {
		t.Fatalf("snapshots: %+v", snaps)
	}
	rec, _ := s.Record(snaps[0].ID)
	if rec.Success == nil || !*rec.Success || len(rec.Packages) != 2 {
		t.Fatalf("record: %+v", rec)
	}

	// Rollback restores the saved configuration and restarts nosd.
	if err := s.Rollback(context.Background(), snaps[0].ID); err != nil {
		t.Fatal(err)
	}
	if p := s.Progress(); p.State != StateRolledBack {
		t.Fatalf("after rollback: %+v", p)
	}
	if agent.called("/v1/snapshot/rollback") != 1 {
		t.Fatalf("calls: %v", agent.calls)
	}
	if err := s.Rollback(context.Background(), "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown snapshot: %v", err)
	}
}

func TestUpdateFailsAndSnapshotFailureStops(t *testing.T) {
	agent := &fakeAgent{statuses: []agentStatus{{State: "none"}, {State: "failed", ExitStatus: 100, Log: []string{"E: broken"}}}}
	s := newTestService(t, agent)
	if err := s.Apply(ApplyRequest{}); err != nil {
		t.Fatal(err)
	}
	p := waitFor(t, s, StateSuccess, StateFailed)
	if p.State != StateFailed || !strings.Contains(p.Error, "exit status 100") {
		t.Fatalf("progress: %+v", p)
	}

	// A failed snapshot stops the update before anything is installed.
	agent = &fakeAgent{statuses: []agentStatus{{State: "none"}}, failOn: map[string]error{"/v1/snapshot/create": errors.New("disk full")}}
	s = newTestService(t, agent)
	if err := s.Apply(ApplyRequest{}); err != nil {
		t.Fatal(err)
	}
	p = waitFor(t, s, StateSuccess, StateFailed)
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
