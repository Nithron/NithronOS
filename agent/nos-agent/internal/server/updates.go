package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type UpdatesPlanRequest struct {
	Packages []string `json:"packages"`
}

type UpdateEntry struct {
	Name      string `json:"name"`
	Current   string `json:"current"`
	Candidate string `json:"candidate"`
	Arch      string `json:"arch"`
	Repo      string `json:"repo"`
}

type UpdatesPlanResponse struct {
	Updates        []UpdateEntry `json:"updates"`
	RebootRequired bool          `json:"reboot_required,omitempty"`
	Raw            string        `json:"raw"`
	Note           string        `json:"note,omitempty"`
}

func handleUpdatesPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if runtime.GOOS == "windows" {
		writeErr(w, http.StatusNotImplemented, "not supported on windows")
		return
	}

	var req UpdatesPlanRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	// detect Debian/apt
	if _, err := exec.LookPath("apt-get"); err != nil || !fileExists("/etc/debian_version") {
		writeJSON(w, http.StatusOK, UpdatesPlanResponse{Updates: []UpdateEntry{}, Raw: "apt-get not available", Note: "Non-Debian or apt not present"})
		return
	}

	// A first refresh downloads the whole package lists, and apt's daily
	// job may hold the lock for a while after boot.
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Minute)
	defer cancel()

	// The lists must be fresh: a check on stale lists offers nothing new
	// (it used to carry on silently when this failed).
	if out, err := refreshPackageLists(ctx); err != nil {
		logAuthPriv(fmt.Sprintf("updates: apt-get update failed: %s", out))
		writeErr(w, http.StatusBadGateway, "the package lists could not be refreshed (apt-get update): "+tailLines(out, 4))
		return
	}

	// simulate upgrade
	sim := exec.CommandContext(ctx, "apt-get", "-s", "upgrade")
	rawOut, err := sim.CombinedOutput()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("apt-get -s upgrade failed: %s", string(rawOut)))
		return
	}

	updates := parseAptSimulateUpgrade(rawOut)

	// if packages specified, filter
	if len(req.Packages) > 0 {
		want := map[string]struct{}{}
		for _, p := range req.Packages {
			p = strings.TrimSpace(p)
			if p != "" {
				want[p] = struct{}{}
			}
		}
		filtered := make([]UpdateEntry, 0, len(updates))
		for _, u := range updates {
			if _, ok := want[u.Name]; ok {
				filtered = append(filtered, u)
			}
		}
		updates = filtered
	}

	resp := UpdatesPlanResponse{Updates: updates, Raw: string(rawOut)}
	if fileExists("/var/run/reboot-required") {
		resp.RebootRequired = true
	}
	writeJSON(w, http.StatusOK, resp)
}

var aptInstLine = regexp.MustCompile(`^Inst\s+([^\s]+)\s+\[([^\]]+)\]\s+\(([^\)]+)\)`) // Inst name [current] (candidate repo [arch])

func parseAptSimulateUpgrade(out []byte) []UpdateEntry {
	updates := []UpdateEntry{}
	archFromParens := regexp.MustCompile(`\[([^\]]+)\]`)
	lines := bytes.Split(out, []byte("\n"))
	for _, ln := range lines {
		s := string(ln)
		if !strings.HasPrefix(s, "Inst ") {
			continue
		}
		m := aptInstLine.FindStringSubmatch(s)
		if len(m) < 4 {
			continue
		}
		name := m[1]
		current := m[2]
		// m[3] contains "candidate distro [arch]"
		candBlock := m[3]
		// extract arch if present in brackets
		arch := ""
		if am := archFromParens.FindStringSubmatch(candBlock); len(am) == 2 {
			arch = am[1]
		}
		// candidate version is first token of candBlock
		parts := strings.Fields(candBlock)
		candidate := ""
		repo := ""
		if len(parts) > 0 {
			candidate = parts[0]
		}
		if len(parts) > 1 {
			repo = parts[1]
		}
		updates = append(updates, UpdateEntry{
			Name: name, Current: current, Candidate: candidate, Arch: arch, Repo: repo,
		})
	}
	return updates
}

func fileExists(p string) bool {
	_, err := os.Stat(filepath.Clean(p))
	return err == nil
}

// aptLockBusy is apt's message when another apt (apt-daily, an upgrade)
// holds the lock.
var aptLockBusy = regexp.MustCompile(`Could not get lock|Unable to lock directory|is held by process`)

// runAptUpdate runs apt-get update as its own transient unit, like the
// upgrade: inside nos-agent's service, apt 3 (Debian 13) can't switch its
// download helpers to the _apt user ("setresuid: Operation not permitted").
// --wait --pipe give back its output and exit status. Without systemd (a
// container) it runs directly. A var so tests replace it.
var runAptUpdate = func(ctx context.Context) (string, error) {
	args := []string{"apt-get", "update"}
	if _, err := exec.LookPath("systemd-run"); err == nil && fileExists("/run/systemd/system") {
		args = append([]string{"systemd-run", "--wait", "--pipe", "--collect", "--quiet",
			"--description=nOS package list refresh", "--setenv=DEBIAN_FRONTEND=noninteractive", "--"}, args...)
	}
	out, err := exec.CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
	return string(out), err
}

// refreshPackageLists runs apt-get update, waiting while another apt holds
// the lock, until ctx ends.
func refreshPackageLists(ctx context.Context) (string, error) {
	for {
		out, err := runAptUpdate(ctx)
		if err == nil || !aptLockBusy.MatchString(out) {
			return out, err
		}
		select {
		case <-ctx.Done():
			return out, err
		case <-time.After(aptLockRetry):
		}
	}
}

var aptLockRetry = 5 * time.Second

// tailLines is the last n lines of out, joined for a one-line message.
func tailLines(out string, n int) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " / ")
}
