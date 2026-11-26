package server

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	// "nos/backend/nosd/pkg/firewall"

	"nos/backend/nosd/internal/fsatomic"
)

// registerAgent lets nos-agent register with the bootstrap token.
func (d *routeDeps) registerAgent(r chi.Router) {
	cfg := d.cfg
	// Agent registration (bootstrap trust)
	r.Post("/api/v1/agents/register", func(w http.ResponseWriter, r *http.Request) {
		if !cfg.AllowAgentRegistration {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		var body struct {
			Token string `json:"token"`
			Node  string `json:"node"`
			Arch  string `json:"arch"`
			OS    string `json:"os"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		// compare against bootstrap token
		bootTok, _ := os.ReadFile("/etc/nos/agent-token")
		want := strings.TrimSpace(string(bootTok))
		if want == "" || subtle.ConstantTimeCompare([]byte(strings.TrimSpace(body.Token)), []byte(want)) != 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// rotate per-agent token and persist (very simple JSON list)
		agentsPath := filepath.Join("/var/lib/nos", "agents.json")
		type agentRec struct{ ID, Token, Node, Arch, OS, CreatedAt string }
		var list []agentRec
		if b, err := os.ReadFile(agentsPath); err == nil {
			_ = json.Unmarshal(b, &list)
		}
		id := generateUUID()
		tok := generateUUID()
		rec := agentRec{ID: id, Token: tok, Node: body.Node, Arch: body.Arch, OS: body.OS, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
		list = append(list, rec)
		_ = os.MkdirAll(filepath.Dir(agentsPath), 0o755)
		_ = fsatomic.SaveJSON(agentsPath, list, 0o600)
		writeJSON(w, map[string]any{"id": id, "token": tok})
	})

}
