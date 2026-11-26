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

	pwhash "nos/backend/nosd/internal/auth/hash"
	userstore "nos/backend/nosd/internal/auth/store"
	"nos/backend/nosd/internal/config"
	"nos/backend/nosd/pkg/auth"
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
		env.secret, _, err = auth.GenerateTOTPSecret("nOS", "alice")
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
	// The token that was just replaced still works for a few seconds
	// (two tabs refreshing at once) ...
	if res = e.refresh(first); res.Code != http.StatusOK {
		t.Fatalf("concurrent refresh with the previous token: %d", res.Code)
	}
	res = e.refresh(second)
	third := refreshCookie(res)
	if res.Code != http.StatusOK || third == nil {
		t.Fatalf("second refresh: %d", res.Code)
	}
	// ... but a token two rotations old is a copy: refused, and every
	// session of the user ends, so the newest token stops working too.
	if res = e.refresh(first); res.Code != http.StatusUnauthorized {
		t.Fatalf("replayed refresh token: %d", res.Code)
	}
	if res = e.refresh(third); res.Code != http.StatusUnauthorized {
		t.Fatalf("after reuse, the newer token must be revoked too: %d", res.Code)
	}
}

// The settings page's /auth/2fa/* used an in-memory manager that treated
// everyone as "user-123", accepted any password, and was never checked at
// login. It now uses the user store, and enrollment only takes effect after
// a valid code.
func TestSettings2FAUsesUserStore(t *testing.T) {
	e := newLoginEnv(t, false)
	b, _ := json.Marshal(map[string]string{"username": "alice", "password": "Correct-Horse-9"})
	res := httptest.NewRecorder()
	e.r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(b)))
	cookies := res.Result().Cookies()
	call := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		for _, c := range cookies {
			req.AddCookie(c)
		}
		out := httptest.NewRecorder()
		e.r.ServeHTTP(out, req)
		return out
	}
	if res := call(http.MethodPost, "/api/v1/auth/2fa/enroll", `{"password":"wrong"}`); res.Code != http.StatusUnauthorized {
		t.Fatalf("enroll with a wrong password: %d", res.Code)
	}
	res = call(http.MethodPost, "/api/v1/auth/2fa/enroll", `{"password":"Correct-Horse-9"}`)
	var enr struct {
		Secret      string   `json:"secret"`
		BackupCodes []string `json:"backup_codes"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &enr)
	if res.Code != http.StatusOK || enr.Secret == "" || len(enr.BackupCodes) == 0 {
		t.Fatalf("enroll: %d %s", res.Code, res.Body.String())
	}
	if code, _ := e.login("Correct-Horse-9", ""); code != http.StatusOK {
		t.Fatalf("pending enrollment must not change login yet: %d", code)
	}
	valid, _ := totp.GenerateCode(enr.Secret, time.Now())
	if res := call(http.MethodPost, "/api/v1/auth/2fa/verify", `{"code":"`+valid+`"}`); res.Code != http.StatusOK {
		t.Fatalf("verify: %d %s", res.Code, res.Body.String())
	}
	if code, ec := e.login("Correct-Horse-9", ""); code != http.StatusUnauthorized || ec != "auth.totp.required" {
		t.Fatalf("after enabling, login must ask for the code: %d %s", code, ec)
	}
	if code, _ := e.login("Correct-Horse-9", enr.BackupCodes[0]); code != http.StatusOK {
		t.Fatalf("backup code from enrollment: %d", code)
	}
}
