//go:build prommetrics

package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsEndpointExposesRegistry(t *testing.T) {
	initMetrics()
	// Register a sample counter via our helper path
	incBtrfsStatus("balance")
	h := metricsHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	if !strings.Contains(string(body), "nos_agent_btrfs_status_calls_total") {
		t.Fatalf("expected metric name in output")
	}
}
