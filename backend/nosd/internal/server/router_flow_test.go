package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	userstore "nithronos/backend/nosd/internal/auth/store"
	"nithronos/backend/nosd/internal/config"

	"github.com/gorilla/securecookie"
	"github.com/pquerna/otp/totp"
)

func TestSetupFullFlowAnd410(t *testing.T) {
	// temp state
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "secret.key")
	firstbootPath := filepath.Join(dir, "firstboot.json")
	usersPath := filepath.Join(dir, "users.json")
	// seed secret
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	_ = os.WriteFile(secretPath, key, 0o600)
	_ = os.WriteFile(usersPath, []byte("{}"), 0o600)
	_ = os.WriteFile(firstbootPath, []byte(`{"otp":"111111","issued_at":"`+time.Now().UTC().Format(time.RFC3339)+`","expires_at":"`+time.Now().UTC().Add(15*time.Minute).Format(time.RFC3339)+`"}`), 0o600)
	// Point config/env to temp files
	t.Setenv("NOS_SECRET_PATH", secretPath)
	t.Setenv("NOS_USERS_PATH", usersPath)
	t.Setenv("NOS_FIRSTBOOT_PATH", firstbootPath)
	t.Setenv("NOS_RL_PATH", filepath.Join(dir, "ratelimit.json"))
	t.Setenv("NOS_ETC_DIR", dir)
	// Ensure apps manager writes under temp dir and does not open event log file
	t.Setenv("NOS_APPS_STATE", filepath.Join(dir, "apps.json"))
	t.Setenv("NOS_DISABLE_APP_EVENTS", "1")
	cfg := config.FromEnv()
	r := NewRouter(cfg)

	// state
	t.Log("state")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/setup/state", nil))
	if res.Code != 200 {
		t.Fatalf("state: expected 200, got %d", res.Code)
	}

	// verify-otp
	var token string
	{
		t.Log("verify-otp")
		body := bytes.NewBuffer(mustJSON(map[string]string{"otp": "111111"}))
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/setup/otp/verify", body))
		if res.Code != 200 {
			t.Fatalf("verify-otp: %d", res.Code)
		}
		var out map[string]any
		_ = json.Unmarshal(res.Body.Bytes(), &out)
		token, _ = out["token"].(string)
		if token == "" {
			t.Fatal("missing token")
		}
	}

	// expired token should fail (accept 400 with any error message)
	{
		t.Log("expired-token")
		sc := securecookie.New(key, nil)
		claims := map[string]any{"purpose": "setup", "exp": time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339)}
		expTok, _ := sc.Encode("nos_setup", claims)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/setup/first-admin", bytes.NewBuffer(mustJSON(map[string]any{"username": "alice", "password": "StrongPassw0rd!"})))
		req.Header.Set("Authorization", "Bearer "+expTok)
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != http.StatusBadRequest {
			t.Fatalf("expired token expected 400, got %d", res.Code)
		}
	}

	// create-admin (without totp)
	{
		t.Log("create-admin")
		req := httptest.NewRequest(http.MethodPost, "/api/v1/setup/first-admin", bytes.NewBuffer(mustJSON(map[string]any{"username": "alice", "password": "StrongPassw0rd!", "enable_totp": false})))
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("create-admin: expected 200, got %d %s", res.Code, res.Body.String())
		}
	}

	// Mark setup as complete
	{
		t.Log("mark-setup-complete")
		req := httptest.NewRequest(http.MethodPost, "/api/v1/setup/complete", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != http.StatusNoContent {
			t.Fatalf("setup/complete: expected 204, got %d %s", res.Code, res.Body.String())
		}
	}

	// state now 410
	{
		t.Log("state-410")
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/setup/state", nil))
		if res.Code != http.StatusGone {
			t.Fatalf("state after setup: expected 410, got %d", res.Code)
		}
	}

	// login to get session and csrf
	var cookies []*http.Cookie
	var csrf string
	{
		t.Log("login-1")
		lb := mustJSON(map[string]any{"username": "alice", "password": "StrongPassw0rd!"})
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(lb)))
		if res.Code != 200 {
			t.Fatalf("login: %d %s", res.Code, res.Body.String())
		}
		cookies = res.Result().Cookies()
		for _, c := range cookies {
			if c.Name == "nos_csrf" {
				csrf = c.Value
			}
		}
		// verify cookie presence
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		if uid, ok := decodeSessionUID(req, cfg); !ok || uid == "" {
			t.Fatalf("nos_session did not decode; cookies=%v", cookies)
		}
	}

	// sessions list
	{
		t.Log("sessions-list")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/sessions", nil)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != 200 {
			t.Fatalf("sessions list: %d", res.Code)
		}
		var arr []map[string]any
		_ = json.Unmarshal(res.Body.Bytes(), &arr)
		if len(arr) == 0 {
			t.Fatalf("expected at least one session")
		}
	}

	// revoke current session
	{
		t.Log("revoke-current")
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sessions/revoke", bytes.NewReader(mustJSON(map[string]string{"scope": "current"})))
		for _, c := range cookies {
			req.AddCookie(c)
		}
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != 200 {
			t.Fatalf("revoke current: %d", res.Code)
		}
		// me should now be unauthorized
		res2 := httptest.NewRecorder()
		r.ServeHTTP(res2, httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil))
		if res2.Code != http.StatusUnauthorized {
			t.Fatalf("me after revoke current: %d", res2.Code)
		}
	}

	// enroll
	{
		t.Log("enroll")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/totp/enroll", nil)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != 200 {
			t.Fatalf("enroll: %d", res.Code)
		}
	}

	// fetch user to get secret and generate code
	secCode := func() string {
		t.Log("get-secret")
		us, _ := userstore.New(usersPath)
		u, _ := us.FindByUsername("alice")
		// Enrollment is pending until a code confirms it; login must not
		// require a code yet.
		if u.TOTPPendingEnc == "" || u.TOTPEnc != "" {
			t.Fatalf("expected a pending secret only after enroll: pending=%q active=%q", u.TOTPPendingEnc, u.TOTPEnc)
		}
		pt, err := decryptWithSecretKey(secretPath, u.TOTPPendingEnc)
		if err != nil {
			t.Fatalf("decrypt secret: %v", err)
		}
		code, _ := totp.GenerateCode(string(pt), time.Now())
		return code
	}()

	// verify
	{
		t.Log("verify")
		vb := mustJSON(map[string]string{"code": secCode})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/totp/verify", bytes.NewReader(vb))
		for _, c := range cookies {
			req.AddCookie(c)
		}
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != 200 {
			t.Fatalf("verify totp: %d %s", res.Code, res.Body.String())
		}
		var out map[string]any
		_ = json.Unmarshal(res.Body.Bytes(), &out)
		if _, ok := out["recovery_codes"].([]any); !ok {
			t.Fatal("missing recovery codes")
		}
	}

	// login with code now required
	{
		t.Log("login-2")
		code, _ := totp.GenerateCode(secCodeSecret(t, secretPath, usersPath), time.Now())
		lb := mustJSON(map[string]any{"username": "alice", "password": "StrongPassw0rd!", "code": code})
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(lb)))
		if res.Code != 200 {
			t.Fatalf("login with code: %d", res.Code)
		}
		cookies = res.Result().Cookies()
	}

	// me
	{
		t.Log("me")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != 200 {
			t.Fatalf("me: %d", res.Code)
		}
	}

	// logout
	{
		t.Log("logout")
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil))
		if res.Code != http.StatusNoContent {
			t.Fatalf("logout: %d", res.Code)
		}
	}

	// me unauthorized
	{
		t.Log("me-unauth")
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil))
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("me after logout: %d", res.Code)
		}
	}
}

