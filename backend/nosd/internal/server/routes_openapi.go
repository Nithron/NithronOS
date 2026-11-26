package server

import (
	"net/http"
	"nithronos/backend/nosd/internal/version"

	"github.com/go-chi/chi/v5"
	// "nithronos/backend/nosd/pkg/firewall"
)

// registerOpenAPI serves the OpenAPI document.
func (d *routeDeps) registerOpenAPI(r chi.Router) {
	// Serve minimal OpenAPI JSON for v1 at /api/v1/openapi.json
	r.Get("/api/v1/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]any{
			"openapi": "3.0.3",
			"info":    map[string]any{"title": "NithronOS API", "version": version.Version},
			"servers": []any{map[string]any{"url": "/api/v1"}},
		})
	})

}
