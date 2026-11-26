package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"nos/backend/nosd/internal/pools"
	"nos/backend/nosd/pkg/agentclient"
)

type fakeBtrfsAgent struct {
	posts []string
	fail  map[string]*agentclient.HTTPError // by mount
}

func (f *fakeBtrfsAgent) GetJSON(_ context.Context, path string, out any) error {
	switch {
	case strings.HasPrefix(path, "/v1/btrfs/scrub/status?mount=%2Fmnt%2Fa"):
		return json.Unmarshal([]byte(`{"status":"Scrub started:    Thu Nov 26 10:00:00 2026\nStatus:           running\nTotal to scrub:   2.00GiB\nBytes scrubbed:   1.00GiB  (50.00%)\nError summary:    no errors found\n"}`), out)
	case strings.HasPrefix(path, "/v1/btrfs/scrub/status"):
		return json.Unmarshal([]byte(`{"status":"UUID: x\n\tno stats available\n"}`), out)
	case strings.HasPrefix(path, "/v1/btrfs/balance/status?mount=%2Fmnt%2Fb"):
		return json.Unmarshal([]byte(`{"running":true,"percent":12,"left":"7","total":"8"}`), out)
	default:
		return json.Unmarshal([]byte(`{"running":false}`), out)
	}
}

func (f *fakeBtrfsAgent) PostJSON(_ context.Context, path string, body any, _ any) error {
	mount := body.(map[string]string)["mount"]
	f.posts = append(f.posts, path+" "+mount)
	if e := f.fail[mount]; e != nil {
		return e
	}
	return nil
}

func maintenanceRouter(t *testing.T, agent *fakeBtrfsAgent) http.Handler {
	t.Helper()
	old := listPoolsFn
	listPoolsFn = func(context.Context) ([]pools.Pool, error) {
		return []pools.Pool{{ID: "a", UUID: "uuid-a", Mount: "/mnt/a"}, {ID: "b", Mount: "/mnt/b"}, {ID: "unmounted"}}, nil
	}
	t.Cleanup(func() { listPoolsFn = old })
	r := chi.NewRouter()
	poolMaintenance{agent: agent}.routes(r, func(h http.Handler) http.Handler { return h })
	return r
}

func TestScrubAndBalanceStatusAreReal(t *testing.T) {
	r := maintenanceRouter(t, &fakeBtrfsAgent{})
	res := call(t, r, http.MethodGet, "/api/v1/scrub/status", nil)
	var scrubs []ScrubStatus
	_ = json.Unmarshal(res.Body.Bytes(), &scrubs)
	if res.Code != 200 || len(scrubs) != 2 {
		t.Fatalf("scrub status: %d %s", res.Code, res.Body.String())
	}
	if a := scrubs[0]; a.Status != "running" || *a.Progress != 50 || *a.BytesTotal != 2<<30 || a.StartedAt == nil {
		t.Fatalf("pool a: %+v", a)
	}
	if b := scrubs[1]; b.Status != "idle" || b.Progress != nil {
		t.Fatalf("pool b (never scrubbed): %+v", b)
	}

	res = call(t, r, http.MethodGet, "/api/v1/balance/status", nil)
	var bal []BalanceStatus
	_ = json.Unmarshal(res.Body.Bytes(), &bal)
	if len(bal) != 2 || bal[0].Status != "idle" || bal[1].Status != "running" || *bal[1].Progress != 12 {
		t.Fatalf("balance status: %s", res.Body.String())
	}

	res = call(t, r, http.MethodGet, "/api/v1/pools/scrub/status?mount=/mnt/a", nil)
	var one ScrubStatus
	if _ = json.Unmarshal(res.Body.Bytes(), &one); one.Mount != "/mnt/a" {
		t.Fatalf("one pool: %s", res.Body.String())
	}
	if res := call(t, r, http.MethodGet, "/api/v1/pools/scrub/status?mount=/etc", nil); res.Code != http.StatusNotFound {
		t.Fatalf("unknown mount: %d", res.Code)
	}
}

func TestScrubAndBalanceOps(t *testing.T) {
	agent := &fakeBtrfsAgent{fail: map[string]*agentclient.HTTPError{"/mnt/b": {Status: http.StatusConflict, Body: "scrub already running"}}}
	r := maintenanceRouter(t, agent)

	// No body: every mounted pool. One already running still counts as done.
	res := call(t, r, http.MethodPost, "/api/v1/scrub/start", nil)
	if res.Code != 200 || len(agent.posts) != 2 || !strings.Contains(res.Body.String(), "already running") {
		t.Fatalf("start all: %d %s %v", res.Code, res.Body.String(), agent.posts)
	}

	agent.posts = nil
	res = call(t, r, http.MethodPost, "/api/v1/balance/cancel", `{"poolId":"uuid-a"}`)
	if res.Code != 200 || len(agent.posts) != 1 || agent.posts[0] != "/v1/btrfs/balance/cancel /mnt/a" {
		t.Fatalf("cancel by uuid: %d %v", res.Code, agent.posts)
	}

	// Only conflicts: 409.
	if res := call(t, r, http.MethodPost, "/api/v1/scrub/start", `{"mount":"/mnt/b"}`); res.Code != http.StatusConflict {
		t.Fatalf("already running: %d", res.Code)
	}
	// A named pool that doesn't exist is never "every pool".
	agent.posts = nil
	if res := call(t, r, http.MethodPost, "/api/v1/balance/start", `{"mount":"/"}`); res.Code != http.StatusNotFound || len(agent.posts) != 0 {
		t.Fatalf("unknown mount: %d %v", res.Code, agent.posts)
	}
	// A pool busy with a device operation isn't started.
	poolLockMu.Lock()
	poolHeldByTx["/mnt/a"] = "tx-1"
	poolLockMu.Unlock()
	t.Cleanup(func() {
		poolLockMu.Lock()
		delete(poolHeldByTx, "/mnt/a")
		poolLockMu.Unlock()
	})
	if res := call(t, r, http.MethodPost, "/api/v1/balance/start", `{"mount":"/mnt/a"}`); res.Code != http.StatusConflict || len(agent.posts) != 0 {
		t.Fatalf("busy pool: %d %v", res.Code, agent.posts)
	}
}
