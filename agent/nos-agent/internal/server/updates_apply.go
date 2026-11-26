package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type UpdatesApplyRequest struct {
	Packages []string `json:"packages"`
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
	_ = json.NewDecoder(r.Body).Decode(&req)
	if _, err := exec.LookPath("apt-get"); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "note": "apt-get not available", "changed": []string{}})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Minute)
	defer cancel()

	// Simulate to compute changed list
	var simArgs []string
	if len(req.Packages) > 0 {
		simArgs = append([]string{"-s", "install"}, req.Packages...)
	} else {
		simArgs = []string{"-s", "upgrade"}
	}
	sim := exec.CommandContext(ctx, "apt-get", simArgs...)
	simOut, _ := sim.CombinedOutput()
	updates := parseAptSimulateUpgrade(simOut)
	changed := make([]string, 0, len(updates))
	for _, u := range updates {
		changed = append(changed, u.Name)
	}

	// Apply
	args := []string{"-y"}
	if len(req.Packages) > 0 {
		args = append(args, "install")
		args = append(args, req.Packages...)
	} else {
		args = append(args, "upgrade")
	}
	cmd := exec.CommandContext(ctx, "apt-get", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, strings.TrimSpace(string(out)))
		return
	}

	// Log the operation concisely
	_ = appendUpdateLog(time.Now().UTC(), changed)

	resp := map[string]any{"ok": true, "changed": changed}
	if fileExists("/var/run/reboot-required") {
		resp["reboot_required"] = true
	}
	writeJSON(w, http.StatusOK, resp)
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
