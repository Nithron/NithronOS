package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	userstore "nithronos/backend/nosd/internal/auth/store"
	"nithronos/backend/nosd/internal/config"
)

func (e loginEnv) signIn(t *testing.T) (cookies []*http.Cookie, csrf string) {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"username": "alice", "password": "Correct-Horse-9"})
	res := httptest.NewRecorder()
	e.r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(b)))
	if res.Code != http.StatusOK {
		t.Fatalf("login: %d %s", res.Code, res.Body.String())
	}
	cookies = res.Result().Cookies()
	for _, c := range cookies {
		if c.Name == cookieCSRF {
			csrf = c.Value
		}
	}
	return cookies, csrf
}

func (e loginEnv) call(method, path, body string, cookies []*http.Cookie, csrf string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	for _, c := range cookies {
		req.AddCookie(c)
	}
	req.Header.Set("X-CSRF-Token", csrf)
	res := httptest.NewRecorder()
	e.r.ServeHTTP(res, req)
	return res
}

// Changing a password used to panic (it read a context key nothing set). Now
// it works and ends the user's other sessions, keeping the current one.
func TestChangePasswordEndsOtherSessions(t *testing.T) {
	e := newLoginEnv(t, false)
	here, csrf := e.signIn(t)
	elsewhere, _ := e.signIn(t)

	res := e.call(http.MethodPost, "/api/v1/users/u-alice/password", `{"current_password":"Correct-Horse-9","new_password":"Even-Better-Horse-10"}`, here, csrf)
	if res.Code != http.StatusOK {
		t.Fatalf("change password: %d %s", res.Code, res.Body.String())
	}
	if res := e.call(http.MethodGet, "/api/v1/auth/me", "", here, ""); res.Code != http.StatusOK {
		t.Fatalf("current session after password change: %d", res.Code)
	}
	if res := e.call(http.MethodGet, "/api/v1/auth/me", "", elsewhere, ""); res.Code != http.StatusUnauthorized {
		t.Fatalf("other session after password change: %d", res.Code)
	}
}

// Signing out ends this session only, and its cookies stop working at once.
func TestLogoutEndsOnlyThisSession(t *testing.T) {
	e := newLoginEnv(t, false)
	here, csrf := e.signIn(t)
	elsewhere, _ := e.signIn(t)
	if res := e.call(http.MethodPost, "/api/v1/auth/logout", "", here, csrf); res.Code != http.StatusNoContent {
		t.Fatalf("logout: %d", res.Code)
	}
	if res := e.call(http.MethodGet, "/api/v1/auth/me", "", here, ""); res.Code != http.StatusUnauthorized {
		t.Fatalf("signed-out cookies still work: %d", res.Code)
	}
	if res := e.call(http.MethodGet, "/api/v1/auth/me", "", elsewhere, ""); res.Code != http.StatusOK {
		t.Fatalf("logout signed out another device: %d", res.Code)
	}
}

type revokeRecorder struct{ all []string }

func (r *revokeRecorder) RevokeAll(uid string) error        { r.all = append(r.all, uid); return nil }
func (r *revokeRecorder) RevokeOthers(string, string) error { return nil }

func TestDeleteUserRevokesSessions(t *testing.T) {
	us, err := userstore.New(filepath.Join(t.TempDir(), "users.json"))
	if err != nil {
		t.Fatal(err)
	}
	_ = us.UpsertUser(userstore.User{ID: "u-admin", Username: "admin1", Roles: []string{"admin"}})
	_ = us.UpsertUser(userstore.User{ID: "u-bob", Username: "bob", Roles: []string{"user"}})
	rec := &revokeRecorder{}
	r := chi.NewRouter()
	r.Mount("/", NewUsersHandler(us, rec, config.Defaults()).Routes())
	res := httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest(http.MethodDelete, "/u-bob", nil))
	if res.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", res.Code, res.Body.String())
	}
	if len(rec.all) != 1 || rec.all[0] != "u-bob" {
		t.Fatalf("sessions not revoked: %v", rec.all)
	}
}
