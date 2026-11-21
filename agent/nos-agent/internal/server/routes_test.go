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

	// POST should be 200 on non-Windows; 501 on Windows
	body := UpdatesApplyRequest{}
	b, _ := json.Marshal(body)
	rrPost := httptest.NewRecorder()
	reqPost := httptest.NewRequest(http.MethodPost, "/v1/updates/apply", bytes.NewReader(b))
	mux.ServeHTTP(rrPost, reqPost)
	if runtime.GOOS == "windows" {
		if rrPost.Code != http.StatusNotImplemented {
			t.Fatalf("POST /v1/updates/apply expected 501 on windows, got %d", rrPost.Code)
		}
	} else {
		if rrPost.Code != http.StatusOK {
			t.Fatalf("POST /v1/updates/apply expected 200, got %d body=%s", rrPost.Code, rrPost.Body.String())
		}
	}
}
