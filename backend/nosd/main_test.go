package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"nos/backend/nosd/internal/config"
	"nos/backend/nosd/internal/server"
)

// On a fresh install nosd must create a setup code, print it (stdout goes to
// the journal and the console) and write the console banner.
func TestFirstBootCodeIsCreatedAndShown(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NOS_USERS_PATH", filepath.Join(dir, "users.json"))
	t.Setenv("NOS_FIRSTBOOT_PATH", filepath.Join(dir, "state", "firstboot.json"))
	t.Setenv("NOS_FIRSTBOOT_ISSUE_PATH", filepath.Join(dir, "run", "firstboot.issue"))

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	cfg := config.FromEnv()
	fb := server.FirstBootManager(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	fb.Run(ctx, time.Hour, func(err error) { t.Errorf("run: %v", err) })
	_ = w.Close()
	out, _ := io.ReadAll(r)

	if !regexp.MustCompile(`setup code: \d{6}`).Match(out) {
		t.Fatalf("expected the code on stdout, got: %s", out)
	}
	if _, err := os.Stat(cfg.FirstBootPath); err != nil {
		t.Fatalf("state not created: %v", err)
	}
	banner, err := os.ReadFile(cfg.FirstBootIssuePath)
	if err != nil || !regexp.MustCompile(`One-time setup code: +\d{3} \d{3}`).Match(banner) {
		t.Fatalf("console banner: %v\n%s", err, banner)
	}
}

// Regression (S9): the agent bootstrap token was 32 raw bytes, which the agent
// sends inside a JSON string, so registration rarely matched.
func TestEnsureAgentTokenIsHex(t *testing.T) {
	p := filepath.Join(t.TempDir(), "agent-token")
	if err := os.WriteFile(p, []byte{0xff, 0x00, 0x81, '\n', 0x7f}, 0o600); err != nil {
		t.Fatal(err)
	}
	ensureAgentToken(p)
	b, _ := os.ReadFile(p)
	tok := strings.TrimSpace(string(b))
	if !isHexToken(tok) {
		t.Fatalf("binary token not replaced: %q", b)
	}
	ensureAgentToken(p)
	if b2, _ := os.ReadFile(p); strings.TrimSpace(string(b2)) != tok {
		t.Fatal("a valid token must be kept")
	}
}
