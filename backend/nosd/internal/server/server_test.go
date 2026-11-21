package server

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestMain points every on-disk path nosd uses at a throwaway directory, so
// tests never write into the repository (devdata/) or the real /etc and /var.
// Individual tests can still override any of these with t.Setenv.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "nosd-server-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "create temp dir:", err)
		os.Exit(1)
	}

	env := map[string]string{
		// Skip auth in handler tests
		"NOS_TEST_SKIP_AUTH": "1",
		// Soften rate limits to avoid delays in tests
		"NOS_RATE_OTP_PER_MIN":      "1000",
		"NOS_RATE_LOGIN_PER_15M":    "1000",
		"NOS_RATE_OTP_WINDOW_SEC":   "1",
		"NOS_RATE_LOGIN_WINDOW_SEC": "1",
		// Isolated state
		"NOS_USERS_PATH":       filepath.Join(dir, "etc", "users.json"),
		"NOS_SESSIONS_PATH":    filepath.Join(dir, "lib", "sessions.json"),
		"NOS_SECRET_PATH":      filepath.Join(dir, "etc", "secret.key"),
		"NOS_FIRSTBOOT_PATH":   filepath.Join(dir, "lib", "state", "firstboot.json"),
		"NOS_RL_PATH":          filepath.Join(dir, "lib", "ratelimit.json"),
		"NOS_SHARES_PATH":      filepath.Join(dir, "etc", "shares.json"),
		"NOS_ETC_DIR":          filepath.Join(dir, "etc"),
		"NOS_APPS_STATE":       filepath.Join(dir, "lib", "apps.json"),
		"NOS_APPS_DATA_DIR":    filepath.Join(dir, "lib", "apps"),
		"NOS_APPS_INSTALL_DIR": filepath.Join(dir, "srv", "apps"),
	}
	for k, v := range env {
		if err := os.Setenv(k, v); err != nil {
			fmt.Fprintln(os.Stderr, "setenv:", err)
			os.Exit(1)
		}
	}

	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
