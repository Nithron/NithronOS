package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const SocketPath = "/run/nos-agent.sock"

// Start creates the unix socket listener and serves the HTTP API.
func Start() error {
	if err := mustBeRoot(); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(SocketPath), 0o755); err != nil {
		return fmt.Errorf("mkdir socket dir: %w", err)
	}
	_ = os.Remove(SocketPath)

	l, err := net.Listen("unix", SocketPath)
	if err != nil {
		return fmt.Errorf("listen unix: %w", err)
	}
	// Only root and the nos service account may connect: root:nos 0660, plus
	// a peer-credential check on every request (see peercred.go).
	acc := lookupServiceAccount("nos")
	if runtime.GOOS != "windows" {
		if acc.gid >= 0 {
			if err := os.Chown(SocketPath, 0, acc.gid); err != nil {
				// e.g. a unit without CAP_CHOWN; nosd won't be able to connect
				log.Printf("nos-agent: cannot give group nos access to %s: %v", SocketPath, err)
			}
		}
		if err := os.Chmod(SocketPath, 0o660); err != nil {
			return fmt.Errorf("chmod socket: %w", err)
		}
	}

	// Bootstrap: register with nosd on first start (best-effort)
	go func() { _ = registerWithNosd() }()

	// init prometheus registry
	initMetrics()

	srv := &http.Server{
		Handler:           requirePeer(acc, buildMux()),
		ConnContext:       withPeerCred,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.Serve(l)
}

// buildMux constructs the HTTP handler with all routes registered.
func buildMux() http.Handler {
	mux := http.NewServeMux()
	// System configuration endpoint
	mux.HandleFunc("/execute", handleExecute)
	// Storage endpoints
	mux.HandleFunc("/v1/btrfs/create", handleBtrfsCreate)
	mux.HandleFunc("/v1/btrfs/mount", handleBtrfsMount)
	mux.HandleFunc("/v1/btrfs/snapshot", handleBtrfsSnapshot)
	mux.HandleFunc("/v1/btrfs/balance/status", handleBtrfsBalanceStatus)
	mux.HandleFunc("/v1/btrfs/replace/status", handleBtrfsReplaceStatus)
	mux.HandleFunc("/v1/service/reload", handleServiceReload)
	mux.HandleFunc("/v1/app/compose-up", handleComposeUp)
	mux.HandleFunc("/v1/app/compose-down", handleComposeDown)
	mux.HandleFunc("/v1/systemd/install-app", handleSystemdInstall)
	mux.HandleFunc("/v1/firewall/apply", handleFirewallApply)
	mux.HandleFunc("/v1/fs/write", handleFSWrite)
	mux.HandleFunc("/v1/fs/mkdir", handleFSMkdir)
	mux.HandleFunc("/v1/run", handleRun)
	mux.HandleFunc("/v1/fstab/ensure", handleFstabEnsure)
	mux.HandleFunc("/v1/fstab/remove", handleFstabRemove)
	mux.HandleFunc("/v1/crypttab/ensure", handleCrypttabEnsure)
	mux.HandleFunc("/v1/crypttab/remove", handleCrypttabRemove)
	mux.HandleFunc("/v1/btrfs/scrub/start", handleBtrfsScrubStart)
	mux.HandleFunc("/v1/btrfs/scrub/status", handleBtrfsScrubStatus)
	mux.HandleFunc("/v1/btrfs/check-repair", handleBtrfsCheckRepair)
	mux.HandleFunc("/v1/btrfs/usage", handleBtrfsUsage)
	mux.HandleFunc("/v1/smb/user-create", handleSMBUserCreate)
	mux.HandleFunc("/v1/smb/users", handleSMBUsersList)
	mux.HandleFunc("/v1/snapshot/create", handleSnapshotCreate)
	mux.HandleFunc("/v1/snapshot/list", handleSnapshotList)
	mux.HandleFunc("/v1/snapshot/rollback", handleSnapshotRollback)
	mux.HandleFunc("/v1/updates/plan", handleUpdatesPlan)
	mux.HandleFunc("/v1/updates/apply", handleUpdatesApply)
	mux.HandleFunc("/v1/snapshot/prune", handleSnapshotPrune)
	mux.HandleFunc("/v1/storage/lsblk", handleStorageLsblk)
	mux.HandleFunc("/v1/smart", handleSmartSummary)
	// Prometheus metrics on the same unix socket
	mux.Handle("/metrics", metricsHandler())
	return mux
}

// Registration with nosd using bootstrap token on disk
func registerWithNosd() error {
	// Read bootstrap token
	bootTok, err := os.ReadFile("/etc/nos/agent-token")
	if err != nil || len(bootTok) == 0 {
		return err
	}
	// Identify
	node, _ := os.Hostname()
	arch := runtime.GOARCH
	osname := runtime.GOOS
	payload := map[string]string{
		"token": strings.TrimSpace(string(bootTok)),
		"node":  node,
		"arch":  arch,
		"os":    osname,
	}
	b, _ := json.Marshal(payload)
	// nosd default bind is 127.0.0.1:9000
	req, err := http.NewRequest("POST", "http://127.0.0.1:9000/api/v1/agents/register", strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if _, copyErr := io.Copy(io.Discard, resp.Body); copyErr != nil {
			return fmt.Errorf("register status %d and read error: %w", resp.StatusCode, copyErr)
		}
		return fmt.Errorf("register status %d", resp.StatusCode)
	}
	var out struct{ ID, Token string }
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if out.Token == "" {
		return fmt.Errorf("no token returned")
	}
	// Persist per-agent token for future calls
	_ = os.MkdirAll("/var/lib/nos", 0o755)
	return os.WriteFile("/var/lib/nos/agent-auth.json", []byte(fmt.Sprintf("{\n\t\"id\": \"%s\",\n\t\"token\": \"%s\"\n}\n", out.ID, out.Token)), 0o600)
}

