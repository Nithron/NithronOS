package snapdb

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"nithronos/backend/nosd/internal/fsatomic"
)

// SnapshotTarget describes one snapshot created for a target path as part of an update transaction.
type SnapshotTarget struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	Type      string    `json:"type"` // "btrfs" | "tar"
	Location  string    `json:"location"`
	CreatedAt time.Time `json:"created_at"`
}

// UpdateTx records the metadata for an updates apply operation.
type UpdateTx struct {
	TxID       string           `json:"tx_id"`
	StartedAt  time.Time        `json:"started_at"`
	FinishedAt *time.Time       `json:"finished_at,omitempty"`
	Packages   []string         `json:"packages"`
	Reason     string           `json:"reason"` // typically "pre-update"
	Targets    []SnapshotTarget `json:"targets"`
	Success    *bool            `json:"success,omitempty"`
	Notes      string           `json:"notes,omitempty"`
}

// baseDir returns the directory to store the snapshots index under.
// Order of precedence:
// 1) NOS_SNAPDB_DIR env var if set
// 2) default /var/lib/nos/snapshots
func baseDir() string {
	if v := strings.TrimSpace(os.Getenv("NOS_SNAPDB_DIR")); v != "" {
		return filepath.Clean(v)
	}
	return "/var/lib/nos/snapshots"
}

// pathIndex returns the full file path to the index.json.
func pathIndex() string { return filepath.Join(baseDir(), "index.json") }

// EnsureDir ensures the database directory exists with 0755 perms.
func EnsureDir() error {
	return os.MkdirAll(baseDir(), 0o755)
}

// Append adds an UpdateTx to the JSON array file atomically with a coarse lock.
func Append(tx UpdateTx) error {
	if err := EnsureDir(); err != nil {
		return err
	}
	// Use fsatomic-level lock on target index path
	path := pathIndex()
	return fsatomic.WithLock(path, func() error {
		idx, err := readAll()
		if err != nil {
			return err
		}
		idx = append(idx, tx)
		return writeAll(idx)
	})
}

// FindByTx returns a transaction by tx_id.
func FindByTx(txID string) (UpdateTx, error) {
	idx, err := readAll()
	if err != nil {
		return UpdateTx{}, err
	}
	for i := len(idx) - 1; i >= 0; i-- {
		if idx[i].TxID == txID {
			return idx[i], nil
		}
	}
	return UpdateTx{}, errors.New("not found")
}

// ListRecent returns up to n most recent transactions ordered by StartedAt desc.
func ListRecent(n int) ([]UpdateTx, error) {
	idx, err := readAll()
	if err != nil {
		return nil, err
	}
	sort.Slice(idx, func(i, j int) bool { return idx[i].StartedAt.After(idx[j].StartedAt) })
	if n <= 0 || n >= len(idx) {
		return idx, nil
	}
	return idx[:n], nil
}

// Internal helpers

func readAll() ([]UpdateTx, error) {
	path := pathIndex()
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []UpdateTx{}, nil
		}
		return nil, err
	}
	var out []UpdateTx
	if len(b) == 0 {
		return []UpdateTx{}, nil
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func writeAll(items []UpdateTx) error {
	path := pathIndex()
	// Save with 0644 as this is non-sensitive metadata
	return fsatomic.SaveJSON(path, items, fs.FileMode(0o644))
}
