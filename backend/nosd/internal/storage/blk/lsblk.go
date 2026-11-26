package blk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"nos/backend/nosd/pkg/agentclient"
	"nos/backend/nosd/pkg/shell"
)

var ErrNoLsblk = errors.New("lsblk not found")

// ListCandidates runs lsblk and returns filtered candidate disks.
func ListCandidates(ctx context.Context) ([]Device, error) {
	if _, err := exec.LookPath("lsblk"); err != nil {
		return []Device{}, ErrNoLsblk
	}
	var tree rawTree
	// Prefer agent (restricted allowlist) when available
	if runtime.GOOS != "windows" {
		if _, err := os.Stat("/run/nos-agent.sock"); err == nil {
			client := agentclient.New("/run/nos-agent.sock")
			if err := client.GetJSON(ctx, "/v1/storage/lsblk", &tree); err == nil {
				goto HAVE_TREE
			}
		}
	}
	{
		args := []string{"--bytes", "--json", "-O", "-o", "NAME,KNAME,PATH,SIZE,ROTA,TYPE,TRAN,VENDOR,MODEL,SERIAL,MOUNTPOINT,FSTYPE,RM"}
		res, err := shell.Run(ctx, 5*time.Second, "lsblk", args...)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(res.Stdout, &tree); err != nil {
			return nil, fmt.Errorf("lsblk json: %w", err)
		}
	}
HAVE_TREE:
	devices := flatten(tree)
	out := []Device{}
	for _, d := range devices {
		if d.Type != "disk" {
			continue
		}
		// accept both SSD and HDD; rota filter will be used by caller if needed
		if d.Removable != nil && *d.Removable {
			continue
		}
		if d.SizeBytes < 8*1024*1024*1024 {
			continue
		}
		// Detect filesystem and btrfs membership flags
		d.BtrfsMember = isBtrfsMember(ctx, d.Path)
		d.LUKS = strings.EqualFold(d.FSType, "crypto_LUKS")
		if d.FSType != "" {
			d.Warnings = append(d.Warnings, "existing-filesystem")
		}
		out = append(out, d)
	}
	return out, nil
}

func flatten(t rawTree) []Device {
	out := []Device{}
	var walk func(rawDevice)
	walk = func(n rawDevice) {
		// Only record top-level disks and parts but we filter later
		out = append(out, Device{
			Name:      n.Name,
			Path:      firstNonEmpty(n.Path, "/dev/"+n.Name),
			SizeBytes: normalizeSize(n.Size),
			Model:     n.Model,
			Serial:    n.Serial,
			Rota:      n.Rota,
			Removable: n.RM,
			Type:      n.Type,
			FSType:    n.FSType,
		})
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, bd := range t.Blockdevices {
		walk(bd)
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func normalizeSize(v any) uint64 {
	switch t := v.(type) {
	case float64:
		if t < 0 {
			return 0
		}
		return uint64(t)
	case int64:
		if t < 0 {
			return 0
		}
		return uint64(t)
	case json.Number:
		n, _ := t.Int64()
		if n < 0 {
			return 0
		}
		return uint64(n)
	default:
		return 0
	}
}

func isBtrfsMember(ctx context.Context, devPath string) bool {
	// `btrfs inspect-internal dump-super` is heavy; prefer `btrfs device scan` + `btrfs fi show`
	if _, err := exec.LookPath("btrfs"); err != nil {
		return false
	}
	// best-effort: `btrfs filesystem show` reports devices and their members
	res, err := shell.Run(ctx, 3*time.Second, "btrfs", "filesystem", "show", "--raw")
	if err != nil {
		return false
	}
	s := string(res.Stdout)
	// lines contain paths like dev_path size state
	return strings.Contains(s, devPath)
}
