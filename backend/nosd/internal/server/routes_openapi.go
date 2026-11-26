package server

import (
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	// "nos/backend/nosd/pkg/firewall"
)

// registerOpenAPI serves the OpenAPI document, generated from the router on
// the first request (it used to be a stub with no paths).
func (d *routeDeps) registerOpenAPI(r chi.Router) {
	var (
		once sync.Once
		doc  map[string]any
	)
	r.Get("/api/v1/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
		once.Do(func() { doc = OpenAPIDocument(ListRoutes(r)) })
		writeJSON(w, doc)
	})
}
