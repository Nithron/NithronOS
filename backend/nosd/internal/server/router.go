package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nos/backend/nosd/internal/version"
	"os"
	"os/exec"
	"time"

	"github.com/google/uuid"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"

	"nos/backend/nosd/internal/config"
	"nos/backend/nosd/pkg/agentclient"
	// "nos/backend/nosd/pkg/firewall"
)

// AgentClient interface for nos-agent interactions
type AgentClient interface {
	GetJSON(ctx context.Context, path string, out interface{}) error
	PostJSON(ctx context.Context, path string, body interface{}, out interface{}) error
}

// agentMetricsClient implements AgentMetricsClient to fetch text metrics from nos-agent
type agentMetricsClient struct{ socket string }

func (a agentMetricsClient) FetchMetrics(ctx context.Context) ([]byte, error) {
	cli := agentclient.New(a.socket)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/metrics", nil)
	res, err := cli.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return io.ReadAll(res.Body)
}

// zerolog's time format is a package global: set it once, not on every
// Logger call while other goroutines are logging (a data race).
func init() { zerolog.TimeFieldFormat = time.RFC3339 }

func Logger(cfg config.Config) *zerolog.Logger {
	logger := zerolog.New(os.Stderr).Level(logLevel()).With().Timestamp().Logger()
	return &logger
}

// NewRouter builds the nosd HTTP API. The routes live in routes_*.go, one
// file per area, as methods on routeDeps.
func NewRouter(cfg config.Config) http.Handler {
	h, _ := NewServer(cfg)
	return h
}

// NewServer builds the API and returns, with it, the background work the
// running daemon starts once (tests and route-dump only build the router).
func NewServer(cfg config.Config) (http.Handler, func()) {
	d := newRouteDeps(cfg)
	start := func() {
		if d.monitorStart != nil {
			d.monitorStart(context.Background())
		}
		if d.backupStart != nil {
			d.backupStart(context.Background())
		}
		if d.updates != nil {
			// An upgrade restarts nosd; follow it again.
			d.updates.Resume()
		}
		if d.sharesApplier != nil {
			// The host's Samba/NFS configuration and ACLs follow the stores
			// (this also moves systems upgraded from per-share files).
			go d.sharesApplier.applyAtStartup()
		}
	}

	// chi requires every middleware before the first route. (The metrics
	// routes used to be registered before withSession, so enabling metrics
	// made nosd panic at startup.)
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	// No RealIP middleware: it replaced RemoteAddr with the client-supplied
	// X-Forwarded-For, which let remote clients pass the localhost-only checks
	// and dodge per-IP rate limits. RemoteAddr stays the real peer; clientIP()
	// honours X-Forwarded-For only from a trusted proxy, and isLocalRequest()
	// guards the local-only endpoints.
	r.Use(zerologMiddleware(Logger(cfg), cfg))
	r.Use(securityHeaders)
	// Dynamic CORS based on runtime config
	SetRuntimeCORSOrigin(cfg.CORSOrigin)
	r.Use(DynamicCORS)
	// Identify the signed-in user once per request, against the server-side
	// session store (see sessionauth.go).
	r.Use(withSession(d.mgr, cfg))

	r.Get("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"ok": true, "version": version.Version})
	})
	d.registerObservability(r)
	d.registerRecovery(r)
	d.registerFirstBootReset(r)
	d.registerAgent(r)
	d.registerSetup(r)
	d.registerOpenAPI(r)
	d.registerAuth(r)
	d.registerProtected(r) // everything that needs a signed-in user
	d.registerSystem(r)

	// Log route inventory once on startup for visibility (method + path)
	func() {
		var routes []map[string]string
		_ = chi.Walk(r, func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
			routes = append(routes, map[string]string{"method": method, "path": route})
			return nil
		})
		if b, err := json.Marshal(routes); err == nil {
			Logger(cfg).Info().RawJSON("api_routes", b).Msg("")
		}
	}()
	return r, start
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		fmt.Printf("Failed to write response: %v\n", err)
	}
}

func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// generateUUID returns a random (version 4) UUID from crypto/rand. It is used
// for IDs that must not be guessable: per-agent auth tokens, session records,
// user IDs. (It used to be derived from the clock, with the last 8 bytes always
// zero, so agent tokens could be predicted from the registration time.)
func generateUUID() string { return uuid.NewString() }

func hasRole(roles []string, role string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}