func TestSetupCookiePathAndCookieAuth(t *testing.T) {
	// temp state
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "secret.key")
	firstbootPath := filepath.Join(dir, "firstboot.json")
	usersPath := filepath.Join(dir, "users.json")
	// seed secret
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	_ = os.WriteFile(secretPath, key, 0o600)
	_ = os.WriteFile(usersPath, []byte("{}"), 0o600)
	_ = os.WriteFile(firstbootPath, []byte(`{"otp":"222222","issued_at":"`+time.Now().UTC().Format(time.RFC3339)+`","expires_at":"`+time.Now().UTC().Add(15*time.Minute).Format(time.RFC3339)+`"}`), 0o600)
	// Point config/env to temp files
	t.Setenv("NOS_SECRET_PATH", secretPath)
	t.Setenv("NOS_USERS_PATH", usersPath)
	t.Setenv("NOS_FIRSTBOOT_PATH", firstbootPath)
	t.Setenv("NOS_RL_PATH", filepath.Join(dir, "ratelimit.json"))
	t.Setenv("NOS_ETC_DIR", dir)
	t.Setenv("NOS_APPS_STATE", filepath.Join(dir, "apps.json"))
	t.Setenv("NOS_DISABLE_APP_EVENTS", "1")
	cfg := config.FromEnv()
	r := NewRouter(cfg)

	// verify-otp: should set Set-Cookie with Path=/api/v1/setup and HttpOnly
	var setupCookie *http.Cookie
	{
		body := bytes.NewBuffer(mustJSON(map[string]string{"otp": "222222"}))
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/setup/otp/verify", body))
		if res.Code != 200 {
			t.Fatalf("verify-otp: %d", res.Code)
		}
		for _, c := range res.Result().Cookies() {
			if c.Name == "nos_setup" {
				setupCookie = c
			}
		}
		if setupCookie == nil {
			t.Fatalf("missing nos_setup cookie; headers=%v", res.Result().Header.Values("Set-Cookie"))
		}
		if setupCookie.Path != "/api/v1/setup" {
			t.Fatalf("expected nos_setup Path=/api/v1/setup, got %q", setupCookie.Path)
		}
		if !setupCookie.HttpOnly {
			t.Fatalf("expected nos_setup HttpOnly=true")
		}
	}

	// first-admin without any token should be 401
	{
		req := httptest.NewRequest(http.MethodPost, "/api/v1/setup/first-admin", bytes.NewBuffer(mustJSON(map[string]any{"username": "charlie", "password": "StrongPassw0rd!"})))
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("first-admin without token expected 401, got %d", res.Code)
		}
	}

	// first-admin with cookie should be 200
	{
		req := httptest.NewRequest(http.MethodPost, "/api/v1/setup/first-admin", bytes.NewBuffer(mustJSON(map[string]any{"username": "charlie", "password": "StrongPassw0rd!"})))
		req.AddCookie(setupCookie)
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("first-admin with cookie expected 200, got %d (%s)", res.Code, res.Body.String())
		}
	}

	// state should now be 410
	{
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/setup/state", nil))
		if res.Code != http.StatusGone {
			t.Fatalf("state after setup: expected 410, got %d", res.Code)
		}
	}
}

