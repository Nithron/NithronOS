package server

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// fakeAgent answers nos-agent's socket for tests that build the whole
// server; it records each call's path and body.
type fakeAgent struct {
	mu    sync.Mutex
	calls []fakeAgentCall
	// fail makes a path answer with this agent error.
	fail map[string]string
}

type fakeAgentCall struct {
	Path string
	Body map[string]any
}

func startFakeAgent(t *testing.T) *fakeAgent {
	t.Helper()
	dir, err := os.MkdirTemp("", "nosag")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "a.sock")
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeAgent{fail: map[string]string{}}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.calls = append(f.calls, fakeAgentCall{Path: r.URL.Path, Body: body})
		msg, failing := f.fail[r.URL.Path]
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if failing {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"warnings":[]}`))
	})}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = srv.Close() })
	t.Setenv("NOS_AGENT_SOCKET", sock)
	return f
}

func (f *fakeAgent) called(path string) []fakeAgentCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []fakeAgentCall
	for _, c := range f.calls {
		if c.Path == path {
			out = append(out, c)
		}
	}
	return out
}
