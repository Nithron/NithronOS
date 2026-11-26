package server

import (
	"encoding/json"
	"net/http"

	"nos/backend/nosd/pkg/agentclient"
	"nos/backend/nosd/pkg/httpx"
)

// POST /api/v1/pools/scrub/start { mount }
func handleScrubStart(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mount string `json:"mount"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Mount == "" {
		httpx.WriteError(w, http.StatusBadRequest, "mount required")
		return
	}
	// Busy: use mount as lock key
	if cur := currentPoolTx(body.Mount); cur != "" {
		httpx.WriteError(w, http.StatusConflict, `{"error":{"code":"pool.busy","txId":"`+cur+`"}}`)
		return
	}
	client := agentclient.New("/run/nos-agent.sock")
	var out map[string]any
	if err := client.PostJSON(r.Context(), "/v1/btrfs/scrub/start", body, &out); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, out)
}

// GET /api/v1/pools/scrub/status?mount=...
func handleScrubStatus(w http.ResponseWriter, r *http.Request) {
	mount := r.URL.Query().Get("mount")
	if mount == "" {
		httpx.WriteError(w, http.StatusBadRequest, "mount required")
		return
	}
	client := agentclient.New("/run/nos-agent.sock")
	var out map[string]any
	// forward as GET with query
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, "http://unix/v1/btrfs/scrub/status?mount="+mount, nil)
	res, err := client.HTTP.Do(req)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		httpx.WriteError(w, res.StatusCode, "agent error")
		return
	}
	_ = json.NewDecoder(res.Body).Decode(&out)
	writeJSON(w, out)
}
