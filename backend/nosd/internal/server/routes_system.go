package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"nithronos/backend/nosd/pkg/agentclient"

	// "nithronos/backend/nosd/pkg/firewall"
	"nithronos/backend/nosd/pkg/httpx"
)

// registerSystem serves system settings (/api/v1/system, /network, /telemetry); during setup the setup token is accepted too.
func (d *routeDeps) registerSystem(r chi.Router) {
	cfg := d.cfg
	// System configuration endpoints (outside auth for setup access)
	// During setup, these need to work without authentication
	systemConfigHandler := NewSystemConfigHandler(*Logger(cfg), agentclient.New(cfg.AgentSocket()))
	r.Route("/api/v1/system", func(sr chi.Router) {
		// Allow setup token authentication for system config during setup
		sr.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// During first-boot, always allow authenticated users; also allow setup token.
				// Only require normal auth after setup is complete.
				setupCompleteFile := filepath.Join(cfg.EtcDir, "nos", "setup-complete")

				// Check for normal session first (using same logic as adminRequired)
				uid, ok := signedInUID(r)

				if ok && uid != "" {
					// Valid session found, proceed
					next.ServeHTTP(w, r)
					return
				}

				// No session, check if setup is complete
				if _, err := os.Stat(setupCompleteFile); err == nil {
					// Setup is complete, authentication is required
					httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.required", "Authentication required. Please sign in.", 0)
					return
				}

				// Setup not complete: allow with setup token for CLI/tools
				authz := r.Header.Get("Authorization")
				if strings.HasPrefix(authz, "Bearer ") {
					tok := strings.TrimSpace(authz[7:])
					if claims, err := setupDecodeToken(cfg, tok); err == nil && claims["purpose"] == "setup" {
						next.ServeHTTP(w, r)
						return
					}
				}

				// Otherwise unauthorized
				httpx.WriteTypedError(w, http.StatusUnauthorized, "auth.required", "Authentication required.", 0)
			})
		})
		// Mount system info/services
		sys := NewSystemHandler()
		sr.Get("/info", sys.GetSystemInfo)
		sr.Get("/services", sys.GetServices)
		// Mount system config endpoints under their specific paths
		// Hostname
		sr.Get("/hostname", systemConfigHandler.GetHostname)
		sr.Post("/hostname", systemConfigHandler.SetHostname)
		// Timezone
		sr.Get("/timezone", systemConfigHandler.GetTimezone)
		sr.Post("/timezone", systemConfigHandler.SetTimezone)
		sr.Get("/timezones", systemConfigHandler.ListTimezones)
		// NTP
		sr.Get("/ntp", systemConfigHandler.GetNTP)
		sr.Post("/ntp", systemConfigHandler.SetNTP)
		// Network (system-scoped)
		sr.Get("/network/interfaces", systemConfigHandler.ListInterfaces)
		sr.Get("/network/interfaces/{iface}", systemConfigHandler.GetInterface)
		sr.Post("/network/interfaces/{iface}", systemConfigHandler.ConfigureInterface)
		// Telemetry
		sr.Get("/telemetry/consent", systemConfigHandler.GetTelemetryConsent)
		sr.Post("/telemetry/consent", systemConfigHandler.SetTelemetryConsent)
		// System metrics endpoint expected by FE; reuse system health
		sr.Get("/metrics", handleSystemHealth(cfg))
		// Mount system config endpoints
		sr.Mount("/", systemConfigHandler.Routes())
	})

	// Network endpoints to match FE contract: /api/v1/network/interfaces
	r.Route("/api/v1/network", func(nr chi.Router) {
		// Require auth for network configuration
		nr.Use(requireAuth)
		nr.Get("/interfaces", systemConfigHandler.ListInterfaces)
		nr.Get("/interfaces/{iface}", systemConfigHandler.GetInterface)
		nr.Post("/interfaces/{iface}", systemConfigHandler.ConfigureInterface)
	})

	// Telemetry endpoints to match FE contract: /api/v1/telemetry/consent
	r.Route("/api/v1/telemetry", func(tr chi.Router) {
		tr.Use(requireAuth)
		tr.Get("/consent", systemConfigHandler.GetTelemetryConsent)
		tr.Post("/consent", systemConfigHandler.SetTelemetryConsent)
	})

}
