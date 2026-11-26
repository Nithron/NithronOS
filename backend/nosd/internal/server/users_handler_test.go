package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func (e loginEnv) signInAs(t *testing.T, user, pass string) (cookies []*http.Cookie, csrf string) {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"username": user, "password": pass})
	res := httptest.NewRecorder()
	e.r.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(b)))
	if res.Code != http.StatusOK {
		t.Fatalf("login %s: %d %s", user, res.Code, res.Body.String())
	}
	for _, c := range res.Result().Cookies() {
		if c.Name == cookieCSRF {
			csrf = c.Value
		}
	}
	return res.Result().Cookies(), csrf
}

// A user is created under the name given, with a file-sharing account that
// has the same password; bad names, roles and system refusals create nothing.
func TestCreateUserMakesTheSMBAccount(t *testing.T) {
	e := newLoginEnv(t, false)
	admin, csrf := e.signIn(t)

	res := e.call(http.MethodPost, "/api/v1/users", `{"username":"bob","password":"Bob-Secret-1","roles":["user"]}`, admin, csrf)
	if res.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", res.Code, res.Body.String())
	}
	var bob UserAccount
	_ = json.Unmarshal(res.Body.Bytes(), &bob)
	if bob.Username != "bob" || strings.Join(bob.Roles, ",") != "user" || strings.Contains(res.Body.String(), "email") {
		t.Fatalf("created: %s", res.Body.String())
	}
	smb := e.agent.called("/v1/smb/user-create")
	if len(smb) != 1 || smb[0].Body["username"] != "bob" || smb[0].Body["password"] != "Bob-Secret-1" {
		t.Fatalf("SMB account: %+v", smb)
	}

	for body, want := range map[string]int{
		`{"username":"bob@example.com","password":"Bob-Secret-1"}`:          http.StatusBadRequest,
		`{"username":"1bob","password":"Bob-Secret-1"}`:                     http.StatusBadRequest,
		`{"username":"carol","password":"short"}`:                           http.StatusBadRequest,
		`{"username":"carol","password":"Line\nBreak-1"}`:                   http.StatusBadRequest,
		`{"username":"carol","password":"Carol-Secret-1","roles":["root"]}`: http.StatusBadRequest,
		`{"username":"bob","password":"Bob-Secret-1"}`:                      http.StatusConflict,
	} {
		if res := e.call(http.MethodPost, "/api/v1/users", body, admin, csrf); res.Code != want {
			t.Errorf("%s: %d, want %d (%s)", body, res.Code, want, res.Body.String())
		}
	}

	// The system refuses the name (a daemon account): no nOS user either.
	e.agent.fail["/v1/smb/user-create"] = "not a regular user account"
	res = e.call(http.MethodPost, "/api/v1/users", `{"username":"daemon","password":"Daemon-Pass-1"}`, admin, csrf)
	if res.Code != http.StatusBadGateway || !strings.Contains(res.Body.String(), "not a regular user account") {
		t.Fatalf("refused account: %d %s", res.Code, res.Body.String())
	}
	delete(e.agent.fail, "/v1/smb/user-create")
	list := e.call(http.MethodGet, "/api/v1/users", "", admin, "")
	if strings.Contains(list.Body.String(), `"daemon"`) {
		t.Fatalf("refused user was saved: %s", list.Body.String())
	}
}

// Roles change through PUT (and nothing else does: an "email" no longer
// renames the account); the last administrator stays one.
func TestUpdateUserRolesOnly(t *testing.T) {
	e := newLoginEnv(t, false)
	admin, csrf := e.signIn(t)
	if res := e.call(http.MethodPut, "/api/v1/users/u-alice", `{"roles":["user"]}`, admin, csrf); res.Code != http.StatusConflict {
		t.Fatalf("demote the last admin: %d %s", res.Code, res.Body.String())
	}
	res := e.call(http.MethodPost, "/api/v1/users", `{"username":"bob","password":"Bob-Secret-1"}`, admin, csrf)
	var bob UserAccount
	_ = json.Unmarshal(res.Body.Bytes(), &bob)
	res = e.call(http.MethodPut, "/api/v1/users/"+bob.ID, `{"roles":["admin"],"email":"mallory"}`, admin, csrf)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"username":"bob"`) || !strings.Contains(res.Body.String(), `"admin"`) {
		t.Fatalf("promote: %d %s", res.Code, res.Body.String())
	}
	if res := e.call(http.MethodPut, "/api/v1/users/"+bob.ID, `{"roles":["superuser"]}`, admin, csrf); res.Code != http.StatusBadRequest {
		t.Fatalf("unknown role: %d", res.Code)
	}
}

// A user changes their own password (no admin needed); an admin resets
// another's. Both reach SMB; deleting a user removes their SMB access.
func TestPasswordsAndDeleteReachSMB(t *testing.T) {
	e := newLoginEnv(t, false)
	admin, csrf := e.signIn(t)
	res := e.call(http.MethodPost, "/api/v1/users", `{"username":"bob","password":"Bob-Secret-1"}`, admin, csrf)
	var bob UserAccount
	_ = json.Unmarshal(res.Body.Bytes(), &bob)

	bobC, bobCSRF := e.signInAs(t, "bob", "Bob-Secret-1")
	if res := e.call(http.MethodGet, "/api/v1/users", "", bobC, ""); res.Code != http.StatusForbidden {
		t.Fatalf("a user listed users: %d", res.Code)
	}
	if res := e.call(http.MethodPost, "/api/v1/auth/password", `{"current_password":"wrong-one-1","new_password":"Bob-Better-2"}`, bobC, bobCSRF); res.Code != http.StatusUnauthorized {
		t.Fatalf("wrong current password: %d", res.Code)
	}
	if res := e.call(http.MethodPost, "/api/v1/auth/password", `{"current_password":"Bob-Secret-1","new_password":"Bob-Better-2"}`, bobC, bobCSRF); res.Code != http.StatusOK {
		t.Fatalf("own password: %d %s", res.Code, res.Body.String())
	}
	if res := e.call(http.MethodPost, "/api/v1/users/"+bob.ID+"/reset-password", `{"password":"Bob-Reset-3x"}`, admin, csrf); res.Code != http.StatusNoContent {
		t.Fatalf("reset: %d %s", res.Code, res.Body.String())
	}
	var pw []string
	for _, c := range e.agent.called("/v1/smb/user-create") {
		pw = append(pw, c.Body["password"].(string))
	}
	if strings.Join(pw, ",") != "Bob-Secret-1,Bob-Better-2,Bob-Reset-3x" {
		t.Fatalf("SMB passwords: %v", pw)
	}

	// A refused SMB password changes nothing.
	e.agent.fail["/v1/smb/user-create"] = "smbpasswd failed"
	if res := e.call(http.MethodPost, "/api/v1/users/"+bob.ID+"/reset-password", `{"password":"Bob-Other-4x"}`, admin, csrf); res.Code != http.StatusBadGateway {
		t.Fatalf("refused reset: %d", res.Code)
	}
	delete(e.agent.fail, "/v1/smb/user-create")
	e.signInAs(t, "bob", "Bob-Reset-3x")

	if res := e.call(http.MethodDelete, "/api/v1/users/"+bob.ID, "", admin, csrf); res.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", res.Code)
	}
	if del := e.agent.called("/v1/smb/user-delete"); len(del) != 1 || del[0].Body["username"] != "bob" {
		t.Fatalf("SMB delete: %+v", del)
	}
}
