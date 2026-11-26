package server

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"nos/backend/nosd/pkg/agentclient"
	"nos/backend/nosd/pkg/httpx"

	"github.com/go-chi/chi/v5"
)

// GET /api/v1/pools/{id}
func handlePoolDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		httpx.WriteError(w, http.StatusBadRequest, "id required")
		return
	}
	// Simplify: return usage only; UI already has pool list. In a real system we'd query a store.
	// Query usage from agent if mount path is provided via query (?mount=)
	mount := r.URL.Query().Get("mount")
	if mount == "" {
		httpx.WriteError(w, http.StatusBadRequest, "mount required for usage")
		return
	}
	client := agentclient.New("/run/nos-agent.sock")
	var usage map[string]any
	// GET /v1/btrfs/usage?mount=...
	ureq, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, "http://unix/v1/btrfs/usage?mount="+filepath.Clean(mount), nil)
	res, err := client.HTTP.Do(ureq)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		httpx.WriteError(w, res.StatusCode, "agent error")
		return
	}
	_ = json.NewDecoder(res.Body).Decode(&usage)
	writeJSON(w, map[string]any{"usage": usage})
}
