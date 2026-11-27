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
	"strings"
	"time"
)

type RunStep struct {
	Cmd  string   `json:"cmd"`
	Args []string `json:"args"`
}

type RunRequest struct {
	Steps []RunStep `json:"steps"`
}

type RunResult struct {
	Code   int    `json:"code"`
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

// test seam for system dirs
var etcDir = "/etc"

func fsyncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	_ = f.Sync()
	return nil
}

// handleRun executes a small allowlisted set of commands without a shell.
func handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(req.Steps) == 0 || len(req.Steps) > 32 {
		writeErr(w, http.StatusBadRequest, "invalid steps")
		return
	}
	results := make([]RunResult, 0, len(req.Steps))
	for _, s := range req.Steps {
		if !allowedCommand(s.Cmd, s.Args) {
			writeErr(w, http.StatusBadRequest, "command not allowed")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		binary := s.Cmd
		if binary == "btrfs" {
			binary = "/usr/bin/btrfs"
		}
		cmd := exec.CommandContext(ctx, binary, s.Args...)
		cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
		var stdoutBuf, stderrBuf bytes.Buffer
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf
		err := cmd.Run()
		res := RunResult{Stdout: stdoutBuf.String(), Stderr: truncate(stderrBuf.String(), 4096)}
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				res.Code = ee.ExitCode()
			} else {
				res.Code = -1
			}
		}
		results = append(results, res)
		if res.Code != 0 {
			break
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func allowedCommand(name string, args []string) bool {
	name = strings.TrimSpace(name)
	switch name {
	case "wipefs":
		// allow -n (report) or -a (wipe) and device paths
		if len(args) < 1 || len(args) > 2 {
			return false
		}
		if len(args) == 2 && args[0] != "-n" && args[0] != "-a" {
			return false
		}
		dev := args[len(args)-1]
		return validDevice(dev)
	case "mkfs.btrfs":
		if len(args) < 2 {
			return false
		}
		// very simple: must include -L and devices are /dev/*
		hasL, hasD, hasM := false, false, false
		devs := 0
		for i := 0; i < len(args); i++ {
			a := args[i]
			if a == "-L" && i+1 < len(args) {
				hasL = true
				i++
				continue
			}
			if a == "-d" && i+1 < len(args) {
				hasD = true
				i++
				continue
			}
			if a == "-m" && i+1 < len(args) {
				hasM = true
				i++
				continue
			}
			if strings.HasPrefix(a, "/dev/") {
				if !validDevice(a) {
					return false
				}
				devs++
			}
		}
		return hasL && hasD && hasM && devs >= 1
	case "mount":
		// Only allow: mount -t btrfs -o <opts> <source> <target>
		if len(args) < 5 {
			return false
		}
		if args[0] != "-t" || args[1] != "btrfs" {
			return false
		}
		// allow -o next
		idx := 2
		if args[idx] == "-o" {
			idx += 2
		}
		if len(args) < idx+2 {
			return false
		}
		src := args[idx]
		dst := args[idx+1]
		if !strings.HasPrefix(src, "UUID=") && !validDevice(src) {
			return false
		}
		// pools only: never over a system folder
		return isAllowedMountPath(dst)
	case "umount":
		return len(args) == 1 && isAllowedMountPath(args[0])
	case "blkid":
		// blkid -s UUID -o value <device>
		if len(args) != 5 {
			return false
		}
		if args[0] != "-s" || args[1] != "UUID" || args[2] != "-o" || args[3] != "value" {
			return false
		}
		return validDevice(args[4])
	case "btrfs":
		// Central strict allowlist by subcommand prefix
		if !allowedBtrfsPrefix(args) {
			return false
		}
		// device add/remove ... <mount>
		if len(args) >= 3 && args[0] == "device" && (args[1] == "add" || args[1] == "remove") {
			mnt := args[len(args)-1]
			if !isAllowedMountPath(mnt) {
				return false
			}
			for i := 2; i < len(args)-1; i++ {
				if !validDevice(args[i]) {
					return false
				}
			}
			return true
		}
		// replace start <old> <new> <mount>
		if len(args) == 5 && args[0] == "replace" && args[1] == "start" {
			old := args[2]
			newd := args[3]
			m := args[4]
			return validDevice(old) && validDevice(newd) && isAllowedMountPath(m)
		}
		// replace status <mount>
		if len(args) == 3 && args[0] == "replace" && args[1] == "status" {
			return isAllowedMountPath(args[2])
		}
		// balance start ... <mount>
		if len(args) >= 3 && args[0] == "balance" && args[1] == "start" {
			mnt := args[len(args)-1]
			return isAllowedMountPath(mnt)
		}
		// balance status|cancel <mount>
		if len(args) == 3 && args[0] == "balance" && (args[1] == "status" || args[1] == "cancel") {
			return isAllowedMountPath(args[2])
		}
		// filesystem show|usage [flags] [mount]
		if len(args) >= 2 && args[0] == "filesystem" && (args[1] == "show" || args[1] == "usage") {
			// last non-flag token, if present, must be an allowed mount path
			for i := len(args) - 1; i >= 2; i-- {
				if !strings.HasPrefix(args[i], "-") {
					return isAllowedMountPath(args[i])
				}
			}
			return true
		}
		return false
	case "cryptsetup":
		if len(args) == 0 {
			return false
		}
		// cryptsetup luksFormat [--type luks2] [--batch-mode] <device>
		if args[0] == "luksFormat" {
			dev := ""
			for i := 1; i < len(args); i++ {
				a := args[i]
				if a == "--type" && i+1 < len(args) {
					i++
					continue
				}
				if a == "--batch-mode" {
					continue
				}
				dev = a
			}
			return dev != "" && validDevice(dev)
		}
		// cryptsetup open --key-file <keyfile> <device> <name>
		if args[0] == "open" {
			if len(args) != 5 {
				return false
			}
			if args[1] != "--key-file" {
				return false
			}
			key := args[2]
			dev := args[3]
			name := args[4]
			if !strings.HasPrefix(key, "/") {
				return false
			}
			if !validDevice(dev) {
				return false
			}
			if !strings.HasPrefix(name, "luks-") {
				return false
			}
			return true
		}
		// cryptsetup close <name>
		if args[0] == "close" {
			return len(args) == 2 && strings.HasPrefix(args[1], "luks-")
		}
		return false
	default:
		return false
	}
}

func validDevice(p string) bool {
	return p != "" && strings.HasPrefix(p, "/dev/") && !strings.ContainsAny(p, " \t\n\r\x00")
}

func isAllowedMountPath(p string) bool {
	if p == "" || !strings.HasPrefix(p, "/") || strings.ContainsAny(p, " \t\n\r\x00") {
		return false
	}
	// ensure trailing slash comparison is safe for exact prefix
	ps := p
	if !strings.HasSuffix(ps, "/") {
		ps += "/"
	}
	return strings.HasPrefix(ps, "/srv/") || strings.HasPrefix(ps, "/mnt/")
}

func allowedBtrfsPrefix(args []string) bool {
	if len(args) < 2 {
		return false
	}
	allowed := [][]string{
		{"device", "add"}, {"device", "remove"},
		{"replace", "start"}, {"replace", "status"},
		{"balance", "start"}, {"balance", "status"}, {"balance", "cancel"},
		{"filesystem", "show"}, {"filesystem", "usage"},
	}
	for _, pref := range allowed {
		if len(args) < len(pref) {
			continue
		}
		ok := true
		for i := range pref {
			if args[i] != pref[i] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// Fstab helpers
func handleFstabEnsure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Line string `json:"line"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Line) == "" {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	path := filepath.Join(etcDir, "fstab")
	data := ""
	if b, err := os.ReadFile(path); err == nil {
		data = string(b)
	}
	if !strings.Contains(data, body.Line) {
		if !strings.HasSuffix(data, "\n") && len(data) > 0 {
			data += "\n"
		}
		data += body.Line + "\n"
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		if f, err := os.OpenFile(path+".tmp", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644); err == nil {
			_, _ = f.WriteString(data)
			_ = f.Sync()
			_ = f.Close()
			_ = fsyncDir(filepath.Dir(path))
			_ = os.Rename(path+".tmp", path)
			_ = fsyncDir(filepath.Dir(path))
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func handleFstabRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Contains string `json:"contains"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Contains) == "" {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	path := filepath.Join(etcDir, "fstab")
	b, err := os.ReadFile(path)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("read fstab: %v", err))
		return
	}
	lines := strings.Split(string(b), "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		if !strings.Contains(ln, body.Contains) {
			out = append(out, ln)
		}
	}
	data := strings.Join(out, "\n")
	if f, err := os.OpenFile(path+".tmp", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644); err == nil {
		_, _ = f.WriteString(data)
		_ = f.Sync()
		_ = f.Close()
		_ = fsyncDir(filepath.Dir(path))
		_ = os.Rename(path+".tmp", path)
		_ = fsyncDir(filepath.Dir(path))
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
