package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"
)

func TestUpdatesPlan_Routes_Methods(t *testing.T) {
	// Ensure apt-get missing to avoid external deps and still get 200 on non-Windows
	oldPath := os.Getenv("PATH")
	_ = os.Setenv("PATH", "")
	t.Cleanup(func() { _ = os.Setenv("PATH", oldPath) })

	mux := buildMux()

	// GET should be 405
	rrGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/v1/updates/plan", nil)
	mux.ServeHTTP(rrGet, reqGet)
	if rrGet.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /v1/updates/plan expected 405, got %d", rrGet.Code)
	}

	// POST should be 200 on non-Windows, 501 on Windows (not implemented)
	body := UpdatesPlanRequest{}
	b, _ := json.Marshal(body)
	rrPost := httptest.NewRecorder()
	reqPost := httptest.NewRequest(http.MethodPost, "/v1/updates/plan", bytes.NewReader(b))
	mux.ServeHTTP(rrPost, reqPost)
	if runtime.GOOS == "windows" {
		if rrPost.Code != http.StatusNotImplemented {
			t.Fatalf("POST /v1/updates/plan expected 501 on windows, got %d", rrPost.Code)
		}
	} else {
		if rrPost.Code != http.StatusOK {
			t.Fatalf("POST /v1/updates/plan expected 200, got %d body=%s", rrPost.Code, rrPost.Body.String())
		}
	}
}

func TestUpdatesApply_Route(t *testing.T) {
	// Ensure apt-get missing to avoid running apt
	oldPath := os.Getenv("PATH")
	_ = os.Setenv("PATH", "")
	t.Cleanup(func() { _ = os.Setenv("PATH", oldPath) })

	mux := buildMux()

	// GET should be 405
	rrGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/v1/updates/apply", nil)
	mux.ServeHTTP(rrGet, reqGet)
	if rrGet.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /v1/updates/apply expected 405, got %d", rrGet.Code)
	}

	// Without apt-get (PATH is empty here) there is nothing to run: 501,
	// not a pretend success.
	body := UpdatesApplyRequest{}
	b, _ := json.Marshal(body)
	rrPost := httptest.NewRecorder()
	reqPost := httptest.NewRequest(http.MethodPost, "/v1/updates/apply", bytes.NewReader(b))
	mux.ServeHTTP(rrPost, reqPost)
	if rrPost.Code != http.StatusNotImplemented {
		t.Fatalf("POST /v1/updates/apply without apt-get: expected 501, got %d body=%s", rrPost.Code, rrPost.Body.String())
	}
	// Package names can't carry apt options.
	bad, _ := json.Marshal(UpdatesApplyRequest{Packages: []string{"-o=APT::Foo=1"}})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/updates/apply", bytes.NewReader(bad)))
	if rr.Code != http.StatusBadRequest && runtime.GOOS != "windows" {
		t.Fatalf("bad package name: %d", rr.Code)
	}
}

func TestUpdateUnitStatus(t *testing.T) {
	cases := map[string]string{
		"LoadState=not-found\nActiveState=inactive\n":                                                    "none",
		"LoadState=loaded\nActiveState=activating\nInvocationID=abc\nResult=success\n":                   "running",
		"LoadState=loaded\nActiveState=active\nSubState=exited\nInvocationID=abc\nResult=success\n":      "succeeded",
		"LoadState=loaded\nActiveState=failed\nInvocationID=abc\nResult=exit-code\nExecMainStatus=100\n": "failed",
	}
	for out, want := range cases {
		if got := unitStatus(parseShow(out)); got.State != want {
			t.Errorf("%q: %s, want %s", out, got.State, want)
		}
	}
	if st := unitStatus(parseShow("ActiveState=failed\nInvocationID=x\nExecMainStatus=100\n")); st.ExitStatus != 100 {
		t.Errorf("exit status: %d", st.ExitStatus)
	}
	cmd := updateCommand(nil)
	if cmd[0] != "systemd-run" || cmd[len(cmd)-2] != "--with-new-pkgs" || cmd[len(cmd)-1] != "upgrade" {
		t.Errorf("full upgrade: %v", cmd)
	}
	cmd = updateCommand([]string{"nosd"})
	if cmd[len(cmd)-3] != "install" || cmd[len(cmd)-2] != "--only-upgrade" || cmd[len(cmd)-1] != "nosd" {
		t.Errorf("one package: %v", cmd)
	}
}
