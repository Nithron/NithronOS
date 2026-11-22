package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"nithronos/backend/nosd/internal/config"
)

func TestApplyDevice_ParallelOneBusy(t *testing.T) {
	// Ensure no prior lock
	releasePoolLock("p1")
	t.Cleanup(txWorkers.Wait)

	r := NewRouter(config.FromEnv())
	body := map[string]any{
		"steps":   []map[string]string{{"id": "s1", "description": "add", "command": "btrfs device add /dev/sdb /mnt/p1"}},
		"confirm": "ADD",
	}
	b, _ := json.Marshal(body)

	// fire two requests concurrently
	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(ix int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/pools/p1/apply-device", bytes.NewReader(b))
			req.Header.Set("X-CSRF-Token", "x")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			codes[ix] = w.Code
		}(i)
	}
	wg.Wait()

	// exactly one should be 409
	var c409, c200 int
	for _, c := range codes {
		if c == http.StatusConflict {
			c409++
		}
		if c >= 200 && c < 300 {
			c200++
		}
	}
	if c409 != 1 || c200 != 1 {
		t.Fatalf("expected one 200 and one 409, got %v", codes)
	}
}
