package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nithronos/backend/nosd/internal/config"
)

func TestApplyDeviceConfirmValidation(t *testing.T) {
	t.Setenv("NOS_TEST_FAST_POLL", "1")
	t.Cleanup(txWorkers.Wait) // runs before the env vars above are restored
	r := NewRouter(config.FromEnv())
	// wrong confirm
	body := map[string]any{
		"steps":   []map[string]string{{"id": "s1", "description": "add", "command": "btrfs device add /dev/sdb /mnt/p"}},
		"confirm": "NOPE",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pools/p1/apply-device", bytes.NewReader(b))
	req.Header.Set("X-CSRF-Token", "x")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusPreconditionRequired && res.Code != http.StatusBadRequest {
		t.Fatalf("expected 412/400, got %d", res.Code)
	}

	// correct confirm
	body["confirm"] = "ADD"
	b2, _ := json.Marshal(body)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/pools/p1/apply-device", bytes.NewReader(b2))
	req2.Header.Set("X-CSRF-Token", "x")
	res2 := httptest.NewRecorder()
	r.ServeHTTP(res2, req2)
	if res2.Code < 200 || res2.Code >= 300 { // accept 2xx as OK for async start
		t.Fatalf("unexpected status: %d", res2.Code)
	}
}
