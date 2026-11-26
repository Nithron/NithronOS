package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	userstore "nos/backend/nosd/internal/auth/store"
	"nos/backend/nosd/internal/groups"
	"nos/backend/nosd/internal/policy"
	"nos/backend/nosd/internal/shares"
)

type fakeSharesAgent struct {
	calls []agentSharesRequest
	err   error
}

func (f *fakeSharesAgent) PostJSON(_ context.Context, path string, body any, out any) error {
	if path != "/v1/shares/apply" {
		return errors.New("unexpected path " + path)
	}
	f.calls = append(f.calls, body.(agentSharesRequest))
	if f.err != nil {
		return f.err
	}
	return json.Unmarshal([]byte(`{"ok":true,"warnings":[]}`), out)
}

func (f *fakeSharesAgent) last() agentSharesRequest { return f.calls[len(f.calls)-1] }

type sharesEnv struct {
	shares, groups, policy http.Handler
	agent                  *fakeSharesAgent
	store                  *shares.Store
	pol                    *policy.Store
	acc                    accounts
}

func newSharesEnv(t *testing.T) sharesEnv {
	t.Helper()
	dir := t.TempDir()
	users, err := userstore.New(filepath.Join(dir, "users.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []string{"alice", "bob"} {
		if err := users.UpsertUser(userstore.User{ID: "id-" + u, Username: u}); err != nil {
			t.Fatal(err)
		}
	}
	st, _ := shares.NewStore(filepath.Join(dir, "shares.json"))
	gs, _ := groups.NewStore(filepath.Join(dir, "groups.json"))
	pol, _ := policy.NewStore(filepath.Join(dir, "policy.json"))
	acc := accounts{users: users, groups: gs}
	agent := &fakeSharesAgent{}
	ap := &sharesApplier{shares: st, groups: gs, users: acc, agent: agent}
	pass := func(h http.Handler) http.Handler { return h }
	return sharesEnv{
		shares: (&SharesHandler{store: st, groups: gs, users: acc, policy: pol, applier: ap}).Routes(pass),
		groups: (&GroupsHandler{store: gs, shares: st, users: acc, applier: ap}).Routes(pass),
		policy: (&PolicyHandler{store: pol}).Routes(pass),
		agent:  agent, store: st, pol: pol, acc: acc,
	}
}

func call(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var b []byte
	if s, ok := body.(string); ok {
		b = []byte(s)
	} else if body != nil {
		b, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, strings.NewReader(string(b)))
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	return res
}

func TestShareLifecycleAppliesEveryChange(t *testing.T) {
	env := newSharesEnv(t)

	res := call(t, env.shares, http.MethodPost, "/", `{"name":"media","description":"Films",
		"smb":{"enabled":true,"recycle":{"enabled":true}},
		"nfs":{"enabled":true,"networks":["10.0.0.0/8"]},
		"owners":["user:alice"],"readers":["user:bob"]}`)
	if res.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", res.Code, res.Body.String())
	}
	var sh shares.Share
	_ = json.Unmarshal(res.Body.Bytes(), &sh)
	if sh.Path != "/srv/shares/media" || sh.SMB.Recycle.Directory != ".recycle" {
		t.Fatalf("created: %+v", sh)
	}
	sent := env.agent.last()
	if len(sent.Shares) != 1 || sent.Shares[0].Name != "media" || strings.Join(sent.Users, ",") != "alice,bob" {
		t.Fatalf("agent got: %+v", sent)
	}

	// An update changes only what it sends.
	if res := call(t, env.shares, http.MethodPatch, "/media", `{"smb":{"enabled":true,"guest":true}}`); res.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", res.Code, res.Body.String())
	}
	got, _ := env.store.Lookup("media")
	if !got.SMB.Guest || !got.NFSEnabled() || got.Description != "Films" || got.Owners[0] != "user:alice" {
		t.Fatalf("patch changed more than it sent: %+v", got)
	}
	// Lookup by id works too (NithronSync uses ids).
	if res := call(t, env.shares, http.MethodGet, "/"+got.ID, nil); res.Code != http.StatusOK {
		t.Fatalf("get by id: %d", res.Code)
	}
	if res := call(t, env.shares, http.MethodPatch, "/media", `{"name":"other"}`); res.Code != http.StatusBadRequest {
		t.Fatalf("rename: %d", res.Code)
	}

	if res := call(t, env.shares, http.MethodDelete, "/media", nil); res.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", res.Code)
	}
	if n := len(env.agent.last().Shares); n != 0 {
		t.Fatalf("agent still got %d shares after delete", n)
	}
	if res := call(t, env.shares, http.MethodGet, "/media", nil); res.Code != http.StatusNotFound {
		t.Fatalf("get deleted: %d", res.Code)
	}
}

