package updates

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// rollbackState is a rollback in progress, kept in state.json: the agent's
// revert unit restarts nosd when it is done, and the new nosd finishes it.
type rollbackState struct {
	Of       string `json:"of"`        // the update record rolled back
	RecordID string `json:"record_id"` // the rollback's own record
	Auto     bool   `json:"auto"`      // started because the update failed
	Reason   string `json:"reason,omitempty"`
	Started  bool   `json:"started"` // the agent's revert unit was started
}

// --- preflight and postflight -------------------------------------------------

// spaceProblem says what is wrong with the free space for an update, if anything.
func (s *Service) spaceProblem() string {
	for _, path := range []string{"/", "/var/cache/apt"} {
		free, err := freeBytes(path)
		if err != nil {
			continue // not there (a test system): apt will say
		}
		if free < s.minFree {
			return fmt.Sprintf("Only %.1f GB is free on %s; an update needs %.0f GB", float64(free)/1e9, path, float64(s.minFree)/(1<<30))
		}
	}
	return ""
}

type keepReply struct {
	Kept    []struct{ Name, Version string } `json:"kept"`
	Missing []struct {
		Name, Version, Error string
	} `json:"missing"`
}

// keepVersions has the agent keep the .debs of the installed versions of the
// packages rec upgrades. It returns what was kept; the error names what wasn't.
func (s *Service) keepVersions(ctx context.Context, rec Record) ([]Package, error) {
	s.step(PhaseSnapshot, 15, fmt.Sprintf("Keeping the installed versions of %d package(s)", len(rec.Packages)))
	want := []map[string]string{}
	for _, p := range rec.Packages {
		if p.CurrentVersion != "" {
			want = append(want, map[string]string{"name": p.Name, "version": p.CurrentVersion})
		}
	}
	var reply keepReply
	if err := s.agent.PostJSON(ctx, "/v1/updates/keep", map[string]any{"id": rec.ID, "packages": want}, &reply); err != nil {
		return nil, fmt.Errorf("The installed versions could not be kept, so the update could not be rolled back: %w", err)
	}
	kept := []Package{}
	for _, k := range reply.Kept {
		kept = append(kept, Package{Name: k.Name, CurrentVersion: k.Version})
	}
	if len(reply.Missing) > 0 {
		names := []string{}
		for _, m := range reply.Missing {
			names = append(names, fmt.Sprintf("%s %s (%s)", m.Name, m.Version, m.Error))
		}
		return kept, fmt.Errorf("The installed versions of %d package(s) could not be kept, so the update could not be rolled back: %s", len(reply.Missing), strings.Join(names, "; "))
	}
	s.log("info", PhaseSnapshot, fmt.Sprintf("Kept the installed versions of %d package(s)", len(kept)))
	return kept, nil
}

type healthReply struct {
	OK       bool     `json:"ok"`
	Problems []string `json:"problems"`
}

// postflight checks that nOS works after the upgrade: nosd, nos-agent and
// Caddy running and the web interface answering. It waits for them to come
// back (the upgrade restarts them); still broken, the update is rolled back.
func (s *Service) postflight(ctx context.Context, reboot bool) {
	s.mu.Lock()
	s.st.Progress.State = StateVerifying
	s.mu.Unlock()
	s.step(PhasePostflight, 92, "Checking that nOS works after the update")
	deadline := time.Now().Add(s.postflightWait)
	last := "nos-agent did not answer"
	for {
		var h healthReply
		err := s.agent.GetJSON(ctx, "/v1/updates/health", &h)
		switch {
		case err == nil && h.OK:
			s.pruneAfterUpdate(ctx)
			s.finish(reboot)
			return
		case err == nil:
			last = strings.Join(h.Problems, "; ")
		default:
			last = "nos-agent did not answer: " + err.Error()
		}
		if time.Now().After(deadline) {
			s.autoRollback(ctx, "After the update, "+last)
			return
		}
		time.Sleep(s.pollEvery)
	}
}

