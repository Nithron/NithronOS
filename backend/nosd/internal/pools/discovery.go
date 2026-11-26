package pools

import (
	"bufio"
	"context"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"nos/backend/nosd/pkg/shell"
)

// ListPools attempts to discover mounted btrfs filesystems and returns size/usage details.
func ListPools(ctx context.Context) ([]Pool, error) {
	// Find btrfs mounts via lsblk
	res, err := shell.Run(ctx, 3*time.Second, "lsblk", "-J", "-O")
	if err != nil {
		return []Pool{}, nil
	}
	mounts := findBtrfsMounts(res.Stdout)
	pools := []Pool{}
	for _, m := range mounts {
		p := Pool{ID: m, Label: filepath.Base(m), Mount: m}
		// fetch usage
		size, used, free := btrfsUsage(ctx, m)
		p.Size, p.Used, p.Free = size, used, free
		// try to get label/uuid via `btrfs filesystem show`
		label, uuid := btrfsShowForMount(ctx, m)
		if label != "" {
			p.Label = label
		}
		p.UUID = uuid
		// RAID profile is best-effort via `btrfs fi usage`
		p.RAID = btrfsRaidProfile(ctx, m)
		pools = append(pools, p)
	}
	return pools, nil
}

func findBtrfsMounts(lsblkJSON []byte) []string {
	// naive search for '"fstype":"btrfs","mountpoint":"..."'
	out := []string{}
	s := string(lsblkJSON)
	idx := 0
	for {
		i := strings.Index(s[idx:], "\"fstype\":\"btrfs\"")
		if i < 0 {
			break
		}
		i += idx
		// find mountpoint after this
		j := strings.Index(s[i:], "\"mountpoint\":")
		if j < 0 {
			break
		}
		j += i
		k := strings.Index(s[j:], "\"")
		if k < 0 {
			break
		}
		j += k + 1
		k2 := strings.Index(s[j:], "\"")
		if k2 < 0 {
			break
		}
		mount := s[j : j+k2]
		if mount != "" && mount != "null" {
			out = append(out, mount)
		}
		idx = j + k2
	}
	return uniqueStrings(out)
}

func btrfsUsage(ctx context.Context, mount string) (uint64, uint64, uint64) {
	cmd := exec.CommandContext(ctx, "btrfs", "filesystem", "usage", "-b", mount)
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, 0
	}
	var size, used, free uint64
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	re := regexp.MustCompile(`(?i)^(Device size|Used|Free).*?(\d+)`) // crude
	for scanner.Scan() {
		line := scanner.Text()
		m := re.FindStringSubmatch(line)
		if len(m) == 3 {
			v, _ := strconv.ParseUint(m[2], 10, 64)
			switch strings.ToLower(m[1]) {
			case "device size":
				size = v
			case "used":
				used = v
			case "free":
				free = v
			}
		}
	}
	return size, used, free
}

func btrfsRaidProfile(ctx context.Context, mount string) string {
	cmd := exec.CommandContext(ctx, "btrfs", "filesystem", "usage", mount)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(strings.ToLower(line), "metadata,") {
			// line like:  Metadata, RAID1: total=... used=...
			parts := strings.Split(line, ":")
			if len(parts) > 0 {
				left := parts[0]
				segs := strings.Split(left, ",")
				if len(segs) > 1 {
					return strings.TrimSpace(segs[1])
				}
			}
		}
	}
	return ""
}

func btrfsShowForMount(ctx context.Context, mount string) (string, string) {
	cmd := exec.CommandContext(ctx, "btrfs", "filesystem", "show")
	out, err := cmd.Output()
	if err != nil {
		return "", ""
	}
	var currentLabel, currentUUID string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Label:") {
			// Label: 'pool1'  uuid: 1234-...
			// extract between quotes
			if i := strings.Index(line, "'"); i >= 0 {
				if j := strings.Index(line[i+1:], "'"); j >= 0 {
					currentLabel = line[i+1 : i+1+j]
				}
			}
			if k := strings.Index(strings.ToLower(line), "uuid:"); k >= 0 {
				currentUUID = strings.TrimSpace(line[k+5:])
			}
		}
		if strings.Contains(line, mount) {
			return currentLabel, currentUUID
		}
	}
	return currentLabel, currentUUID
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
