package server

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"
)

// Rolling an update back: before an upgrade nosd asks the agent to keep the
// .deb of every package's installed version (/v1/updates/keep); a rollback
// reinstalls them (/v1/updates/revert) in the systemd unit nos-update-revert,
// which then restarts nosd so it reads the configuration nosd restored just
// before. /v1/updates/health is what nosd checks after an upgrade.

const revertUnit = "nos-update-revert.service"

var (
	// keptDebsDir holds one directory of .debs per update record.
	keptDebsDir = "/var/lib/nos/update-debs"
	aptArchive  = "/var/cache/apt/archives"
)

var (
	reAptVersion = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.+~:-]{0,99}$`)
	reRecordID   = regexp.MustCompile(`^[a-f0-9-]{8,64}$`)
)

// KeptPackage is one installed package whose .deb is to be kept.
type KeptPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Error   string `json:"error,omitempty"`
}

type updatesKeepRequest struct {
	ID       string        `json:"id"`
	Packages []KeptPackage `json:"packages"`
}

func validKept(id string, pkgs []KeptPackage) error {
	if !reRecordID.MatchString(id) {
		return fmt.Errorf("invalid id")
	}
	for _, p := range pkgs {
		if !reAptPackage.MatchString(p.Name) || !reAptVersion.MatchString(p.Version) {
			return fmt.Errorf("invalid package %q %q", p.Name, p.Version)
		}
	}
	return nil
}

// debFileName is how apt names a cached .deb: the epoch's colon is %3a.
func debFileName(name, version, arch string) string {
	return name + "_" + strings.ReplaceAll(version, ":", "%3a") + "_" + arch + ".deb"
}

// keepCommand runs a command in dir with a timeout (vars so tests replace them).
var keepCommand = func(dir string, timeout time.Duration, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %v: %s", name, err, strings.TrimSpace(lastLines(string(out), 3)))
	}
	return nil
}

var installedArch = func(name string) string {
	out, err := exec.Command("dpkg-query", "-W", "-f=${Architecture}", name).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " / ")
}

// keepDeb puts the .deb of name=version in dir: apt's cache first, then the
// package repository, then rebuilt from the installed files (dpkg-repack).
func keepDeb(dir string, p KeptPackage) error {
	arch := installedArch(p.Name)
	if arch == "" {
		return fmt.Errorf("not installed")
	}
	file := debFileName(p.Name, p.Version, arch)
	if fileExists(filepath.Join(dir, file)) {
		return nil
	}
	if b, err := os.ReadFile(filepath.Join(aptArchive, file)); err == nil {
		return os.WriteFile(filepath.Join(dir, file), b, 0o640)
	}
	errDownload := keepCommand(dir, 5*time.Minute, "apt-get", "download", p.Name+"="+p.Version)
	if errDownload == nil {
		return nil
	}
	if err := keepCommand(dir, 10*time.Minute, "dpkg-repack", p.Name); err != nil {
		return fmt.Errorf("not in apt's cache, %v; %v", errDownload, err)
	}
	return nil
}

func handleUpdatesKeep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if runtime.GOOS == "windows" {
		writeErr(w, http.StatusNotImplemented, "not supported on windows")
		return
	}
	var req updatesKeepRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validKept(req.ID, req.Packages); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	dir := filepath.Join(keptDebsDir, req.ID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	kept, missing := []KeptPackage{}, []KeptPackage{}
	for _, p := range req.Packages {
		if err := keepDeb(dir, p); err != nil {
			p.Error = err.Error()
			missing = append(missing, p)
			continue
		}
		kept = append(kept, p)
	}
	logAuthPriv(fmt.Sprintf("updates.keep id=%s kept=%d missing=%d", req.ID, len(kept), len(missing)))
	writeJSON(w, http.StatusOK, map[string]any{"kept": kept, "missing": missing})
}

type updatesIDRequest struct {
	ID string `json:"id"`
}

func decodeID(w http.ResponseWriter, r *http.Request) (string, bool) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return "", false
	}
	var req updatesIDRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil || !reRecordID.MatchString(req.ID) {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return "", false
	}
	return req.ID, true
}

// handleUpdatesKeepDelete removes a record's kept .debs (when nosd prunes it).
func handleUpdatesKeepDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := decodeID(w, r)
	if !ok {
		return
	}
	if err := os.RemoveAll(filepath.Join(keptDebsDir, id)); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// revertScript finishes any half-done dpkg run, reinstalls the kept .debs
// (allowing downgrades, keeping local configuration files), and restarts nosd
// whatever happened, so it reads the configuration restored before.
const revertScript = `dpkg --configure -a
rc=0
if [ "$#" -gt 0 ]; then
  apt-get -y --allow-downgrades -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold install "$@" || rc=$?