type PlanResponse struct {
	Plan []string `json:"plan"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type BtrfsCreateRequest struct {
	Devices []string `json:"devices"`
	Raid    string   `json:"raid"`
	Label   string   `json:"label"`
	Encrypt bool     `json:"encrypt"`
	DryRun  bool     `json:"dry_run"`
}

func handleBtrfsCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req BtrfsCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(req.Devices) == 0 {
		writeErr(w, http.StatusBadRequest, "devices required")
		return
	}
	allowedRaids := map[string]bool{"single": true, "raid1": true, "raid10": true}
	if req.Raid == "" {
		req.Raid = "single"
	}
	if !allowedRaids[strings.ToLower(req.Raid)] {
		writeErr(w, http.StatusBadRequest, "invalid raid")
		return
	}
	if req.Label == "" {
		req.Label = "pool"
	}

	plan := []string{}
	if req.Encrypt {
		for idx, dev := range req.Devices {
			name := fmt.Sprintf("nos%d", idx)
			plan = append(plan,
				fmt.Sprintf("cryptsetup luksFormat %s", shellQuote(dev)),
				fmt.Sprintf("cryptsetup open %s %s", shellQuote(dev), shellQuote(name)),
			)
		}
		mapped := []string{}
		for idx := range req.Devices {
			mapped = append(mapped, "/dev/mapper/"+fmt.Sprintf("nos%d", idx))
		}
		plan = append(plan, mkfsBtrfsCommand(req.Label, req.Raid, mapped...))
	} else {
		plan = append(plan, mkfsBtrfsCommand(req.Label, req.Raid, req.Devices...))
	}

	if req.DryRun || runtime.GOOS == "windows" {
		writeJSON(w, http.StatusOK, PlanResponse{Plan: plan})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	for _, cmdline := range plan {
		parts := strings.Fields(cmdline)
		if len(parts) == 0 {
			continue
		}
		cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, fmt.Sprintf("%s: %s", err, string(out)))
			return
		}
	}
	writeJSON(w, http.StatusOK, PlanResponse{Plan: plan})
}

type ServiceReloadRequest struct {
	Name string `json:"name"`
}

func handleServiceReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req ServiceReloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	switch strings.ToLower(req.Name) {
	case "smb", "smbd":
		cmd := exec.Command("systemctl", "reload", "smbd")
		if out, err := cmd.CombinedOutput(); err != nil {
			writeErr(w, http.StatusInternalServerError, fmt.Sprintf("reload smbd failed: %s", string(out)))
			return
		}
		writeJSON(w, http.StatusOK, PlanResponse{Plan: []string{"systemctl reload smbd"}})
		return
	case "nfs":
		cmd := exec.Command("exportfs", "-ra")
		if out, err := cmd.CombinedOutput(); err != nil {
			writeErr(w, http.StatusInternalServerError, fmt.Sprintf("exportfs -ra failed: %s", string(out)))
			return
		}
		writeJSON(w, http.StatusOK, PlanResponse{Plan: []string{"exportfs -ra"}})
		return
	default:
		writeErr(w, http.StatusBadRequest, "service not allowed")
		return
	}
}

func mkfsBtrfsCommand(label, raid string, devices ...string) string {
	args := []string{"mkfs.btrfs", "-L", label}
	if raid != "single" {
		args = append(args, "-m", raid, "-d", raid)
	}
	args = append(args, devices...)
	return strings.Join(quoteAll(args), " ")
}

func quoteAll(items []string) []string {
	res := make([]string, len(items))
	for i, v := range items {
		res[i] = shellQuote(v)
	}
	return res
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: msg})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// umask and root checks are handled in OS-specific files

// Mount
type BtrfsMountRequest struct {
	Target       string `json:"target"`
	UUIDOrDevice string `json:"uuid_or_device"`
}

func handleBtrfsMount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if runtime.GOOS == "windows" {
		writeErr(w, http.StatusNotImplemented, "not supported on windows")
		return
	}
	var req BtrfsMountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Target == "" || req.UUIDOrDevice == "" {
		writeErr(w, http.StatusBadRequest, "missing fields")
		return
	}
	_ = os.MkdirAll(req.Target, 0o755)
	args := []string{"-t", "btrfs", "-o", "noatime,compress=zstd:3", req.UUIDOrDevice, req.Target}
	cmd := exec.Command("mount", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("mount failed: %s", string(out)))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

// Snapshot
type BtrfsSnapshotRequest struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

func handleBtrfsSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if runtime.GOOS == "windows" {
		writeErr(w, http.StatusNotImplemented, "not supported on windows")
		return
	}
	var req BtrfsSnapshotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Path == "" || req.Name == "" {
		writeErr(w, http.StatusBadRequest, "missing fields")
		return
	}
	snapDir := filepath.Join(req.Path, ".snapshots")
	_ = os.MkdirAll(snapDir, 0o755)
	target := filepath.Join(snapDir, req.Name)
	cmd := exec.Command("btrfs", "subvolume", "snapshot", "-r", req.Path, target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("snapshot failed: %s", string(out)))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}
