package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"nos/backend/nosd/internal/config"
	"nos/backend/nosd/internal/setup/firstboot"
)

func setupOTPEnv(t *testing.T) (config.Config, http.Handler) {
	t.Helper()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "secret.key"), bytes.Repeat([]byte{7}, 32), 0o600)
	t.Setenv("NOS_SECRET_PATH", filepath.Join(dir, "secret.key"))
	t.Setenv("NOS_USERS_PATH", filepath.Join(dir, "users.json"))
	t.Setenv("NOS_FIRSTBOOT_PATH", filepath.Join(dir, "firstboot.json"))
	t.Setenv("NOS_FIRSTBOOT_ISSUE_PATH", filepath.Join(dir, "firstboot.issue"))
	t.Setenv("NOS_RL_PATH", filepath.Join(dir, "ratelimit.json"))
	t.Setenv("NOS_ETC_DIR", dir)
	t.Setenv("NOS_DISABLE_APP_EVENTS", "1")
	cfg := config.FromEnv()
	return cfg, NewRouter(cfg)
}

func getState(t *testing.T, r http.Handler) map[string]any {
	t.Helper()
	res := httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/setup/state", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("state: %d %s", res.Code, res.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &out)
	return out
}

func verifyCode(r http.Handler, code string) *httptest.ResponseRecorder {
	res := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"otp":"` + code + `"}`)
	r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/setup/otp/verify", body))
	return res
}

// Regression: once the code expired, /setup/state reported otpRequired=false
// and setup was stuck until nosd was restarted.
func TestSetupStateReplacesExpiredCode(t *testing.T) {
	cfg, r := setupOTPEnv(t)
	st := getState(t, r)
	if st["otpRequired"] != true || st["expiresAt"] == nil {
		t.Fatalf("fresh install: %v", st)
	}
	first, _ := firstboot.New(cfg.FirstBootPath).Load()

	// Age the code past its expiry.
	first.ExpiresAt = time.Now().Add(-time.Second)
	if err := firstboot.New(cfg.FirstBootPath).Save(first); err != nil {
		t.Fatal(err)
	}
	st = getState(t, r)
	if st["otpRequired"] != true {
		t.Fatalf("expired code must be replaced, got %v", st)
	}
	second, _ := firstboot.New(cfg.FirstBootPath).Load()
	if second == nil || second.Expired(time.Now()) {
		t.Fatalf("no fresh code: %+v", second)
	}
	if res := verifyCode(r, second.OTP); res.Code != http.StatusOK {
		t.Fatalf("new code rejected: %d %s", res.Code, res.Body.String())
	}
	banner, err := os.ReadFile(cfg.FirstBootIssuePath)
	if err != nil || !bytes.Contains(banner, []byte(second.OTP[:3]+" "+second.OTP[3:])) {
		t.Fatalf("console banner should show the new code: %v\n%s", err, banner)
	}
}

func TestSetupVerifyWrongCodesRotate(t *testing.T) {
	t.Setenv("NOS_RATE_OTP_PER_MIN", "1000")
	cfg, r := setupOTPEnv(t)
	getState(t, r)
	orig, _ := firstboot.New(cfg.FirstBootPath).Load()
	wrong := "000000"
	if orig.OTP == wrong {
		wrong = "111111"
	}
	for i := 0; i < firstboot.DefaultMaxAttempts; i++ {
		if res := verifyCode(r, wrong); res.Code != http.StatusBadRequest {
			t.Fatalf("wrong code %d: %d", i, res.Code)
		}
	}
	if res := verifyCode(r, orig.OTP); res.Code == http.StatusOK {
		t.Fatal("the original code must stop working after too many wrong guesses")
	}
	if res := verifyCode(r, "12 3"); res.Code != http.StatusBadRequest {
		t.Fatalf("malformed code: %d", res.Code)
	}
}
