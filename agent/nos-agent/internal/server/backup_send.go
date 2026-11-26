package server

// POST /v1/backup/send replicates one read-only Btrfs snapshot (made by
// /v1/snapshot/create) with btrfs send, to another machine over SSH or to a
// local folder (e.g. a USB disk), incrementally when a parent snapshot is
// given. It runs as the transient unit nos-backup-<job>, so it survives
// restarts and can take hours; GET /v1/backup/status?job= reports it.
// Arguments reach the shell only as positional parameters, never spliced
// into the command text, and the remote path is restricted to safe
// characters because ssh runs it through the remote shell.

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	reBackupJob  = regexp.MustCompile(`^[a-z0-9-]{8,64}$`)
	reSnapPath   = regexp.MustCompile(`^/[A-Za-z0-9._/-]+/\.snapshots/\d{8}-\d{6}-[a-z0-9-]{1,64}$`)
	reHost       = regexp.MustCompile(`^[A-Za-z0-9.:-]{1,253}$`)
	reSSHUser    = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
	reSafePath   = regexp.MustCompile(`^/[A-Za-z0-9._/-]{0,254}$`)
	reKeyName    = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	backupKeyDir = "/etc/nos/backup/keys"
)

// BackupSendRequest is one replication.
type BackupSendRequest struct {
	Job      string `json:"job"`
	Snapshot string `json:"snapshot"`
	Parent   string `json:"parent,omitempty"` // earlier snapshot already on the target
	Target   string `json:"target"`           // ssh, local
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	User     string `json:"user,omitempty"`
	Path     string `json:"path"`          // where the snapshot is received
	Key      string `json:"key,omitempty"` // file name in /etc/nos/backup/keys
}

func (r BackupSendRequest) validate() string {
	switch {
	case !reBackupJob.MatchString(r.Job):
		return "invalid job id"
	case !reSnapPath.MatchString(r.Snapshot) || strings.Contains(r.Snapshot, ".."):
		return "snapshot must be a snapshot made by nOS"
	case r.Parent != "" && (!reSnapPath.MatchString(r.Parent) || filepath.Dir(r.Parent) != filepath.Dir(r.Snapshot)):
		return "parent must be an earlier snapshot of the same folder"
	case !reSafePath.MatchString(r.Path) || strings.Contains(r.Path, ".."):
		return "invalid target path"
	}
	switch r.Target {
	case "local":
		return ""
	case "ssh":
		switch {
		case !reHost.MatchString(r.Host) || strings.HasPrefix(r.Host, "-"):
			return "invalid host"
		case !reSSHUser.MatchString(r.User):
			return "invalid user"
		case r.Port < 1 || r.Port > 65535:
			return "invalid port"
		case r.Key != "" && !reKeyName.MatchString(r.Key):
			return "invalid key name"
		}
		return ""
	}
	return "target must be ssh or local"
}

// backupCommand is the systemd-run command line for req. The pipeline is
// fixed text; the values are bash's positional parameters $1...
func backupCommand(req BackupSendRequest) []string {
	unit := "nos-backup-" + req.Job
	head := []string{"systemd-run", "--unit=" + unit, "--description=nOS backup " + req.Job, "--no-block",
		"--remain-after-exit", "--property=Type=oneshot", "--", "/bin/bash", "-o", "pipefail", "-c"}
	send := `p=(); [ -n "$2" ] && p=(-p "$2"); btrfs send "${p[@]}" "$1"`
	if req.Target == "local" {
		return append(head, send+` | btrfs receive "$3"`, "nos-backup", req.Snapshot, req.Parent, req.Path)
	}
	ssh := `ssh -o BatchMode=yes -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=/var/lib/nos/backup/known_hosts -p "$4"`
	key := ""
	if req.Key != "" {
		key = filepath.Join(backupKeyDir, req.Key)
	}
	return append(head,
		send+` | `+ssh+` ${6:+-i "$6"} -- "$5" "btrfs receive $3"`,
		"nos-backup", req.Snapshot, req.Parent, req.Path, strconv.Itoa(req.Port), req.User+"@"+req.Host, key)
}

func handleBackupSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req BackupSendRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if msg := req.validate(); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if fi, err := os.Stat(req.Snapshot); err != nil || !fi.IsDir() {
		writeErr(w, http.StatusBadRequest, "snapshot not found")
		return
	}
	if req.Target == "local" {
		if err := os.MkdirAll(req.Path, 0o750); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	_ = os.MkdirAll(backupKeyDir, 0o700)
	cmd := backupCommand(req)
	if out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput(); err != nil {
		writeErr(w, http.StatusInternalServerError, "systemd-run: "+strings.TrimSpace(string(out)))
		return
	}
	logAuthPriv("backup.send job=" + req.Job + " snapshot=" + req.Snapshot + " target=" + req.Target)
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "unit": "nos-backup-" + req.Job})
}

// handleBackupStatus reports a replication unit (state, exit status, log).
func handleBackupStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	job := r.URL.Query().Get("job")
	if !reBackupJob.MatchString(job) {
		writeErr(w, http.StatusBadRequest, "invalid job id")
		return
	}
	st := readUnit("nos-backup-" + job + ".service")
	if st.invocation != "" {
		out, _ := exec.Command("journalctl", "_SYSTEMD_INVOCATION_ID="+st.invocation, "-o", "cat", "--no-pager", "-n", "100").Output()
		for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
			if line != "" {
				st.Log = append(st.Log, line)
			}
		}
	}
	writeJSON(w, http.StatusOK, st)
}
