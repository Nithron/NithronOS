package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func handleFirewallApply(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RulesetText string `json:"ruleset_text"`
		Persist     bool   `json:"persist"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.RulesetText == "" {
		writeErr(w, http.StatusBadRequest, "ruleset_text required")
		return
	}
	if len(body.RulesetText) > 200*1024 {
		writeErr(w, http.StatusRequestEntityTooLarge, "ruleset too large")
		return
	}
	if strings.Contains(body.RulesetText, "`") || strings.Contains(body.RulesetText, "$(") {
		writeErr(w, http.StatusBadRequest, "ruleset contains forbidden characters")
		return
	}

	etcNosFwDir := "/etc/nos/firewall"
	nftDropDir := "/etc/nftables.d"
	pendingPath := filepath.Join(etcNosFwDir, "pending.nft")
	backupPath := filepath.Join(etcNosFwDir, fmt.Sprintf("backup-%s.nft", time.Now().UTC().Format("20060102-150405")))
	persistPath := filepath.Join(nftDropDir, "nos.nft")

	if err := os.MkdirAll(etcNosFwDir, 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := os.MkdirAll(nftDropDir, 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := os.WriteFile(pendingPath, []byte(body.RulesetText), 0o600); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Check
	if out, err := exec.Command("nft", "-c", "-f", pendingPath).CombinedOutput(); err != nil {
		writeErr(w, http.StatusBadRequest, strings.TrimSpace(string(out)))
		return
	}

	// Backup current ruleset
	if out, err := exec.Command("nft", "list", "ruleset").CombinedOutput(); err == nil {
		_ = os.WriteFile(backupPath, out, 0o600)
	} else {
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("backup failed: %v", err))
		return
	}

	// Apply new rules
	if out, err := exec.Command("nft", "-f", pendingPath).CombinedOutput(); err != nil {
		_, _ = exec.Command("nft", "-f", backupPath).CombinedOutput()
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("nft apply failed: %s", strings.TrimSpace(string(out))))
		return
	}

	// Persist
	if body.Persist {
		if err := copyFileSimple(pendingPath, persistPath, 0o644); err != nil {
			writeErr(w, http.StatusInternalServerError, fmt.Sprintf("persist copy failed: %v", err))
			return
		}
		if out, err := exec.Command("systemctl", "enable", "--now", "nftables.service").CombinedOutput(); err != nil {
			writeErr(w, http.StatusInternalServerError, fmt.Sprintf("enable nftables failed: %s", strings.TrimSpace(string(out))))
			return
		}
	}

	logAuthPriv(fmt.Sprintf("firewall applied; persist=%v; backup=%s", body.Persist, backupPath))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "backup_path": backupPath})
}

func copyFileSimple(src, dst string, mode os.FileMode) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, mode)
}

// handleFirewallRuleset returns the kernel's current ruleset (`nft list
// ruleset`, `?json=1` for -j); nosd can't read it as the nos user.
func handleFirewallRuleset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	args := []string{"list", "ruleset"}
	if r.URL.Query().Get("json") == "1" {
		args = append([]string{"-j"}, args...)
	}
	out, err := exec.Command("nft", args...).Output()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "nft list ruleset failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ruleset": string(out)})
}
