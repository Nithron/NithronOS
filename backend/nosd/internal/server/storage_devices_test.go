package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nos/backend/nosd/internal/config"
)

func TestStorageDevicesEndpoint(t *testing.T) {
	r := NewRouter(config.FromEnv())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/devices", nil)
	res := httptest.NewRecorder()

	r.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	var body struct {
		Devices []map[string]any `json:"devices"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body.Devices == nil {
		t.Fatalf("devices missing")
	}
}
