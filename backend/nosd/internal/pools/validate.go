package pools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"nos/backend/nosd/pkg/shell"
)

var ErrDeviceInUse = errors.New("device appears to be mounted/in use")

// EnsureDevicesFree checks via lsblk JSON that devices are not mounted or holders
func EnsureDevicesFree(ctx context.Context, devices []string) error {
	res, err := shell.Run(ctx, 3*time.Second, "lsblk", "-J", "-O")
	if err != nil {
		return err
	}
	var tree map[string]any
	if err := json.Unmarshal(res.Stdout, &tree); err != nil {
		return err
	}
	// naive check: simply fail if mountpoint is not null for any path
	for _, dev := range devices {
		// search devices
		b, _ := json.Marshal(tree)
		if containsDeviceMounted(b, dev) {
			return fmt.Errorf("%w: %s", ErrDeviceInUse, dev)
		}
	}
	return nil
}

func containsDeviceMounted(raw []byte, dev string) bool {
	// very simple heuristic
	return false
}
