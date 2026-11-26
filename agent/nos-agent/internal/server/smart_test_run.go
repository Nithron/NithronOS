package server

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// /v1/smart/test: POST {device, type} starts a SMART self-test (short, long
// or conveyance; the disk runs it in the background), GET ?device= reports
// whether one is running.

var reSmartRemaining = regexp.MustCompile(`(\d+)% of test remaining`)
var reSmartCompleted = regexp.MustCompile(`\((\d+)% completed\)`)

// isBlockDevice accepts /dev/<name> paths that are block devices.
func isBlockDevice(p string) bool {
	if !validDevicePath(p) || strings.Contains(strings.TrimPrefix(p, "/dev/"), "..") {
		return false
	}
	fi, err := os.Stat(p)
	return err == nil && fi.Mode()&os.ModeDevice != 0 && fi.Mode()&os.ModeCharDevice == 0
}

// smartTestState reads `smartctl -c -l selftest` output (ATA and NVMe).
func smartTestState(out string) (running bool, percentDone int) {
	if m := reSmartRemaining.FindStringSubmatch(out); m != nil && strings.Contains(out, "in progress") {
		left, _ := strconv.Atoi(m[1])
		return true, 100 - left
	}
	if m := reSmartCompleted.FindStringSubmatch(out); m != nil && strings.Contains(out, "in progress") {
		done, _ := strconv.Atoi(m[1])
		return true, done
	}
	return false, 0
}

func handleSmartTest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		dev := r.URL.Query().Get("device")
		if !isBlockDevice(dev) {
			writeErr(w, http.StatusBadRequest, "invalid device")
			return
		}
		out, _ := exec.Command("smartctl", "-c", "-l", "selftest", dev).CombinedOutput()
		running, pct := smartTestState(string(out))
		writeJSON(w, http.StatusOK, map[string]any{"device": dev, "running": running, "percent_done": pct, "raw": string(out)})
	case http.MethodPost:
		var body struct {
			Device string `json:"device"`
			Type   string `json:"type"`
		}
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body)
		if body.Type == "" {
			body.Type = "short"
		}
		if body.Type != "short" && body.Type != "long" && body.Type != "conveyance" {
			writeErr(w, http.StatusBadRequest, "type must be short, long or conveyance")
			return
		}
		if !isBlockDevice(body.Device) {
			writeErr(w, http.StatusBadRequest, "invalid device")
			return
		}
		out, err := exec.Command("smartctl", "-t", body.Type, body.Device).CombinedOutput()
		msg := strings.TrimSpace(string(out))
		if err != nil {
			if strings.Contains(msg, "in progress") {
				writeErr(w, http.StatusConflict, "a self-test is already running")
				return
			}
			writeErr(w, http.StatusInternalServerError, msg)
			return
		}
		logAuthPriv("smart.test " + body.Type + " " + body.Device)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "device": body.Device, "type": body.Type})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
