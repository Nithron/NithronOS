package server

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type UpdatesApplyRequest struct {
	Packages []string `json:"packages"`
}

// The upgrade runs as its own systemd unit, not as a child of nos-agent:
// upgrading nos-agent (or nosd) restarts it, and apt-get used to be killed
// with it halfway through dpkg. --remain-after-exit keeps the result
// readable after it finishes; the next run stops and resets it first.
const updateUnit = "nos-update.service"

var reAptPackage = regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]{0,99}$`)

// updateCommand is the systemd-run command line for an upgrade of pkgs (all
// upgradable packages when empty). New dependencies are installed (the
// NithronOS -> nOS rename needs that), nothing is removed, and changed
// configuration files keep the local version.
func updateCommand(pkgs []string) []string {
	args := []string{
		"systemd-run", "--unit=nos-update", "--description=nOS update", "--no-block",
		"--remain-after-exit", "--property=Type=oneshot",
		"--setenv=DEBIAN_FRONTEND=noninteractive",
		"--", "/usr/bin/apt-get", "-y",
		"-o", "Dpkg::Options::=--force-confdef", "-o", "Dpkg::Options::=--force-confold",
	}
	if len(pkgs) > 0 {
		return append(append(args, "install", "--only-upgrade"), pkgs...)
	}
	return append(args, "--with-new-pkgs", "upgrade")
}

func handleUpdatesApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if runtime.GOOS == "windows" {
		writeErr(w, http.StatusNotImplemented, "not supported on windows")
		return
	}
	var req UpdatesApplyRequest
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req)
	for _, p := range req.Packages {
		if !reAptPackage.MatchString(p) {
			writeErr(w, http.StatusBadRequest, "invalid package name")
			return
		}
	}
	if _, err := exec.LookPath("apt-get"); err != nil {
		writeErr(w, http.StatusNotImplemented, "apt-get is not available")
		return
	}
	if st := readUpdateUnit(); st.State == "running" {
		writeErr(w, http.StatusConflict, "an update is already running")
		return
	}
	_ = exec.Command("systemctl", "stop", updateUnit).Run()
	_ = exec.Command("systemctl", "reset-failed", updateUnit).Run()
	cmd := updateCommand(req.Packages)
	if out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput(); err != nil {
		writeErr(w, http.StatusInternalServerError, "systemd-run: "+strings.TrimSpace(string(out)))
		return
	}
	_ = appendUpdateLog(time.Now().UTC(), req.Packages)
	logAuthPriv("updates.apply started packages=" + strings.Join(req.Packages, ","))
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "unit": updateUnit})
}

// UpdateUnitStatus is the state of the last upgrade run.
type UpdateUnitStatus struct {
	State          string   `json:"state"` // none, running, succeeded, failed
	Result         string   `json:"result,omitempty"`
	ExitStatus     int      `json:"exit_status"`
	StartedAt      string   `json:"started_at,omitempty"`
	FinishedAt     string   `json:"finished_at,omitempty"`
	Log            []string `json:"log"`
	RebootRequired bool     `json:"reboot_required"`
	invocation     string
}

// unitStatus maps `systemctl show` properties to a state.
func unitStatus(props map[string]string) UpdateUnitStatus {
	st := UpdateUnitStatus{State: "none", Result: props["Result"], Log: []string{}, invocation: props["InvocationID"]}
	st.ExitStatus, _ = strconv.Atoi(props["ExecMainStatus"])
	if v := props["ExecMainStartTimestamp"]; v != "" && v != "n/a" {
		st.StartedAt = v
	}
	if v := props["ExecMainExitTimestamp"]; v != "" && v != "n/a" {
		st.FinishedAt = v
	}
	switch {
	case props["LoadState"] == "not-found" || props["ActiveState"] == "" || (props["ActiveState"] == "inactive" && st.invocation == ""):
		st.State = "none"
	case props["ActiveState"] == "activating" || props["ActiveState"] == "deactivating":
		st.State = "running"
	case props["ActiveState"] == "failed" || (props["Result"] != "" && props["Result"] != "success"):
		st.State = "failed"
	case props["ActiveState"] == "active":
		st.State = "succeeded"
	default:
		st.State = "none"
	}
	return st
}

func parseShow(out string) map[string]string {
	props := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			props[k] = strings.TrimSpace(v)
		}
	}
	return props
}

func readUpdateUnit() UpdateUnitStatus {
	out, _ := exec.Command("systemctl", "show", updateUnit,
		"--property=LoadState,ActiveState,SubState,Result,ExecMainStatus,InvocationID,ExecMainStartTimestamp,ExecMainExitTimestamp").Output()
	return unitStatus(parseShow(string(out)))
}

// handleUpdatesStatus reports the last upgrade run and its log.
func handleUpdatesStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	st := readUpdateUnit()
	if st.invocation != "" {
		out, _ := exec.Command("journalctl", "_SYSTEMD_INVOCATION_ID="+st.invocation, "-o", "cat", "--no-pager", "-n", "300").Output()
		for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
			if line != "" {
				st.Log = append(st.Log, line)
			}
		}
	}
	st.RebootRequired = fileExists("/var/run/reboot-required")
	writeJSON(w, http.StatusOK, st)
}

func appendUpdateLog(ts time.Time, changed []string) error {
	line := ts.Format(time.RFC3339) + " updates: " + strings.Join(changed, ",") + "\n"
	f, err := os.OpenFile("/var/log/nos-updates.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(line); err != nil {
		return err
	}
	return nil
}
