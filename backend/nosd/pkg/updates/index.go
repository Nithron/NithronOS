package updates

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"nithronos/backend/nosd/internal/fsatomic"
)

type SnapshotRef struct {
	TargetID   string `json:"target_id"`
	TargetPath string `json:"target_path"`
	Type       string `json:"type"` // btrfs|tar
	SnapshotID string `json:"snapshot_id"`
	Location   string `json:"location"`
}

type Transaction struct {
	TxID     string        `json:"tx_id"`
	Time     time.Time     `json:"time"`
	Targets  []SnapshotRef `json:"targets"`
	Packages []string      `json:"packages"`
	Result   string        `json:"result"`
}

type Index struct {
	Transactions []Transaction `json:"transactions"`
}

func defaultIndexPath() string {
	// Dev default if present
	dev := filepath.Clean("./devdata/snapshots-index.json")
	if _, err := os.Stat(dev); err == nil {
		return dev
	}
	return "/var/lib/nos/snapshots/index.json"
}

func Load(path string) (Index, error) {
	if path == "" {
		path = defaultIndexPath()
	}
	var idx Index
	ok, err := fsatomic.LoadJSON(path, &idx)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Index{}, nil
		}
		return Index{}, err
	}
	if !ok {
		return Index{}, nil
	}
	return idx, nil
}

func Save(path string, idx Index) error {
	if path == "" {
		path = defaultIndexPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// Serialize across processes via .lock and persist with durability (0644 public metadata)
	return fsatomic.WithLock(path, func() error {
		return fsatomic.SaveJSON(path, idx, fs.FileMode(0o644))
	})
}
