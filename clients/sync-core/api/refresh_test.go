package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"nos/clients/sync-core/config"
)

// A server that keeps refusing the token gets one refresh, then an error
// (the client used to refresh and retry for as long as it got 401).
func TestRefreshesOnceOn401(t *testing.T) {
	var refreshes, calls atomic.Int32
	var sent []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/sync/devices/refresh" {
			refreshes.Add(1)
			_, _ = w.Write([]byte(`{"device_token":"nos_dt_new","refresh_token":"nos_rt_new"}`))
			return
		}
		calls.Add(1)
		sent = append(sent, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	cfg, err := config.LoadFrom(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Update(func(c *config.Config) {
		c.ServerURL, c.DeviceID, c.DeviceToken, c.RefreshToken = srv.URL, "dev1", "nos_dt_paired", "nos_rt_old"
	}); err != nil {
		t.Fatal(err)
	}
	if err := NewClient(cfg).HealthCheck(context.Background()); err == nil {
		t.Fatal("a 401 after the refresh wasn't an error")
	}
	if refreshes.Load() != 1 || calls.Load() != 2 {
		t.Fatalf("refreshes %d, calls %d; want 1 and 2", refreshes.Load(), calls.Load())
	}
	// The paired token first, then the refreshed one, which is kept.
	if sent[0] != "Bearer nos_dt_paired" || sent[1] != "Bearer nos_dt_new" || cfg.GetDeviceToken() != "nos_dt_new" {
		t.Fatalf("sent %q, kept %q", sent, cfg.GetDeviceToken())
	}
}