func TestShareValidation(t *testing.T) {
	env := newSharesEnv(t)
	cases := map[string]string{
		"newline in name":     `{"name":"x]\n[global"}`,
		"newline in desc":     `{"name":"x","description":"a\n   guest ok = yes"}`,
		"bad principal":       `{"name":"x","owners":["alice"]}`,
		"unknown user":        `{"name":"x","owners":["user:mallory"]}`,
		"unknown group":       `{"name":"x","readers":["group:nope"]}`,
		"nfs without clients": `{"name":"x","nfs":{"enabled":true}}`,
		"nfs client options":  `{"name":"x","nfs":{"enabled":true,"networks":["*(rw,no_root_squash)"]}}`,
		"recycle dir escape":  `{"name":"x","smb":{"enabled":true,"recycle":{"enabled":true,"directory":"../etc"}}}`,
		"path traversal name": `{"name":"../../etc"}`,
	}
	for name, body := range cases {
		if res := call(t, env.shares, http.MethodPost, "/", body); res.Code != http.StatusBadRequest {
			t.Errorf("%s: got %d (%s)", name, res.Code, res.Body.String())
		}
	}
	if len(env.agent.calls) != 0 {
		t.Fatal("invalid shares must not reach the agent")
	}

	// The test call reports problems without saving.
	res := call(t, env.shares, http.MethodPost, "/new/test", `{"config":{"owners":["user:mallory"]}}`)
	var out struct {
		Valid  bool         `json:"valid"`
		Errors []fieldError `json:"errors"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &out)
	if res.Code != http.StatusOK || out.Valid || out.Errors[0].Code != "shares.unknown_user" {
		t.Fatalf("test: %d %s", res.Code, res.Body.String())
	}
	if _, ok := env.store.Lookup("new"); ok {
		t.Fatal("test must not save")
	}

	if res := call(t, env.shares, http.MethodPost, "/", `{"name":"dup"}`); res.Code != http.StatusCreated {
		t.Fatal(res.Body.String())
	}
	if res := call(t, env.shares, http.MethodPost, "/", `{"name":"dup"}`); res.Code != http.StatusConflict {
		t.Fatalf("duplicate: %d", res.Code)
	}

	// Policy: guests forbidden.
	if res := call(t, env.policy, http.MethodPut, "/", `{"shares":{"guest_access_forbidden":true}}`); res.Code != http.StatusOK {
		t.Fatal(res.Body.String())
	}
	if res := call(t, env.shares, http.MethodPost, "/", `{"name":"guest","smb":{"enabled":true,"guest":true}}`); res.Code != http.StatusBadRequest {
		t.Fatalf("guest share against policy: %d", res.Code)
	}
}

func TestShareSavedWhenAgentFails(t *testing.T) {
	env := newSharesEnv(t)
	env.agent.err = errors.New("agent down")
	res := call(t, env.shares, http.MethodPost, "/", `{"name":"media","smb":{"enabled":true}}`)
	if res.Code != http.StatusBadGateway || !strings.Contains(res.Body.String(), "shares.apply_failed") {
		t.Fatalf("got %d %s", res.Code, res.Body.String())
	}
	if _, ok := env.store.Lookup("media"); !ok {
		t.Fatal("the share must be saved even when applying fails (it applies later)")
	}
}

func TestGroupsAndShareAccess(t *testing.T) {
	env := newSharesEnv(t)
	if res := call(t, env.groups, http.MethodPost, "/", `{"name":"family","members":["bob","mallory"]}`); res.Code != http.StatusBadRequest {
		t.Fatalf("unknown member: %d", res.Code)
	}
	if res := call(t, env.groups, http.MethodPost, "/", `{"name":"users"}`); res.Code != http.StatusBadRequest {
		t.Fatalf("reserved name: %d", res.Code)
	}
	if res := call(t, env.groups, http.MethodPost, "/", `{"name":"family","members":["bob"]}`); res.Code != http.StatusCreated {
		t.Fatalf("create group: %d %s", res.Code, res.Body.String())
	}
	if g := env.agent.last().Groups; len(g) != 1 || g[0].Members[0] != "bob" {
		t.Fatalf("agent got groups %+v", g)
	}
	if res := call(t, env.shares, http.MethodPost, "/", `{"name":"photos","owners":["user:alice"],"readers":["group:family"]}`); res.Code != http.StatusCreated {
		t.Fatal(res.Body.String())
	}
	if res := call(t, env.groups, http.MethodDelete, "/family", nil); res.Code != http.StatusConflict {
		t.Fatalf("deleting a group in use: %d", res.Code)
	}

	// NithronSync and WebDAV resolve device users the same way as Samba.
	sh, _ := env.store.Lookup("photos")
	for id, want := range map[string][2]bool{"id-alice": {true, true}, "id-bob": {true, false}, "id-nobody": {false, false}} {
		if r, w := env.acc.ShareAccess(sh, id); r != want[0] || w != want[1] {
			t.Errorf("%s: read=%v write=%v, want %v", id, r, w, want)
		}
	}
}

// Opening a share to an app is separate from people's access and reaches the
// agent; a later update that doesn't mention apps keeps them.
func TestShareOpenedToApp(t *testing.T) {
	env := newSharesEnv(t)
	res := call(t, env.shares, http.MethodPost, "/", `{"name":"media","owners":["user:alice"],"apps":["nai","nai"]}`)
	if res.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", res.Code, res.Body.String())
	}
	if sent := env.agent.last().Shares[0]; strings.Join(sent.Apps, ",") != "nai" || strings.Join(sent.Readers, ",") != "" {
		t.Fatalf("agent got: %+v", sent)
	}
	if res := call(t, env.shares, http.MethodPatch, "/media", `{"description":"Films"}`); res.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", res.Code, res.Body.String())
	}
	if got, _ := env.store.Lookup("media"); strings.Join(got.Apps, ",") != "nai" {
		t.Fatalf("apps lost on update: %+v", got)
	}
	if res := call(t, env.shares, http.MethodPatch, "/media", `{"apps":[]}`); res.Code != http.StatusOK {
		t.Fatalf("close to apps: %d", res.Code)
	}
	if sent := env.agent.last().Shares[0]; sent.Apps == nil || len(sent.Apps) != 0 {
		t.Fatalf("closed to apps: %+v", sent)
	}
	if res := call(t, env.shares, http.MethodPatch, "/media", `{"apps":["NAI"]}`); res.Code != http.StatusBadRequest {
		t.Fatalf("bad app id: %d %s", res.Code, res.Body.String())
	}
	if res := call(t, env.groups, http.MethodPost, "/", `{"name":"app-nai"}`); res.Code == http.StatusCreated {
		t.Fatal("a people group may not take an app group's name")
	}
}
