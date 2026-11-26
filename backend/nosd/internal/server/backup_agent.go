package server

import (
	"context"
	"net/url"
	"time"

	"nos/backend/nosd/pkg/backup"
)

// backupAgent runs the backup module's privileged steps through nos-agent:
// snapshots in a subvolume's .snapshots folder (/v1/snapshot/*) and btrfs
// send to another system (/v1/backup/*). Snapshots are always Btrfs: a
// folder that isn't a subvolume is refused rather than copied with tar.
type backupAgent struct{ agent AgentClient }

// agentTimeout bounds one agent call (sending runs in its own unit).
const agentTimeout = 10 * time.Minute

func (b backupAgent) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), agentTimeout)
}

func (b backupAgent) CreateSnapshot(subvolume, tag string) (backup.SnapshotInfo, error) {
	ctx, cancel := b.ctx()
	defer cancel()
	if tag == "" {
		tag = "backup"
	}
	var resp struct {
		ID       string `json:"id"`
		Location string `json:"location"`
	}
	if err := b.agent.PostJSON(ctx, "/v1/snapshot/create", map[string]any{"path": subvolume, "mode": "btrfs", "reason": tag}, &resp); err != nil {
		return backup.SnapshotInfo{}, err
	}
	return backup.SnapshotInfo{ID: resp.ID, Path: resp.Location, Subvolume: subvolume, CreatedAt: time.Now().UTC()}, nil
}

func (b backupAgent) DeleteSnapshot(snap *backup.Snapshot) error {
	ctx, cancel := b.ctx()
	defer cancel()
	return b.agent.PostJSON(ctx, "/v1/snapshot/delete", map[string]any{"path": snap.Subvolume, "snapshot_id": snap.AgentID, "type": "btrfs"}, nil)
}

func (b backupAgent) RollbackSnapshot(snap *backup.Snapshot, stopServices []string) error {
	ctx, cancel := b.ctx()
	defer cancel()
	return b.agent.PostJSON(ctx, "/v1/snapshot/rollback", map[string]any{
		"path": snap.Subvolume, "snapshot_id": snap.AgentID, "type": "btrfs", "stop_services": stopServices,
	}, nil)
}

func (b backupAgent) Send(req backup.SendRequest) error {
	ctx, cancel := b.ctx()
	defer cancel()
	return b.agent.PostJSON(ctx, "/v1/backup/send", req, nil)
}

func (b backupAgent) SendStatus(job string) (backup.SendStatus, error) {
	ctx, cancel := b.ctx()
	defer cancel()
	var st backup.SendStatus
	err := b.agent.GetJSON(ctx, "/v1/backup/status?job="+url.QueryEscape(job), &st)
	return st, err
}
