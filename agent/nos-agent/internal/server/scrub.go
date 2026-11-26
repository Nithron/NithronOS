package server

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// handleBtrfsScrubStart starts a scrub in the background (it can run for
// hours; it used to run with -B, which held the request until it finished).
func handleBtrfsScrubStart(w http.ResponseWriter, r *http.Request) {
	btrfsMountOp(w, r, "scrub start", func(mount string) *exec.Cmd {
		return exec.Command("btrfs", "scrub", "start", mount)
	})
}

// handleBtrfsScrubCancel stops a running scrub.
func handleBtrfsScrubCancel(w http.ResponseWriter, r *http.Request) {
	btrfsMountOp(w, r, "scrub cancel", func(mount string) *exec.Cmd {
		return exec.Command("btrfs", "scrub", "cancel", mount)
	})
}

// handleBtrfsBalanceStart starts a filtered balance in the background: data
// and metadata chunks at most half full are compacted, which frees space
// without rewriting the whole pool.
func handleBtrfsBalanceStart(w http.ResponseWriter, r *http.Request) {
	btrfsMountOp(w, r, "balance start", func(mount string) *exec.Cmd {
		return exec.Command("btrfs", "balance", "start", "--bg", "-dusage=50", "-musage=50", mount)
	})
}

// handleBtrfsBalanceCancel stops a running balance (after the current chunk).
func handleBtrfsBalanceCancel(w http.ResponseWriter, r *http.Request) {
	btrfsMountOp(w, r, "balance cancel", func(mount string) *exec.Cmd {
		return exec.Command("btrfs", "balance", "cancel", mount)
	})
}

// btrfsMountOp runs a btrfs command on {"mount": ...}, which must be the
// mount point of a Btrfs file system. "Already running" and "not running"
// answer 409.
func btrfsMountOp(w http.ResponseWriter, r *http.Request, what string, cmd func(string) *exec.Cmd) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Mount string `json:"mount"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !isBtrfsMount(body.Mount) {
		writeErr(w, http.StatusBadRequest, "not a Btrfs mount point")
		return
	}
	out, err := cmd(body.Mount).CombinedOutput()
	msg := strings.TrimSpace(string(out))
	if err != nil {
		low := strings.ToLower(msg)
		if strings.Contains(low, "already running") || strings.Contains(low, "in progress") ||
			strings.Contains(low, "not running") || strings.Contains(low, "no scrub running") {
			writeErr(w, http.StatusConflict, msg)
			return
		}
		writeErr(w, http.StatusInternalServerError, msg)
		return
	}
	logAuthPriv("btrfs." + strings.ReplaceAll(what, " ", ".") + " mount=" + body.Mount)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "output": msg})
}

// mountsFile is read to tell Btrfs mount points; tests replace it.
var mountsFile = "/proc/self/mounts"

// isBtrfsMount reports whether path is exactly the mount point of a Btrfs
// file system.
func isBtrfsMount(path string) bool {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return false
	}
	data, err := os.ReadFile(mountsFile)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		// /proc/mounts escapes spaces as \040.
		if len(f) >= 3 && f[2] == "btrfs" && strings.ReplaceAll(f[1], `\040`, " ") == path {
			return true
		}
	}
	return false
}

func handleBtrfsScrubStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	mount := r.URL.Query().Get("mount")
	if !isBtrfsMount(mount) {
		writeErr(w, http.StatusBadRequest, "not a Btrfs mount point")
		return
	}
	out, err := exec.Command("btrfs", "scrub", "status", mount).CombinedOutput()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, strings.TrimSpace(string(out)))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": string(out)})
}

func handleBtrfsCheckRepair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Path     string `json:"path"`
		Advanced bool   `json:"advanced"`
		Confirm  string `json:"confirm"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !body.Advanced || strings.ToUpper(strings.TrimSpace(body.Confirm)) != "REPAIR" {
		writeErr(w, http.StatusForbidden, "repair requires advanced=true and confirm=REPAIR")
		return
	}
	if strings.TrimSpace(body.Path) == "" || !filepath.IsAbs(body.Path) {
		writeErr(w, http.StatusBadRequest, "absolute path required")
		return
	}
	out, err := exec.Command("btrfs", "check", "--repair", body.Path).CombinedOutput()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, strings.TrimSpace(string(out)))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "output": string(out)})
}

// handleBtrfsSubvolumes lists the subvolumes of a mounted Btrfs pool.
func handleBtrfsSubvolumes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	mount := r.URL.Query().Get("mount")
	if !isBtrfsMount(mount) {
		writeErr(w, http.StatusBadRequest, "not a Btrfs mount point")
		return
	}
	out, err := exec.Command("btrfs", "subvolume", "list", mount).CombinedOutput()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, strings.TrimSpace(string(out)))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"subvolumes": parseSubvolumeList(string(out))})
}

type subvolume struct {
	ID       string `json:"id"`
	Gen      string `json:"gen"`
	TopLevel string `json:"top_level"`
	Path     string `json:"path"`
}

// parseSubvolumeList reads `btrfs subvolume list` lines:
// "ID 256 gen 10 top level 5 path data".
func parseSubvolumeList(out string) []subvolume {
	list := []subvolume{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 9 || f[0] != "ID" || f[2] != "gen" || f[4] != "top" || f[7] != "path" {
			continue
		}
		// The path is the rest of the line (it may contain spaces).
		path := strings.TrimSpace(line[strings.Index(line, " path ")+len(" path "):])
		list = append(list, subvolume{ID: f[1], Gen: f[3], TopLevel: f[6], Path: path})
	}
	return list
}
