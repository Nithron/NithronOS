package server

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"nos/backend/nosd/internal/config"
	"nos/backend/nosd/internal/fsatomic"
	"nos/backend/nosd/pkg/agentclient"
	"nos/backend/nosd/pkg/httpx"
)

type discoveredPool struct {
	Label   string   `json:"label"`
	UUID    string   `json:"uuid"`
	Devices []string `json:"devices"`
	Mount   string   `json:"mount,omitempty"`
}

// GET /api/v1/pools/discover
func handlePoolsDiscover(w http.ResponseWriter, r *http.Request) {
	list := discoverBtrfs()
	writeJSON(w, list)
}

func discoverBtrfs() []discoveredPool {
	out := []discoveredPool{}
	// Try `btrfs filesystem show --raw`
	if _, err := exec.LookPath("btrfs"); err == nil {
		if b, err := exec.Command("btrfs", "filesystem", "show", "--raw").CombinedOutput(); err == nil {
			out = parseBtrfsShow(string(b))
		}
	}
	// Best-effort mountpoint detection via /proc/mounts
	mounts := map[string]string{}
	if f, err := os.Open("/proc/mounts"); err == nil {
		scan := bufio.NewScanner(f)
		for scan.Scan() {
			line := scan.Text()
			if strings.Contains(line, " btrfs ") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					mounts[parts[0]] = parts[1]
				}
			}
		}
		_ = f.Close()
	}
	for i := range out {
		for _, d := range out[i].Devices {
			if m, ok := mounts[d]; ok {
				out[i].Mount = m
				break
			}
		}
	}
	return out
}

var reShow = regexp.MustCompile(`(?m)Label:\s+'([^']*)'.*?uuid:\s+([0-9a-fA-F-]+)`)

func parseBtrfsShow(s string) []discoveredPool {
	pools := []discoveredPool{}
	blocks := strings.Split(s, "\n\n")
	for _, blk := range blocks {
		blk = strings.TrimSpace(blk)
		if blk == "" {
			continue
		}
		m := reShow.FindStringSubmatch(blk)
		if len(m) < 3 {
			continue
		}
		label := strings.TrimSpace(m[1])
		uuid := strings.TrimSpace(m[2])
		devs := []string{}
		for _, ln := range strings.Split(blk, "\n") {
			ln = strings.TrimSpace(ln)
			if strings.HasPrefix(ln, "\tdevid") && strings.Contains(ln, " path ") {
				parts := strings.Split(ln, " path ")
				if len(parts) == 2 {
					devs = append(devs, strings.TrimSpace(parts[1]))
				}
			}
		}
		pools = append(pools, discoveredPool{Label: label, UUID: uuid, Devices: devs})
	}
	return pools
}

// POST /api/v1/pools/import
func handlePoolsImport(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			UUID         string `json:"uuid"`
			Label        string `json:"label"`
			Mountpoint   string `json:"mountpoint"`
			MountOptions string `json:"mountOptions"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		// Accept either UUID or label; derive mountpoint if not provided
		uuid := strings.TrimSpace(body.UUID)
		label := strings.TrimSpace(body.Label)
		if uuid == "" && label == "" {
			httpx.WriteError(w, http.StatusBadRequest, "uuid or label required")
			return
		}
		if strings.TrimSpace(body.Mountpoint) == "" {
			name := label
			if name == "" {
				name = uuid
			}
			if name == "" {
				name = "pool"
			}
			body.Mountpoint = filepath.Join("/mnt", strings.ReplaceAll(strings.ToLower(name), " ", "_"))
		} else if !filepath.IsAbs(body.Mountpoint) {
			httpx.WriteError(w, http.StatusBadRequest, "mountpoint must be absolute or omitted")
			return
		}
		// Busy check: use UUID as pool ID key
		if cur := currentPoolTx(body.UUID); cur != "" {
			httpx.WriteError(w, http.StatusConflict, `{"error":{"code":"pool.busy","txId":"`+cur+`"}}`)
			return
		}
		client := agentclient.New("/run/nos-agent.sock")
		// mkdir -p mountpoint
		_ = client.PostJSON(r.Context(), "/v1/fs/mkdir", map[string]any{"path": body.Mountpoint, "mode": "0755"}, nil)
		// choose options
		opts := strings.TrimSpace(body.MountOptions)
		if opts == "" {
			opts = computeDefaultMountOpts(r.Context(), []string{"/dev/disk/by-uuid/" + body.UUID})
		}
		// ensure fstab entry and mount
		line := "UUID=" + body.UUID + " " + body.Mountpoint + " btrfs " + opts + " 0 0"
		_ = client.PostJSON(r.Context(), "/v1/fstab/ensure", map[string]any{"line": line}, nil)
		// mount now
		_ = client.PostJSON(r.Context(), "/v1/run", map[string]any{"steps": []map[string]any{{"cmd": "mount", "args": []string{"-t", "btrfs", "-o", opts, "UUID=" + body.UUID, body.Mountpoint}}}}, nil)
		// ensure subvol structure
		for _, sv := range []string{"data", "snaps", "apps"} {
			_ = client.PostJSON(r.Context(), "/v1/run", map[string]any{"steps": []map[string]any{{"cmd": "btrfs", "args": []string{"subvolume", "create", filepath.Join(body.Mountpoint, sv)}}}}, nil)
		}
		// persist pool record best-effort
		type PoolRecord struct {
			Name, Mount  string
			Devices      []string
			MountOptions string
		}
		rec := PoolRecord{Name: body.Label, Mount: body.Mountpoint, Devices: []string{}, MountOptions: opts}
		_ = fsatomic.WithLock(filepath.Join(cfg.EtcDir, "nos", "pools.json"), func() error {
			var list []PoolRecord
			_, _ = fsatomic.LoadJSON(filepath.Join(cfg.EtcDir, "nos", "pools.json"), &list)
			list = append(list, rec)
			return fsatomic.SaveJSON(filepath.Join(cfg.EtcDir, "nos", "pools.json"), list, 0o600)
		})
		writeJSON(w, map[string]any{"ok": true})
	}
}
