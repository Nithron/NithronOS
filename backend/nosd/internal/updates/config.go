package updates

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"nos/backend/nosd/internal/fsatomic"
)

// Config is /etc/nos/update/config.json, read whenever it is needed (no
// restart after editing it).
type Config struct {
	// SnapshotRetention is how many update snapshots are kept; older ones
	// are deleted after each successful update.
	SnapshotRetention int `json:"snapshot_retention"`
}

const (
	DefaultRetention = 3
	MaxRetention     = 20
)

func (s *Service) configPath() string {
	return filepath.Join(filepath.Dir(s.channelFile), "config.json")
}

// Config returns the update settings, with defaults for what isn't set.
func (s *Service) Config() Config {
	c := Config{SnapshotRetention: DefaultRetention}
	b, err := os.ReadFile(s.configPath())
	if err != nil {
		return c
	}
	var file Config
	if json.Unmarshal(b, &file) == nil && file.SnapshotRetention >= 1 && file.SnapshotRetention <= MaxRetention {
		c.SnapshotRetention = file.SnapshotRetention
	}
	return c
}

// SetConfig saves the update settings.
func (s *Service) SetConfig(c Config) error {
	if c.SnapshotRetention < 1 || c.SnapshotRetention > MaxRetention {
		return fmt.Errorf("snapshot_retention must be between 1 and %d", MaxRetention)
	}
	if err := os.MkdirAll(filepath.Dir(s.configPath()), 0o750); err != nil {
		return err
	}
	return fsatomic.SaveJSON(s.configPath(), c, 0o640)
}