// pruneAfterUpdate keeps the newest snapshots (snapshot_retention).
func (s *Service) pruneAfterUpdate(ctx context.Context) {
	keep := s.Config().SnapshotRetention
	n, err := s.Prune(ctx, keep)
	switch {
	case err != nil:
		s.log("warn", PhasePostflight, "Could not delete old snapshots: "+err.Error())
	case n > 0:
		s.log("info", PhasePostflight, fmt.Sprintf("Deleted %d old snapshot(s); the newest %d are kept", n, keep))
	}
}

// finishNothing ends an update that found nothing to upgrade.
func (s *Service) finishNothing() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	p := &s.st.Progress
	p.State, p.Phase, p.Progress, p.CompletedAt, p.Message = StateSuccess, PhasePreflight, 100, &now, "Nothing to update: everything is up to date"
	s.running = false
	s.saveLocked()
	s.publishLocked()
}

// --- rollback --------------------------------------------------------------------

// autoRollback rolls the running update back after it failed for reason.
func (s *Service) autoRollback(ctx context.Context, reason string) {
	s.mu.Lock()
	recID := s.st.RecordID
	s.mu.Unlock()
	rec, err := s.Record(recID)
	if err != nil || (len(rec.Targets) == 0 && len(rec.Kept) == 0) {
		s.fail(reason + ". Nothing was saved before the update, so it could not be rolled back.")
		return
	}
	s.log("error", PhaseRollback, reason+". Rolling the update back.")
	s.beginRollback(rec, true, reason)
	if err := s.continueRollback(ctx); err != nil {
		return
	}
	s.followRevert(ctx)
}

// Rollback restores what was saved before update id: nOS's configuration and
// the package versions installed then. It returns once the reinstall has
// started; nosd restarts when it is done, and Progress says how it went.
func (s *Service) Rollback(ctx context.Context, id string) error {
	rec, err := s.Record(id)
	if err != nil {
		return err
	}
	if len(rec.Targets) == 0 && len(rec.Kept) == 0 {
		return fmt.Errorf("update %s has no snapshot", id)
	}
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return ErrBusy
	}
	s.running = true
	s.mu.Unlock()
	s.beginRollback(rec, false, "")
	if err := s.continueRollback(ctx); err != nil {
		return err
	}
	go s.followRevert(context.Background())
	return nil
}

func (s *Service) beginRollback(rec Record, auto bool, reason string) {
	now := time.Now().UTC()
	rb := &rollbackState{Of: rec.ID, RecordID: uuid.NewString(), Auto: auto, Reason: reason}
	_ = s.putRecord(Record{ID: rb.RecordID, CreatedAt: now, Reason: "rollback", Packages: []Package{}, Targets: rec.Targets, Kept: rec.Kept, Notes: "rollback of " + rec.ID})

	s.mu.Lock()
	defer s.mu.Unlock()
	p := &s.st.Progress
	if !auto {
		*p = Progress{StartedAt: now, Logs: []LogEntry{}}
	}
	p.State, p.Phase, p.Progress, p.SnapshotID = StateRollingBack, PhaseRollback, 10, rec.ID
	p.Message = "Restoring the configuration"
	p.Logs = append(p.Logs, LogEntry{Timestamp: now, Level: "info", Message: "Restoring the configuration and the previous package versions", Phase: PhaseRollback})
	s.st.Rollback, s.st.LogsSeen = rb, 0
	s.saveLocked()
	s.publishLocked()
}

// resumeRollback carries on after nosd restarted during a rollback.
func (s *Service) resumeRollback(ctx context.Context, started bool) {
	if !started {
		if err := s.continueRollback(ctx); err != nil {
			return
		}
	}
	s.followRevert(ctx)
}

