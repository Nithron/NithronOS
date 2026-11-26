package pools

import (
	"context"
	"time"

	ipools "nos/backend/nosd/internal/pools"
)

// Root represents an allowed mount root (reserved for future fields)
type Root struct{ Mountpoint string }

// AllowedRoots returns a list of mountpoints discovered from existing pools.
// It reuses the internal pools discovery to find mounted Btrfs filesystems.
func AllowedRoots() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	list, err := ipools.ListPools(ctx)
	if err != nil {
		// graceful default roots
		return []string{"/srv", "/mnt"}, nil
	}
	seen := map[string]struct{}{"/srv": {}, "/mnt": {}}
	roots := []string{"/srv", "/mnt"}
	for _, p := range list {
		if p.Mount == "" {
			continue
		}
		if _, ok := seen[p.Mount]; ok {
			continue
		}
		seen[p.Mount] = struct{}{}
		roots = append(roots, p.Mount)
	}
	return roots, nil
}
