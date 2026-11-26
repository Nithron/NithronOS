package server

import (
	"net/http"
	"strings"

	"nos/backend/nosd/pkg/agentclient"
	"nos/backend/nosd/pkg/httpx"
)

func handleSmartProxy(w http.ResponseWriter, r *http.Request) {
	dev := r.URL.Query().Get("device")
	if dev == "" || !strings.HasPrefix(dev, "/dev/") || strings.ContainsAny(dev, " \t\n\r\x00") {
		httpx.WriteError(w, http.StatusBadRequest, "invalid device")
		return
	}
	client := agentclient.New("/run/nos-agent.sock")
	var out map[string]any
	if err := client.GetJSON(r.Context(), "/v1/smart?device="+dev, &out); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, out)
}
