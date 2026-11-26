package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"nos/backend/nosd/internal/pools"
	"nos/backend/nosd/pkg/httpx"
	"nos/backend/nosd/pkg/shell"
)

type planCreateRequest struct {
	pools.PoolSpec
	Force        bool   `json:"force"`
	MountOptions string `json:"mountOptions"`
}

func handlePlanCreateV1(w http.ResponseWriter, r *http.Request) {
	var req planCreateRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	spec, err := pools.ValidateSpec(req.PoolSpec)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Guard rails: refuse if any device has fs/signature unless force=true
	warnings := []string{}
	if _, err := exec.LookPath("wipefs"); err == nil {
		for _, d := range spec.Devices {
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			res, _ := shell.Run(ctx, 3*time.Second, "wipefs", "-n", d)
			cancel()
			out := strings.TrimSpace(string(res.Stdout))
			if out != "" {
				warnings = append(warnings, fmt.Sprintf("device %s has existing signatures", d))
			}
		}
	}
	if len(warnings) > 0 && !req.Force {
		httpx.WriteError(w, http.StatusPreconditionFailed, "devices have existing signatures; set force=true to proceed")
		return
	}

	steps := []pools.PlanStep{}
	// 1) wipefs report (non-destructive)
	for idx, d := range spec.Devices {
		steps = append(steps, pools.PlanStep{
			ID:          fmt.Sprintf("wipefs-check-%d", idx+1),
			Description: "report existing filesystem/signatures",
			Command:     fmt.Sprintf("wipefs -n %s", shellQuote(d)),
			Destructive: false,
		})
	}

	// Optional: encryption
	mapped := []string{}
	if spec.Encrypt.Enabled {
		key := spec.Encrypt.Keyfile
		steps = append(steps, pools.PlanStep{ID: "key-ensure", Description: "ensure pool key exists (0600)", Command: fmt.Sprintf("[keyfile] %s", shellQuote(key)), Destructive: false})
		for idx, dev := range spec.Devices {
			name := fmt.Sprintf("luks-%s-%d", spec.Name, idx)
			steps = append(steps, pools.PlanStep{ID: fmt.Sprintf("luks-format-%d", idx+1), Description: "luksFormat device", Command: fmt.Sprintf("cryptsetup luksFormat --type luks2 --batch-mode %s", shellQuote(dev)), Destructive: true})
			steps = append(steps, pools.PlanStep{ID: fmt.Sprintf("luks-open-%d", idx+1), Description: "open LUKS mapping", Command: fmt.Sprintf("cryptsetup open --key-file %s %s %s", shellQuote(key), shellQuote(dev), shellQuote(name)), Destructive: false})
			mapped = append(mapped, filepath.Join("/dev/mapper", name))
		}
	}

	// 2) mkfs.btrfs
	mkTargets := spec.Devices
	if spec.Encrypt.Enabled {
		mkTargets = mapped
	}
	mkfs := []string{"mkfs.btrfs", "-L", spec.Name, "-d", spec.RaidData, "-m", spec.RaidMeta}
	mkfs = append(mkfs, mkTargets...)
	steps = append(steps, pools.PlanStep{
		ID:          "mkfs-btrfs",
		Description: "create btrfs filesystem",
		Command:     strings.Join(quoteAll(mkfs), " "),
		Destructive: true,
	})

	// Compute mount options (default by device mix if not provided)
	opts := strings.TrimSpace(req.MountOptions)
	if opts == "" {
		opts = getDefaultMountOpts(r.Context(), spec.Devices)
	}
	// 3) mount by UUID (discover from first device) or mapper
	steps = append(steps, pools.PlanStep{
		ID:          "mkdir-mountpoint",
		Description: "create mountpoint directory",
		Command:     fmt.Sprintf("mkdir -p %s", shellQuote(spec.Mountpoint)),
		Destructive: false,
	})
	if spec.Encrypt.Enabled {
		steps = append(steps, pools.PlanStep{ID: "mount", Description: "mount btrfs (mapper)", Command: fmt.Sprintf("mount -t btrfs -o %s %s %s", shellQuote(opts), shellQuote(mkTargets[0]), shellQuote(spec.Mountpoint)), Destructive: false})
	} else {
		steps = append(steps, pools.PlanStep{ID: "mount", Description: "mount filesystem by UUID", Command: fmt.Sprintf("mount -t btrfs -o %s UUID=$(blkid -s UUID -o value %s) %s", shellQuote(opts), shellQuote(spec.Devices[0]), shellQuote(spec.Mountpoint)), Destructive: false})
	}

	// 4) default subvolumes
	for _, sv := range []string{"data", "snaps", "apps"} {
		steps = append(steps, pools.PlanStep{
			ID:          "subvol-" + sv,
			Description: "create default subvolume",
			Command:     fmt.Sprintf("btrfs subvolume create %s", shellQuote(strings.TrimRight(spec.Mountpoint, "/")+"/"+sv)),
			Destructive: false,
		})
	}

	// 5) proposed fstab entry and crypttab lines
	fstab := []string{fmt.Sprintf("UUID=<uuid> %s btrfs %s 0 0", spec.Mountpoint, opts)}
	if spec.Encrypt.Enabled {
		fstab[0] = fmt.Sprintf("/dev/mapper/luks-%s-0 %s btrfs %s 0 0", spec.Name, spec.Mountpoint, opts)
		for idx := range mkTargets {
			name := fmt.Sprintf("luks-%s-%d", spec.Name, idx)
			fstab = append(fstab, fmt.Sprintf("[crypttab] %s UUID=<luksUUID-%d> %s luks,discard", name, idx, spec.Encrypt.Keyfile))
		}
	}

	// include options in response
	writeJSON(w, map[string]any{"plan": pools.CreatePlan{Steps: steps}, "fstab": fstab, "warnings": warnings, "mountOptions": opts})
}

// local quote helpers (copy of agent style)
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func quoteAll(items []string) []string {
	res := make([]string, len(items))
	for i, v := range items {
		res[i] = shellQuote(v)
	}
	return res
}
