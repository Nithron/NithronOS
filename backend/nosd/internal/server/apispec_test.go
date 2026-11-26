package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"nos/backend/nosd/internal/config"
)

func TestListRoutesNormalizesAndSorts(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/api/v1/pools/{id:[0-9]+}/", func(http.ResponseWriter, *http.Request) {})
	r.Post("/api/v1/auth/login", func(http.ResponseWriter, *http.Request) {})
	r.Get("/api/v1/auth/login", func(http.ResponseWriter, *http.Request) {})
	got := ListRoutes(r)
	want := []APIRoute{
		{"GET", "/api/v1/auth/login"},
		{"POST", "/api/v1/auth/login"},
		{"GET", "/api/v1/pools/{id}"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("route %d: got %v, want %v", i, got[i], want[i])
		}
	}
	doc := OpenAPIDocument(got)
	op := doc["paths"].(map[string]map[string]any)["/api/v1/pools/{id}"]["get"].(map[string]any)
	if op["operationId"] != "getPoolsId" || op["parameters"] == nil {
		t.Fatalf("operation: %v", op)
	}
}

// /api/v1/openapi.json used to be a stub without paths; it now describes the
// routes the server really has.
func TestServedOpenAPIListsRealRoutes(t *testing.T) {
	t.Setenv("NOS_TEST_SKIP_AUTH", "1")
	r := NewRouter(config.FromEnv())
	res := httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status %d", res.Code)
	}
	var doc struct {
		Paths map[string]map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	for path, method := range map[string]string{"/api/v1/auth/login": "post", "/api/v1/openapi.json": "get", "/api/v1/setup/state": "get"} {
		if _, ok := doc.Paths[path][method]; !ok {
			t.Errorf("%s %s missing from the served document", method, path)
		}
	}
}