func TestSetupVerifyOTP_TypedErrors(t *testing.T) {
	cfg := config.Defaults()
	// disable app events file in tests
	t.Setenv("NOS_DISABLE_APP_EVENTS", "1")
	r := NewRouter(cfg)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/setup/otp/verify", bytes.NewBufferString(`{"otp":"1"}`)))
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "setup.otp.invalid") {
		t.Fatalf("expected code setup.otp.invalid, got %s", res.Body.String())
	}
}

func secCodeSecret(t *testing.T, secretPath, usersPath string) string {
	us, _ := userstore.New(usersPath)
	u, _ := us.FindByUsername("alice")
	pt, err := decryptWithSecretKey(secretPath, u.TOTPEnc)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	return string(pt)
}

func TestCreateAdmin_WriteFailThenRetrySameToken(t *testing.T) {
	// Prepare isolated users db and secret
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "secret.key")
	usersPath := filepath.Join(dir, "users.json")
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(1 + i)
	}
	if err := os.WriteFile(secretPath, key, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NOS_SECRET_PATH", secretPath)
	t.Setenv("NOS_USERS_PATH", usersPath)
	t.Setenv("NOS_ETC_DIR", dir)
	// ensure apps manager does not open event log file
	t.Setenv("NOS_APPS_STATE", filepath.Join(dir, "apps.json"))
	t.Setenv("NOS_DISABLE_APP_EVENTS", "1")
	cfg := config.FromEnv()
	r := NewRouter(cfg)

	// Issue a setup token directly
	sc := securecookie.New(key, nil)
	claims := map[string]any{"purpose": "setup", "exp": time.Now().Add(10 * time.Minute).UTC().Format(time.RFC3339)}
	tok, _ := sc.Encode("nos_setup", claims)

	// Simulate write failure
	t.Setenv("NOS_TEST_SIMULATE_WRITE_FAIL", "1")
	body := mustJSON(map[string]any{"username": "bob", "password": "StrongPassw0rd!"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup/first-admin", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "setup.write_failed") {
		t.Fatalf("expected setup.write_failed, got %s", res.Body.String())
	}

	// Clear failure flag and retry with SAME token and a fresh request body
	t.Setenv("NOS_TEST_SIMULATE_WRITE_FAIL", "0")
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/setup/first-admin", bytes.NewBuffer(body))
	req2.Header.Set("Authorization", "Bearer "+tok)
	res2 := httptest.NewRecorder()
	r.ServeHTTP(res2, req2)
	if res2.Code != http.StatusOK {
		t.Fatalf("retry expected 200, got %d (%s)", res2.Code, res2.Body.String())
	}
}

func TestSetupState_MissingEmptyInvalid(t *testing.T) {
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "secret.key")
	usersPath := filepath.Join(dir, "users.json")
	firstbootPath := filepath.Join(dir, "firstboot.json")
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(1 + i)
	}
	if err := os.WriteFile(secretPath, key, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NOS_SECRET_PATH", secretPath)
	t.Setenv("NOS_USERS_PATH", usersPath)
	t.Setenv("NOS_FIRSTBOOT_PATH", firstbootPath)
	t.Setenv("NOS_ETC_DIR", dir)
	// Start with empty users.json
	if err := os.WriteFile(usersPath, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	// Ensure apps manager writes event log under our temp dir to avoid cleanup issues
	t.Setenv("NOS_APPS_STATE", filepath.Join(dir, "apps.json"))
	t.Setenv("NOS_DISABLE_APP_EVENTS", "1")
	cfg := config.FromEnv()
	r := NewRouter(cfg)

	// Missing users.json
	{
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/setup/state", nil))
		if res.Code != http.StatusOK {
			t.Fatalf("missing: expected 200, got %d", res.Code)
		}
		var st map[string]any
		_ = json.Unmarshal(res.Body.Bytes(), &st)
		if fb, _ := st["firstBoot"].(bool); !fb {
			t.Fatalf("missing: expected firstBoot=true, got %v", st)
		}
	}

	// Empty users.json
	if err := os.WriteFile(usersPath, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	{
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/setup/state", nil))
		if res.Code != http.StatusOK {
			t.Fatalf("empty: expected 200, got %d", res.Code)
		}
		var st map[string]any
		_ = json.Unmarshal(res.Body.Bytes(), &st)
		if fb, _ := st["firstBoot"].(bool); !fb {
			t.Fatalf("empty: expected firstBoot=true, got %v", st)
		}
	}

	// Invalid users.json
	if err := os.WriteFile(usersPath, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	{
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/setup/state", nil))
		if res.Code != http.StatusOK {
			t.Fatalf("invalid: expected 200, got %d", res.Code)
		}
		var st map[string]any
		_ = json.Unmarshal(res.Body.Bytes(), &st)
		if fb, _ := st["firstBoot"].(bool); !fb {
			t.Fatalf("invalid: expected firstBoot=true, got %v", st)
		}
	}
}

func TestSetupTransition_DeleteUsersRestoresFirstBoot(t *testing.T) {
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "secret.key")
	usersPath := filepath.Join(dir, "users.json")
	firstbootPath := filepath.Join(dir, "firstboot.json")
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(1 + i)
	}
	if err := os.WriteFile(secretPath, key, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NOS_SECRET_PATH", secretPath)
	t.Setenv("NOS_USERS_PATH", usersPath)
	t.Setenv("NOS_FIRSTBOOT_PATH", firstbootPath)
	// seed firstboot otp
	otp := "654321"
	fb := map[string]any{"otp": otp, "created_at": time.Now().UTC().Format(time.RFC3339), "used": false}
	if b, _ := json.MarshalIndent(fb, "", "  "); b != nil {
		if err := os.WriteFile(firstbootPath, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// high rate limits
	t.Setenv("NOS_RATE_LOGIN_PER_15M", "1000")
	t.Setenv("NOS_RATE_OTP_PER_MIN", "1000")
	t.Setenv("NOS_ETC_DIR", dir)
	// Ensure apps manager writes under temp dir and does not open event log file
	t.Setenv("NOS_APPS_STATE", filepath.Join(dir, "apps.json"))
	t.Setenv("NOS_DISABLE_APP_EVENTS", "1")

	cfg := config.FromEnv()
	r := NewRouter(cfg)

	// Initially: firstBoot=true
	{
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/setup/state", nil))
		if res.Code != http.StatusOK {
			t.Fatalf("initial: %d", res.Code)
		}
	}

	// Verify OTP to get token
	var token string
	{
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/setup/otp/verify", bytes.NewBuffer(mustJSON(map[string]string{"otp": otp}))))
		if res.Code != http.StatusOK {
			t.Fatalf("verify: %d", res.Code)
		}
		var out map[string]any
		_ = json.Unmarshal(res.Body.Bytes(), &out)
		token, _ = out["token"].(string)
		if token == "" {
			t.Fatal("missing token")
		}
	}

	// Create admin
	{
		req := httptest.NewRequest(http.MethodPost, "/api/v1/setup/first-admin", bytes.NewBuffer(mustJSON(map[string]any{"username": "root", "password": "StrongPassw0rd!"})))
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("create-admin: expected 200, got %d %s", res.Code, res.Body.String())
		}
	}

	// Mark setup as complete
	{
		req := httptest.NewRequest(http.MethodPost, "/api/v1/setup/complete", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != http.StatusNoContent {
			t.Fatalf("setup/complete: %d %s", res.Code, res.Body.String())
		}
	}

	// Now gated: /state returns 410
	{
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/setup/state", nil))
		if res.Code != http.StatusGone {
			t.Fatalf("after setup: expected 410, got %d", res.Code)
		}
	}

	// Delete users.json; firstBoot should be true again
	_ = os.Remove(usersPath)
	{
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/setup/state", nil))
		if res.Code != http.StatusOK {
			t.Fatalf("after delete users: %d", res.Code)
		}
		var st map[string]any
		_ = json.Unmarshal(res.Body.Bytes(), &st)
		if fb, _ := st["firstBoot"].(bool); !fb {
			t.Fatalf("after delete users: expected firstBoot=true, got %v", st)
		}
	}
}