// continueRollback restores the configuration, then has the agent reinstall
// the kept versions (its unit restarts nosd at the end).
func (s *Service) continueRollback(ctx context.Context) error {
	s.mu.Lock()
	rb := *s.st.Rollback
	s.mu.Unlock()
	rec, err := s.Record(rb.Of)
	if err != nil {
		s.rollbackFailed("The update record " + rb.Of + " is gone")
		return err
	}
	for _, t := range rec.Targets {
		if err := s.agent.PostJSON(ctx, "/v1/snapshot/rollback", map[string]any{"path": t.Path, "snapshot_id": t.ID, "type": t.Type}, nil); err != nil {
			s.rollbackFailed("Restoring " + t.Path + " failed: " + err.Error())
			return err
		}
		s.log("info", PhaseRollback, "Restored "+t.Path)
	}
	s.step(PhaseRollback, 40, fmt.Sprintf("Reinstalling the previous versions of %d package(s); nosd restarts when it is done", len(rec.Kept)))
	if err := s.agent.PostJSON(ctx, "/v1/updates/revert", map[string]any{"id": rec.ID}, nil); err != nil {
		s.rollbackFailed("Could not start reinstalling the previous versions: " + err.Error())
		return err
	}
	s.mu.Lock()
	s.st.Rollback.Started = true
	s.saveLocked()
	s.mu.Unlock()
	return nil
}

// followRevert polls the agent's revert unit until it finishes.
func (s *Service) followRevert(ctx context.Context) {
	start := time.Now()
	for {
		var st agentStatus
		if err := s.agent.GetJSON(ctx, "/v1/updates/revert/status", &st); err != nil {
			if time.Since(start) > 2*time.Hour {
				s.rollbackFailed("Lost track of the rollback: " + err.Error())
				return
			}
			time.Sleep(s.pollEvery)
			continue
		}
		s.addAgentLog(st.Log)
		switch st.State {
		case "succeeded":
			s.finishRollback()
			return
		case "failed":
			s.rollbackFailed(fmt.Sprintf("Reinstalling the previous versions failed (exit status %d)", st.ExitStatus))
			return
		case "none":
			if time.Since(start) > time.Minute {
				s.rollbackFailed("The rollback did not run")
				return
			}
		}
		time.Sleep(s.pollEvery)
	}
}

func (s *Service) finishRollback() {
	s.mu.Lock()
	rb := s.st.Rollback
	s.mu.Unlock()
	if rb != nil {
		s.markRecord(rb.RecordID, true, "rollback of "+rb.Of)
		if rb.Auto {
			s.markRecord(rb.Of, false, rb.Reason+"; rolled back automatically")
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	p := &s.st.Progress
	p.State, p.Progress, p.CompletedAt = StateRolledBack, 100, &now
	p.Message = "Rolled back: the configuration and the previous package versions are restored"
	if rb != nil && rb.Auto {
		p.Error = rb.Reason
		p.Message = "The update failed and was rolled back: the configuration and the previous package versions are restored"
	}
	p.Logs = append(p.Logs, LogEntry{Timestamp: now, Level: "info", Message: p.Message, Phase: PhaseRollback})
	s.st.Rollback, s.running = nil, false
	s.saveLocked()
	s.publishLocked()
}

func (s *Service) rollbackFailed(msg string) {
	s.mu.Lock()
	rb := s.st.Rollback
	s.mu.Unlock()
	if rb != nil {
		s.markRecord(rb.RecordID, false, msg)
		if rb.Auto {
			s.markRecord(rb.Of, false, rb.Reason+"; the rollback failed: "+msg)
			msg = rb.Reason + ". The rollback failed too: " + msg
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	p := &s.st.Progress
	p.State, p.CompletedAt, p.Message = StateFailed, &now, "Rollback failed"
	p.Error = msg + ". The system may be between versions; see the log."
	p.Logs = append(p.Logs, LogEntry{Timestamp: now, Level: "error", Message: p.Error, Phase: PhaseRollback})
	s.st.Rollback, s.running = nil, false
	s.saveLocked()
	s.publishLocked()
}
