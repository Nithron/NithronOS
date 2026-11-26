package backup

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

type fakeAgent struct {
	mu       sync.Mutex
	sent     []SendRequest
	deleted  []string
	statuses []SendStatus
}

func (f *fakeAgent) CreateSnapshot(subvolume, tag string) (SnapshotInfo, error) {
	return SnapshotInfo{ID: "20261126-120000-" + tag, Path: subvolume + "/.snapshots/20261126-120000-" + tag, Subvolume: subvolume, CreatedAt: time.Now()}, nil
}
func (f *fakeAgent) DeleteSnapshot(s *Snapshot) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, s.AgentID)
	return nil
}
func (f *fakeAgent) RollbackSnapshot(*Snapshot, []string) error { return nil }
func (f *fakeAgent) Send(req SendRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, req)
	return nil
}
func (f *fakeAgent) SendStatus(string) (SendStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	st := f.statuses[0]
	if len(f.statuses) > 1 {
		f.statuses = f.statuses[1:]
	}
	return st, nil
}

func waitJob(t *testing.T, jm *JobManager, id string) *BackupJob {
	t.Helper()
	for i := 0; i < 500; i++ {
		if j, ok := jm.GetJob(id); ok && (j.State == JobStateSucceeded || j.State == JobStateFailed) {
			return j
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("job did not finish")
	return nil
}

func TestSnapshotThenReplicate(t *testing.T) {
	dir := t.TempDir()
	agent := &fakeAgent{statuses: []SendStatus{{State: "running", Log: []string{"At subvol x"}}, {State: "succeeded", Log: []string{"At subvol x"}}}}
	s := NewScheduler(zerolog.Nop(), filepath.Join(dir, "schedules.json"), agent)
	r := NewReplicator(zerolog.Nop(), filepath.Join(dir, "dest.json"), filepath.Join(dir, "keys"), s.GetJobManager())
	r.Connect(agent, s.ListSnapshots)
	r.pollEvery = time.Millisecond

	job, err := s.CreateSnapshot([]string{"/srv/shares/media"}, "manual")
	if err != nil {
		t.Fatal(err)
	}
	if j := waitJob(t, s.GetJobManager(), job.ID); j.State != JobStateSucceeded {
		t.Fatalf("snapshot job: %+v", j)
	}
	snaps := s.ListSnapshots()
	if len(snaps) != 1 || snaps[0].Path != "/srv/shares/media/.snapshots/20261126-120000-manual" || snaps[0].AgentID == "" {
		t.Fatalf("snapshots: %+v", snaps)
	}

	if err := r.CreateDestination(&Destination{Name: "nas2", Type: "ssh", Host: "backup.lan", User: "nos", Path: "/backups", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	dest := r.ListDestinations()[0]
	rjob, err := r.Replicate(dest.ID, snaps[0].ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if j := waitJob(t, s.GetJobManager(), rjob.ID); j.State != JobStateSucceeded {
		t.Fatalf("replication: %+v", j)
	}
	if len(agent.sent) != 1 || agent.sent[0].Snapshot != snaps[0].Path || agent.sent[0].Target != "ssh" || agent.sent[0].Port != 22 {
		t.Fatalf("sent: %+v", agent.sent)
	}

	if err := s.DeleteSnapshot(snaps[0].ID); err != nil || len(agent.deleted) != 1 {
		t.Fatalf("delete: %v %v", err, agent.deleted)
	}
}

func TestDestinationValidation(t *testing.T) {
	r := NewReplicator(zerolog.Nop(), filepath.Join(t.TempDir(), "d.json"), t.TempDir(), NewJobManager(zerolog.Nop()))
	for _, d := range []Destination{
		{Name: "x", Type: "ssh", Host: "h", User: "u", Path: "/x; rm -rf /"},
		{Name: "x", Type: "ssh", Host: "-oProxyCommand=evil", User: "u", Path: "/x"},
		{Name: "x", Type: "local", Path: "../etc"},
		{Name: "x", Type: "rclone", RemoteName: "r", RemotePath: "p"},
	} {
		d := d
		if err := r.CreateDestination(&d); err == nil {
			t.Errorf("accepted %+v", d)
		}
	}
}

func TestHooksRefused(t *testing.T) {
	s := NewScheduler(zerolog.Nop(), filepath.Join(t.TempDir(), "s.json"), &fakeAgent{})
	err := s.CreateSchedule(&Schedule{Name: "x", Subvolumes: []string{"/srv/shares/a"}, Frequency: ScheduleFrequency{Type: "daily"}, PreHooks: []string{"rm -rf /"}})
	if err == nil {
		t.Fatal("a schedule with hooks was accepted")
	}
}
