package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"os/user"
	"regexp"
	"strings"
)

var usernameRe = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)

type SMBUserCreateRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func handleSMBUserCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req SMBUserCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if !usernameRe.MatchString(req.Username) {
		writeErr(w, http.StatusBadRequest, "invalid username")
		return
	}
	// The password goes to smbpasswd on stdin, one line twice; it can't
	// contain a line break. (It used to be put into an `sh -c` command line,
	// where $(...) in a password ran as root.)
	if len(req.Password) < 8 || len(req.Password) > 256 || strings.ContainsAny(req.Password, "\r\n\x00") {
		writeErr(w, http.StatusBadRequest, "password must be 8-256 characters on one line")
		return
	}
	// A login without a home and shell. Existing accounts must be regular
	// users: an SMB password for root or a daemon account is refused.
	if _, err := user.Lookup(req.Username); err != nil {
		if out, err := exec.Command("useradd", "-M", "-s", "/usr/sbin/nologin", req.Username).CombinedOutput(); err != nil {
			writeErr(w, http.StatusInternalServerError, fmt.Sprintf("useradd failed: %s", strings.TrimSpace(string(out))))
			return
		}
	}
	if !(realSharesHost{}).UserOK(req.Username) {
		writeErr(w, http.StatusBadRequest, "not a regular user account")
		return
	}
	cmd := exec.Command("smbpasswd", "-s", "-a", req.Username)
	cmd.Stdin = strings.NewReader(req.Password + "\n" + req.Password + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("smbpasswd failed: %s", strings.TrimSpace(string(out))))
		return
	}
	logAuthPriv("smb.user-create " + req.Username)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func handleSMBUsersList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	cmd := exec.Command("pdbedit", "-L")
	out, err := cmd.CombinedOutput()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "pdbedit failed")
		return
	}
	users := []string{}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		// format: username:... or username:XXXX:... depending on backend
		if i := strings.Index(line, ":"); i > 0 {
			u := line[:i]
			if usernameRe.MatchString(u) {
				users = append(users, u)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}
