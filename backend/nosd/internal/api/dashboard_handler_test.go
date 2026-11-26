package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDashboardUsesItsSources(t *testing.T) {
	h := Dashboard(DashboardSources{
		Shares: func() []ShareInfo { return []ShareInfo{{Name: "media", Proto: "SMB", Path: "/srv/shares/media", State: "active"}} },
	})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil))
	var out DashboardResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Shares) != 1 || out.Shares[0].Name != "media" {
		t.Fatalf("shares: %+v", out.Shares)
	}
	if out.Apps == nil || len(out.Apps) != 0 {
		t.Fatalf("apps without a source must be an empty list, not made-up apps: %+v", out.Apps)
	}
}