fi
systemctl restart --no-block nosd
exit $rc`

func revertCommand(debs []string) []string {
	args := []string{
		"systemd-run", "--unit=nos-update-revert", "--description=nOS update rollback", "--no-block",
		"--remain-after-exit", "--property=Type=oneshot",
		"--setenv=DEBIAN_FRONTEND=noninteractive",
		"--", "/bin/sh", "-c", revertScript, "nos-update-revert",
	}
	return append(args, debs...)
}

// keptDebs lists a record's kept .debs (absolute paths, so apt installs the files).
func keptDebs(id string) []string {
	files, _ := filepath.Glob(filepath.Join(keptDebsDir, id, "*.deb"))
	slices.Sort(files)
	return files
}

func handleUpdatesRevert(w http.ResponseWriter, r *http.Request) {
	id, ok := decodeID(w, r)
	if !ok {
		return
	}
	if runtime.GOOS == "windows" {
		writeErr(w, http.StatusNotImplemented, "not supported on windows")
		return
	}
	if readUpdateUnit().State == "running" || readUnit(revertUnit).State == "running" {
		writeErr(w, http.StatusConflict, "an update or rollback is already running")
		return
	}
	_ = exec.Command("systemctl", "stop", revertUnit).Run()
	_ = exec.Command("systemctl", "reset-failed", revertUnit).Run()
	debs := keptDebs(id)
	cmd := revertCommand(debs)
	if out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput(); err != nil {
		writeErr(w, http.StatusInternalServerError, "systemd-run: "+strings.TrimSpace(string(out)))
		return
	}
	logAuthPriv(fmt.Sprintf("updates.revert started id=%s debs=%d", id, len(debs)))
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "unit": revertUnit, "packages": len(debs)})
}

// handleUpdatesRevertStatus reports the last rollback run, like /v1/updates/status.
func handleUpdatesRevertStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, unitWithLog(revertUnit))
}

// The services an update must leave running, and the page nosd answers
// through Caddy (port 80 proxies /api without redirecting).
var (
	healthUnits = []string{"nosd", "nos-agent", "caddy"}
	healthURL   = "http://127.0.0.1/api/v1/health"
	webIndex    = "/usr/share/nos/web/index.html"
)

var unitActive = func(unit string) string {
	out, _ := exec.Command("systemctl", "is-active", unit).Output()
	return strings.TrimSpace(string(out))
}

var webAnswers = func() error {
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(healthURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s answered %d", healthURL, resp.StatusCode)
	}
	return nil
}

// UpdatesHealth is the check after an upgrade.
type UpdatesHealth struct {
	OK       bool              `json:"ok"`
	Units    map[string]string `json:"units"`
	Problems []string          `json:"problems"`
}

func updatesHealth() UpdatesHealth {
	h := UpdatesHealth{Units: map[string]string{}, Problems: []string{}}
	for _, u := range healthUnits {
		st := unitActive(u)
		h.Units[u] = st
		if st != "active" {
			h.Problems = append(h.Problems, fmt.Sprintf("%s is %s", u, cmp.Or(st, "unknown")))
		}
	}
	if !fileExists(webIndex) {
		h.Problems = append(h.Problems, "the web interface's files are missing ("+webIndex+")")
	}
	if err := webAnswers(); err != nil {
		h.Problems = append(h.Problems, "the web interface does not answer: "+err.Error())
	}
	h.OK = len(h.Problems) == 0
	return h
}

func handleUpdatesHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, updatesHealth())
}
