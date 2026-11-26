package pools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"nos/backend/nosd/pkg/shell"
)

// ListPools lists the mounted Btrfs filesystems with their size and use. It
// works as the unprivileged nos user: mounts come from findmnt (the mount
// table) and sizes from statfs. (It searched lsblk's JSON for the text
// "fstype":"btrfs" without the space lsblk prints, so it never found a
// pool, and it asked `btrfs filesystem show`, which needs root, for the
// label.) The RAID profile is filled in when the caller can ask nos-agent.
func ListPools(ctx context.Context) ([]Pool, error) {
	res, err := shell.Run(ctx, 3*time.Second, "findmnt", "-J", "-t", "btrfs", "--nofsroot", "-o", "TARGET,SOURCE,UUID,LABEL")
	if err != nil {
		// findmnt exits 1 when nothing matches: no Btrfs mounted.
		return []Pool{}, nil
	}
	return poolsFromFindmnt(res.Stdout, statfsUsage, isDir), nil
}

type findmntJSON struct {
	Filesystems []struct {
		Target string `json:"target"`
		Source string `json:"source"`
		UUID   string `json:"uuid"`
		Label  string `json:"label"`
	} `json:"filesystems"`
}

// poolsFromFindmnt makes one pool per filesystem (a filesystem mounted at
// several places is listed once, at its shortest mount point). Mounts that
// aren't directories (files bind-mounted into a container) are skipped.
func poolsFromFindmnt(out []byte, usage func(string) (uint64, uint64, uint64), dir func(string) bool) []Pool {
	var fm findmntJSON
	if err := json.Unmarshal(out, &fm); err != nil {
		return []Pool{}
	}
	byFS := map[string]Pool{}
	order := []string{}
	for _, f := range fm.Filesystems {
		if f.Target == "" || !dir(f.Target) {
			continue
		}
		key := f.UUID
		if key == "" {
			key = f.Source
		}
		if p, seen := byFS[key]; seen && len(p.Mount) <= len(f.Target) {
			continue
		} else if !seen {
			order = append(order, key)
		}
		label := f.Label
		if label == "" {
			label = filepath.Base(f.Target)
			if f.Target == "/" {
				label = "system"
			}
		}
		// The id goes into URLs (/pools/{id}/...): the UUID, or the label when
		// udev doesn't know it; never the mount path, whose slashes broke
		// every per-pool route.
		id := f.UUID
		if id == "" {
			id = label
		}
		p := Pool{ID: id, Label: label, UUID: f.UUID, Mount: f.Target, Devices: []string{}}
		if f.Source != "" {
			p.Devices = []string{f.Source}
		}
		p.Size, p.Used, p.Free = usage(f.Target)
		byFS[key] = p
	}
	pools := make([]Pool, 0, len(order))
	used := map[string]int{}
	for _, k := range order {
		p := byFS[k]
		if n := used[p.ID]; n > 0 { // two filesystems with the same label
			p.ID = fmt.Sprintf("%s-%d", p.ID, n+1)
		}
		used[byFS[k].ID]++
		pools = append(pools, p)
	}
	return pools
}

func statfsUsage(path string) (size, used, free uint64) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, 0
	}
	bs := uint64(st.Bsize)
	size, free = st.Blocks*bs, st.Bavail*bs
	used = (st.Blocks - st.Bfree) * bs
	return size, used, free
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}
