package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"nos/backend/nosd/internal/config"
)

// Fast window tests for OTP and Login using short windows via env knobs
func TestOTPAndLoginRateLimitIntegration(t *testing.T) {
	dir := t.TempDir()
	// minimal config for secret/users/firstboot; reuse existing helper flow lightly
	secretPath := filepath.Join(dir, "secret.key")
	firstbootPath := filepath.Join(dir, "firstboot.json")
	usersPath := filepath.Join(dir, "users.json")
	_ = os.WriteFile(secretPath, bytes.Repeat([]byte{1}, 32), 0o600)
	_ = os.WriteFile(usersPath, []byte("{}"), 0o600)
	_ = os.WriteFile(firstbootPath, []byte(`{"otp":"111111","issued_at":"`+time.Now().UTC().Format(time.RFC3339)+`","expires_at":"`+time.Now().UTC().Add(15*time.Minute).Format(time.RFC3339)+`"}`), 0o600)
	t.Setenv("NOS_SECRET_PATH", secretPath)
	t.Setenv("NOS_FIRSTBOOT_PATH", firstbootPath)
	t.Setenv("NOS_USERS_PATH", usersPath)
	t.Setenv("NOS_TRUST_PROXY", "0")
	// tighten windows: 2/min for OTP and 2/15m for login (we will simulate IP/user only)
	t.Setenv("NOS_RATE_OTP_PER_MIN", "2")
	t.Setenv("NOS_RATE_LOGIN_PER_15M", "2")
	t.Setenv("NOS_RATE_OTP_WINDOW_SEC", "2")
	t.Setenv("NOS_RATE_LOGIN_WINDOW_SEC", "2")

	cfg := config.FromEnv()
	r := NewRouter(cfg)

	// OTP: allow twice, third returns 429 and Retry-After > 0
	for i := 0; i < 2; i++ {
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/setup/otp/verify", bytes.NewBufferString(`{"otp":"111111"}`)))
		if res.Code != 200 {
			t.Fatalf("otp allow #%d: %d", i+1, res.Code)
		}
	}
	{
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/setup/otp/verify", bytes.NewBufferString(`{"otp":"111111"}`)))
		if res.Code != http.StatusTooManyRequests {
			t.Fatalf("otp limit: %d", res.Code)
		}
		ra := res.Header().Get("Retry-After")
		if ra == "" {
			t.Fatalf("missing Retry-After")
		}
		if n, _ := strconv.Atoi(ra); n <= 0 {
			t.Fatalf("retry-after nonpositive: %s", ra)
		}
	}

	// wait and confirm recovery for OTP window (should not be 429 after window elapses)
	time.Sleep(2100 * time.Millisecond)
	{
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/setup/otp/verify", bytes.NewBufferString(`{"otp":"111111"}`)))
		if res.Code == http.StatusTooManyRequests {
			t.Fatalf("expected OTP limiter recovery after window")
		}
	}

	// Login limit by IP and user: exercise IP path easily by no username (but login path requires body)
	// We won't perform full login; just ensure limiter check path returns 429 when triggered.
	// For this repository, login performs multiple checks; here we can call it with invalid body to just pass the limiter.
	for i := 0; i < 2; i++ {
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"bob","password":"x"}`)))
		// it may return 401 for wrong password; we only care that it is not 429
		if res.Code == http.StatusTooManyRequests {
			t.Fatalf("unexpected 429 on #%d", i+1)
		}
	}
	{
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"bob","password":"x"}`)))
		if res.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429 for login limit, got %d", res.Code)
		}
		ra := res.Header().Get("Retry-After")
		if ra == "" {
			t.Fatalf("missing Retry-After (login)")
		}
	}
	// wait and confirm recovery for login window
	time.Sleep(2100 * time.Millisecond)
	{
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"bob","password":"x"}`)))
		if res.Code == http.StatusTooManyRequests {
			t.Fatalf("expected recovery after window")
		}
	}
}
