package server

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// reSnapshotID is what handleSnapshotCreate names snapshots:
// <yyyymmdd>-<hhmmss>-<reason slug>.
var reSnapshotID = regexp.MustCompile(`^\d{8}-\d{6}-[a-z0-9-]{1,64}$`)

// handleSnapshotDelete deletes one snapshot made by /v1/snapshot/create:
// {path, snapshot_id, type}. Only those locations can be named.
func handleSnapshotDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Path       string `json:"path"`
		SnapshotID string `json:"snapshot_id"`
		Type       string `json:"type"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !filepath.IsAbs(req.Path) || filepath.Clean(req.Path) != req.Path || !reSnapshotID.MatchString(req.SnapshotID) {
		writeErr(w, http.StatusBadRequest, "invalid path or snapshot id")
		return
	}
	switch strings.ToLower(req.Type) {
	case "tar":
		archive := filepath.Join(snapshotsTarDirForPath(req.Path), req.SnapshotID+".tar.gz")
		if err := os.Remove(archive); err != nil && !os.IsNotExist(err) {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
	case "btrfs":
		sub := filepath.Join(req.Path, ".snapshots", req.SnapshotID)
		if _, err := os.Stat(sub); os.IsNotExist(err) {
			break
		}
		if out, err := exec.Command("btrfs", "subvolume", "delete", sub).CombinedOutput(); err != nil {
			writeErr(w, http.StatusInternalServerError, strings.TrimSpace(string(out)))
			return
		}
	default:
		writeErr(w, http.StatusBadRequest, "type must be tar or btrfs")
		return
	}
	logAuthPriv("snapshot deleted path=" + req.Path + " id=" + req.SnapshotID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
