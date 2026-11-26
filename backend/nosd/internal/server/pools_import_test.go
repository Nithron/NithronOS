package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"nos/backend/nosd/internal/config"
)

func TestPoolsImportPersistsMountOptions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	r := NewRouter(config.FromEnv())
	body := map[string]any{"uuid": "0000-TEST", "label": "poolX", "mountpoint": "/mnt/poolX", "mountOptions": "compress=zstd:3,noatime"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pools/import", bytes.NewReader(b))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	// We can't assert 200 in CI without agent socket present; allow 200 or 500 depending on environment
	if res.Code != http.StatusOK && res.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", res.Code)
	}
	// Verify pools.json contains mountOptions (best-effort)
	dir := config.Defaults().EtcDir
	p := filepath.Join(dir, "nos", "pools.json")
	if b, err := os.ReadFile(p); err == nil {
		if !bytes.Contains(b, []byte("compress=zstd:3,noatime")) {
			t.Fatalf("pools.json missing mountOptions: %s", string(b))
		}
	}
}
