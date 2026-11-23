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

	"github.com/pquerna/otp/totp"

	pwhash "nithronos/backend/nosd/internal/auth/hash"
	userstore "nithronos/backend/nosd/internal/auth/store"
	"nithronos/backend/nosd/internal/config"
	"nithronos/backend/nosd/pkg/auth"
)

type loginEnv struct {
	r      http.Handler
	secret string // TOTP secret (base32)
	users  string // users.json path
}

// newLoginEnv seeds one admin, "alice" / "Correct-Horse-9", with TOTP enabled
// when withTOTP is set, plus the given recovery codes.
func newLoginEnv(t *testing.T, withTOTP bool, recovery ...string) loginEnv {
	t.Helper()
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "secret.key")
	usersPath := filepath.Join(dir, "users.json")
	if err := os.WriteFile(secretPath, bytes.Repeat([]byte{3}, 32), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NOS_SECRET_PATH", secretPath)
	t.Setenv("NOS_USERS_PATH", usersPath)
	t.Setenv("NOS_RL_PATH", filepath.Join(dir, "rl.json"))
	t.Setenv("NOS_SESSIONS_PATH", filepath.Join(dir, "sessions.json"))
	t.Setenv("NOS_RATE_LOGIN_PER_15M", "1000")
	t.Setenv("NOS_DISABLE_APP_EVENTS", "1")

	h, err := pwhash.HashPassword("Correct-Horse-9")
	if err != nil {
		t.Fatal(err)
	}
	u := userstore.User{ID: "u-alice", Username: "alice", PasswordHash: h, Roles: []string{"admin"}}
	env := loginEnv{users: usersPath}
	if withTOTP {
		env.secret, _, err = auth.GenerateTOTPSecret("NithronOS", "alice")
		if err != nil {
			t.Fatal(err)
		}
		if u.TOTPEnc, err = encryptWithSecretKey(secretPath, []byte(env.secret)); err != nil {
			t.Fatal(err)
		}
		for _, c := range recovery {
			u.RecoveryHashes = append(u.RecoveryHashes, hashRecovery(c))
		}
	}
	us, err := userstore.New(usersPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := us.UpsertUser(u); err != nil {
		t.Fatal(err)
	}
	env.r = NewRouter(config.FromEnv())
	return env
}

func (e loginEnv) login(password, code string) (int, string) {
	b, _ := json.Marshal(map[string]string{"username": "alice", "password": password, "code": code})
	res := httptest.NewRecorder()
	e.r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(b)))
	var out struct {
		Error struct{ Code string } `json:"error"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &out)
	return res.Code, out.Error.Code
}

func TestLoginWithoutTOTP(t *testing.T) {
	e := newLoginEnv(t, false)
	if code, _ := e.login("wrong", ""); code != http.StatusUnauthorized {
		t.Fatalf("wrong password: %d", code)
	}
	if code, _ := e.login("Correct-Horse-9", ""); code != http.StatusOK {
		t.Fatalf("right password: %d", code)
	}
}

// Regression (S1): login ignored the two-factor code entirely.
func TestLoginRequiresTOTPWhenEnabled(t *testing.T) {
	e := newLoginEnv(t, true)
	if code, ec := e.login("Correct-Horse-9", ""); code != http.StatusUnauthorized || ec != "auth.totp.required" {
		t.Fatalf("no code: %d %s", code, ec)
	}
	if code, ec := e.login("Correct-Horse-9", "000000"); code != http.StatusUnauthorized || ec != "auth.totp.invalid" {
		t.Fatalf("wrong code: %d %s", code, ec)
	}
	valid, err := totp.GenerateCode(e.secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := e.login("wrong", valid); code != http.StatusUnauthorized {
		t.Fatalf("valid code must not bypass the password: %d", code)
	}
	if code, _ := e.login("Correct-Horse-9", valid); code != http.StatusOK {
		t.Fatalf("password + valid code: %d", code)
	}
}

func TestLoginRecoveryCodeWorksOnce(t *testing.T) {
	e := newLoginEnv(t, true, "abcDEF1234", "zyxWVU9876")
	if code, _ := e.login("Correct-Horse-9", "abcDEF1234"); code != http.StatusOK {
		t.Fatalf("recovery code: %d", code)
	}
	if code, ec := e.login("Correct-Horse-9", "abcDEF1234"); code != http.StatusUnauthorized || ec != "auth.totp.invalid" {
		t.Fatalf("reused recovery code must fail: %d %s", code, ec)
	}
	us, _ := userstore.New(e.users)
	u, err := us.FindByUsername("alice")
	if err != nil || len(u.RecoveryHashes) != 1 {
		t.Fatalf("one recovery code should remain: %v %d", err, len(u.RecoveryHashes))
	}
}

// Regression (S3): "plain:" and "dev:" hashes were accepted as plaintext.
func TestLoginRejectsPlaintextHashes(t *testing.T) {
	e := newLoginEnv(t, false)
	us, _ := userstore.New(e.users)
	u, _ := us.FindByUsername("alice")
	u.PasswordHash = "plain:letmein"
	if err := us.UpsertUser(u); err != nil {
		t.Fatal(err)
	}
	if code, _ := e.login("letmein", ""); code != http.StatusUnauthorized {
		t.Fatalf("plain: hash accepted: %d", code)
	}
}

// Regression (S2): /auth/totp/setup and /confirm were reachable without
// signing in.
func TestUnauthenticatedTOTPSetupRoutesAreGone(t *testing.T) {
	e := newLoginEnv(t, false)
	for _, p := range []string{"/api/v1/auth/totp/setup", "/api/v1/auth/totp/confirm"} {
		res := httptest.NewRecorder()
		e.r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, p, bytes.NewBufferString(`{"email":"alice"}`)))
		if res.Code == http.StatusOK {
			t.Fatalf("%s still answers 200", p)
		}
	}
}

func refreshCookie(res *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range res.Result().Cookies() {
		if c.Name == cookieRefresh && c.Value != "" {
			return c
		}
	}
	return nil
}

func (e loginEnv) refresh(c *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.AddCookie(c)
	res := httptest.NewRecorder()
	e.r.ServeHTTP(res, req)
	return res
}

// Regression (S6): refresh tokens were never rotated, so a stolen refresh
// cookie stayed valid for 7 days. Each token now works once; replaying a
// spent one revokes the user's sessions.
func TestRefreshTokenRotationAndReuse(t *testing.T) {
	e := newLoginEnv(t, false)
	b, _ := json.Marshal(map[string]any{"username": "alice", "password": "Correct-Horse-9", "rememberMe": true})
	res := httptest.NewRecorder()
	e.r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(b)))
	first := refreshCookie(res)
	if res.Code != http.StatusOK || first == nil {
		t.Fatalf("login: %d, refresh cookie %v", res.Code, first)
	}

	res = e.refresh(first)
	second := refreshCookie(res)
	if res.Code != http.StatusOK || second == nil || second.Value == first.Value {
		t.Fatalf("first refresh: %d, new cookie %v", res.Code, second)
	}
	// Replaying the spent token is refused and revokes the user's sessions,
	// so the newer token stops working too.
	if res = e.refresh(first); res.Code != http.StatusUnauthorized {
		t.Fatalf("replayed refresh token: %d", res.Code)
	}
	if res = e.refresh(second); res.Code != http.StatusUnauthorized {
		t.Fatalf("after reuse, the newer token must be revoked too: %d", res.Code)
	}
}
