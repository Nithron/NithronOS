package server

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"nithronos/backend/nosd/internal/api"
	"nithronos/backend/nosd/internal/apps"
	pwhash "nithronos/backend/nosd/internal/auth/hash"
	"nithronos/backend/nosd/internal/auth/session"
	userstore "nithronos/backend/nosd/internal/auth/store"
	"nithronos/backend/nosd/internal/config"
	"nithronos/backend/nosd/internal/disks"
	"nithronos/backend/nosd/internal/notifications"
	"nithronos/backend/nosd/internal/pools"
	"nithronos/backend/nosd/internal/ratelimit"
	"nithronos/backend/nosd/internal/shares"
	"nithronos/backend/nosd/internal/sessions"
	"nithronos/backend/nosd/pkg/agentclient"
	"nithronos/backend/nosd/pkg/auth"

	// "nithronos/backend/nosd/pkg/firewall"
	"nithronos/backend/nosd/pkg/httpx"
	poolroots "nithronos/backend/nosd/pkg/pools"

	// "nithronos/backend/nosd/pkg/shares" // TODO: Restore when integrating old shares
	"nithronos/backend/nosd/pkg/snapdb"

	"nithronos/backend/nosd/internal/fsatomic"

	"strconv"

	firstboot "nithronos/backend/nosd/internal/setup/firstboot"

	"github.com/gorilla/securecookie"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
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

func Logger(cfg config.Config) *zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339
	level := currentLevel
	logger := zerolog.New(os.Stderr).Level(level).With().Timestamp().Logger()
	return &logger
}

func NewRouter(cfg config.Config) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(zerologMiddleware(Logger(cfg), cfg))
	r.Use(securityHeaders)

	// Dynamic CORS based on runtime config
	SetRuntimeCORSOrigin(cfg.CORSOrigin)
	r.Use(DynamicCORS)

	// Observability endpoints: metrics and pprof
	if cfg.MetricsEnabled {
		r.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
			// very simple allowlist by exact ip match or prefix
			if len(cfg.MetricsAllowlist) > 0 {
				ip := clientIP(r, cfg)
				allowed := false
				for _, a := range cfg.MetricsAllowlist {
					if a == ip || (strings.HasSuffix(a, ".") && strings.HasPrefix(ip, a)) {
						allowed = true
						break
					}
				}
				if !allowed {
					w.WriteHeader(http.StatusForbidden)
					return
				}
			}
			w.Header().Set("Content-Type", "text/plain; version=0.0.4")
			var b strings.Builder
			b.WriteString("nosd_up 1\n")
			// pool metrics (best-effort)
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if list, err := pools.ListPools(ctx); err == nil {
				var total uint64
				var used uint64
				for _, p := range list {
					total += p.Size
					used += p.Used
				}
				b.WriteString(fmt.Sprintf("pool_total_bytes %d\n", total))
				b.WriteString(fmt.Sprintf("pool_used_bytes %d\n", used))
			}
			// SMART metrics for common devices (best-effort)
			for _, dev := range []string{"/dev/sda", "/dev/nvme0n1"} {
				client := agentclient.New("/run/nos-agent.sock")
				var out map[string]any
				if err := client.GetJSON(r.Context(), "/v1/smart?device="+dev, &out); err == nil {
					if t, ok := out["temperature_c"].(float64); ok {
						b.WriteString(fmt.Sprintf("smart_disk_temp_celsius{dev=\"%s\"} %g\n", dev, t))
					}
					if st, ok := out["passed"].(bool); ok {
						if st {
							b.WriteString(fmt.Sprintf("smart_pass{dev=\"%s\"} 1\n", dev))
						} else {
							b.WriteString(fmt.Sprintf("smart_pass{dev=\"%s\"} 0\n", dev))
						}
					}
				}
			}
			// Btrfs tx progress (best-effort gauges set by executor)
			if p := currentBalancePercent(); p >= 0 {
				b.WriteString(fmt.Sprintf("btrfs_balance_percent %g\n", p))
			}
			if p := currentReplacePercent(); p >= 0 {
				b.WriteString(fmt.Sprintf("btrfs_replace_percent %g\n", p))
			}
			_, _ = w.Write([]byte(b.String()))
		})
		// Combined metrics endpoint: nosd + agent
		r.Get("/metrics/all", func(w http.ResponseWriter, r *http.Request) {
			NewCombinedMetricsHandler(prom.DefaultGatherer, agentMetricsClient{socket: cfg.AgentSocket()}).ServeHTTP(w, r)
		})
	}

	if cfg.PprofEnabled {
		// Guard pprof: localhost only
		r.Mount("/debug/pprof", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			if i := strings.LastIndex(ip, ":"); i >= 0 {
				ip = ip[:i]
			}
			if ip != "127.0.0.1" && ip != "::1" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			http.DefaultServeMux.ServeHTTP(w, r)
		}))
	}

	// Init stores
	store, _ := auth.NewStore(cfg.UsersPath)
	users, _ := userstore.New(cfg.UsersPath)
	codec := auth.NewSessionCodec(cfg.SessionHashKey, cfg.SessionBlockKey)
	InitJobsStore(cfg)

	// Initialize shares handler
	agentClient := agentclient.New(cfg.AgentSocket())
	sharesStorePath := filepath.Join(filepath.Dir(cfg.UsersPath), "shares.json")
	sharesHandler, err := NewSharesHandlerV2(sharesStorePath, agentClient)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize shares handler")
	}

	// Initialize backup handler (using existing implementation)
	// The existing backup handler requires scheduler, replicator, and restorer
	// For now, we'll skip initializing it as it needs more complex setup
	var backupHandler *BackupHandler

	// Initialize notifications manager
	notificationsPath := filepath.Join(filepath.Dir(cfg.UsersPath), "notifications")
	notificationManager, err := notifications.NewManager(notificationsPath)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize notifications manager")
	}

	// Initialize apps manager
	appManagerConfig := &apps.Config{
		AppsRoot:      "/srv/apps",
		StateFile:     filepath.Join(filepath.Dir(cfg.UsersPath), "apps.json"),
		CatalogPath:   "/usr/share/nithronos/apps",
		CachePath:     "/var/lib/nos/apps/catalog.cache.json",
		SourcesPath:   "/etc/nos/apps/catalogs.d",
		TemplatesPath: "/usr/share/nithronos/apps",
		AgentPath:     cfg.AgentSocket(),
		CaddyPath:     "/etc/caddy/Caddyfile.d",
	}
	if v := os.Getenv("NOS_APPS_STATE"); v != "" {
		appManagerConfig.StateFile = v
	}
	appsManager, _ := apps.NewManager(appManagerConfig)
	// Disk-backed session and ratelimit stores
	sessStore := sessions.New(cfg.SessionsPath)
	rlStore := ratelimit.New(cfg.RateLimitPath)
	mgr := session.New(cfg.SessionsPath)

	// On startup: if first boot and OTP exists/valid, log it
	func() {
		// Determine if setup complete by checking users on disk (fresh load)
		us, _ := userstore.New(cfg.UsersPath)
		if us == nil || !us.HasAdmin() {
			// Load first-boot OTP state
			var st struct {
				OTP       string `json:"otp"`
				CreatedAt string `json:"created_at"`
				Used      bool   `json:"used"`
			}
			if b, err := os.ReadFile(cfg.FirstBootPath); err == nil {
				_ = json.Unmarshal(b, &st)
				if st.OTP != "" && !st.Used {
					if t, err := time.Parse(time.RFC3339, st.CreatedAt); err == nil && time.Since(t) < 15*time.Minute {
						Logger(cfg).Info().Msgf("First-boot OTP: %s (valid 15m)", st.OTP)
					}
				}
			}
		}
	}()

	// Session verification middleware for server-side binding (non-enforcing)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uid, sid, ok := decodeSessionParts(r, cfg)
			if !ok || uid == "" || sid == "" {
				next.ServeHTTP(w, r)
				return
			}
			ua := r.Header.Get("User-Agent")
			ip := clientIP(r, cfg)
			if id, ok2 := mgr.Verify(sid, ua, ip); !ok2 || id != uid {
				// do not enforce; continue without rejecting
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// SessionContext middleware (parse cookies only; no auth enforcement)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// parse nos_session; ignore failures
			uid, sid, ok := decodeSessionParts(r, cfg)
			if ok {
				// attach to context via headers (lightweight)
				r.Header.Set("X-UID", uid)
				r.Header.Set("X-SID", sid)
			}
			next.ServeHTTP(w, r)
		})
	})

	r.Get("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"ok": true, "version": "0.9.5-pre-alpha"})
	})

	// Health monitoring endpoints (for real-time data)
	r.Get("/api/v1/health/system", handleSystemHealth(cfg))
	r.Get("/api/v1/health/disks", handleDiskHealth(cfg))
	r.Get("/api/v1/monitoring/system", handleSystemHealth(cfg)) // Reuse system health for monitoring

	// Metrics endpoints allowed without /api prefix for tech monitoring
	r.Get("/metrics", handleSystemHealth(cfg))
	r.Get("/metrics/all", handleMetricsStream(cfg))

	// Dashboard endpoints (v1)
	r.Get("/api/v1/dashboard", api.HandleDashboard)
	r.Get("/api/v1/storage/summary", api.HandleStorageSummary)
	r.Get("/api/v1/health/disks/summary", api.HandleDisksSummary)
	r.Get("/api/v1/events/recent", api.HandleRecentEvents)
	r.Get("/api/v1/maintenance/status", api.HandleMaintenanceStatus)

	// Storage: block device inventory
	r.Get("/api/v1/storage/devices", handleListDevices)
	// SMART health proxy
	r.Get("/api/v1/health/smart", handleSmartProxy)

	// Storage: block device inventory
	r.Get("/api/v1/storage/devices", handleListDevices)

	// Recovery routes (localhost only)
	if cfg.RecoveryMode {
		r.Route("/api/v1/recovery", func(rr chi.Router) {
			rr.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ip := r.RemoteAddr
					if i := strings.LastIndex(ip, ":"); i >= 0 {
						ip = ip[:i]
					}
					if ip != "127.0.0.1" && ip != "::1" {
						w.WriteHeader(http.StatusForbidden)
						return
					}
					next.ServeHTTP(w, r)
				})
			})
			rr.Post("/reset-password", func(w http.ResponseWriter, r *http.Request) {
				var body struct{ Username, Password string }
				_ = json.NewDecoder(r.Body).Decode(&body)
				if strings.TrimSpace(body.Username) == "" || strings.TrimSpace(body.Password) == "" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				users, err := userstore.New(cfg.UsersPath)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				u, err := users.FindByUsername(strings.ToLower(body.Username))
				if err != nil {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				h, herr := pwhash.HashPassword(body.Password)
				if herr != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				u.PasswordHash = h
				u.LockedUntil = ""
				u.FailedAttempts = 0
				_ = users.UpsertUser(u)
				writeJSON(w, map[string]any{"ok": true})
			})
			rr.Post("/disable-2fa", func(w http.ResponseWriter, r *http.Request) {
				var body struct{ Username string }
				_ = json.NewDecoder(r.Body).Decode(&body)
				if strings.TrimSpace(body.Username) == "" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				users, err := userstore.New(cfg.UsersPath)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				u, err := users.FindByUsername(strings.ToLower(body.Username))
				if err != nil {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				u.TOTPEnc = ""
				u.RecoveryHashes = nil
				_ = users.UpsertUser(u)
				writeJSON(w, map[string]any{"ok": true})
			})
			rr.Post("/generate-otp", func(w http.ResponseWriter, r *http.Request) {
				// Regenerate a one-time setup OTP (best-effort)
				var st struct {
					OTP       string `json:"otp"`
					CreatedAt string `json:"created_at"`
					Used      bool   `json:"used"`
				}
				st.OTP = genOTP6()
				st.CreatedAt = time.Now().UTC().Format(time.RFC3339)
				st.Used = false
				_ = os.MkdirAll(filepath.Dir(cfg.FirstBootPath), 0o755)
				_ = fsatomic.SaveJSON(r.Context(), cfg.FirstBootPath, st, 0o600)
				_ = writeFirstBootOTPFile(st.OTP)
				writeJSON(w, map[string]any{"otp": st.OTP})
			})
		})
	}

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
		if len(bootTok) == 0 || strings.TrimSpace(body.Token) != strings.TrimSpace(string(bootTok)) {
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
		_ = fsatomic.SaveJSON(r.Context(), agentsPath, list, 0o600)
		writeJSON(w, map[string]any{"id": id, "token": tok})
	})

	// Setup routes are always registered under /api/v1, but gated with 410 when setup is complete
	r.Route("/api/v1/setup", func(sr chi.Router) {
		sr.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Allow /complete endpoint to bypass the check
				if strings.HasSuffix(r.URL.Path, "/complete") {
					next.ServeHTTP(w, r)
					return
				}
				// Evaluate setup completion from disk on every request (robust against file changes)
				us, _ := userstore.New(cfg.UsersPath)
				if us != nil && us.HasAdmin() {
					httpx.WriteTypedError(w, http.StatusGone, "setup.complete", "Setup already completed", 0)
					return
				}
				next.ServeHTTP(w, r)
			})
		})
		sr.Get("/state", func(w http.ResponseWriter, r *http.Request) {
			// Compute firstBoot and whether an OTP is currently required (exists and valid)
			firstBoot := true
			if us, _ := userstore.New(cfg.UsersPath); us != nil && us.HasAdmin() {
				firstBoot = false
			}
			otpRequired := false
			if st, err := firstboot.New(cfg.FirstBootPath).Load(); err == nil && st != nil {
				if time.Now().Before(st.ExpiresAt) && st.OTP != "" {
					otpRequired = true
				}
			}
			writeJSON(w, map[string]any{"firstBoot": firstBoot, "otpRequired": otpRequired})
		})

		// Rate limiter (persisted): per-IP cfg.RateOTPPerMin per minute for setup endpoints
		sr.Post("/otp/verify", func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r, cfg)
			otpWin := time.Duration(cfg.RateOTPWindowSec) * time.Second
			if otpWin <= 0 {
				otpWin = time.Minute
			}
			ok1, rem1, reset1 := rlStore.Allow("otp:ip:"+ip, cfg.RateOTPPerMin, otpWin)
			if !ok1 {
				retry := int(time.Until(reset1).Seconds())
				Logger(cfg).Warn().Str("event", "rate.limited").Str("route", "/api/v1/setup/otp/verify").Str("key", "otp:ip:"+ip).Int("remaining", rem1).Int("retryAfterSec", retry).Msg("")
				httpx.WriteTypedError(w, http.StatusTooManyRequests, "rate.limited", "Too many attempts. Try later.", retry)
				return
			}
			var body struct{ OTP string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			if len(body.OTP) != 6 {
				httpx.WriteTypedError(w, http.StatusBadRequest, "setup.otp.invalid", "Enter the 6-digit code", 0)
				return
			}
			st, err := firstboot.New(cfg.FirstBootPath).Load()
			if err != nil {
				if os.IsPermission(err) {
					httpx.WriteTypedError(w, http.StatusInternalServerError, "storage_error", "setup storage not writable", 0)
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if st == nil || st.OTP == "" || st.OTP != body.OTP {
				httpx.WriteTypedError(w, http.StatusBadRequest, "setup.otp.invalid", "Invalid one-time code", 0)
				return
			}
			if time.Now().After(st.ExpiresAt) {
				httpx.WriteTypedError(w, http.StatusGone, "setup.otp.expired", "Your code expired. Request a new one.", 0)
				return
			}
			payload := map[string]any{"purpose": "setup", "exp": time.Now().Add(10 * time.Minute).UTC().Format(time.RFC3339)}
			val, err := setupEncodeToken(cfg, payload)
			if err != nil {
				Logger(cfg).Error().Str("event", "setup.token.error").Err(err).Msg("")
				httpx.WriteTypedError(w, http.StatusInternalServerError, "secret_unreadable", "secret.key not readable", 0)
				return
			}
			// Announce/update OTP in runtime file for systemd announcer (best-effort)
			_ = writeFirstBootOTPFile(st.OTP)
			// Set setup session cookie under /api/v1/setup
			secure := isSecureRequest(r, cfg)
			writeSetupCookie(w, val, 10*time.Minute, secure)
			writeJSON(w, map[string]any{"ok": true, "token": val})
		})

		// First admin creation (consumes setup token)
		sr.With(requireSetupAuth(cfg)).Post("/first-admin", func(w http.ResponseWriter, r *http.Request) {
			if users == nil {
				httpx.WriteTypedError(w, http.StatusInternalServerError, "store.lock", "User store unavailable", 0)
				return
			}
			var body struct {
				Username   string `json:"username"`
				Password   string `json:"password"`
				EnableTOTP bool   `json:"enable_totp"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			uname := strings.TrimSpace(body.Username)
			if !validUsername(uname) {
				httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid username", 0)
				return
			}
			if !validPassword(body.Password) {
				httpx.WriteTypedError(w, http.StatusBadRequest, "input.weak_password", "Choose a stronger password", 0)
				return
			}
			if _, err := users.FindByUsername(uname); err == nil {
				httpx.WriteTypedError(w, http.StatusConflict, "input.username_taken", "Username is taken", 0)
				return
			}
			phc, err := pwhash.HashPassword(body.Password)
			if err != nil {
				Logger(cfg).Error().Str("event", "setup.hash.error").Err(err).Msg("")
				httpx.WriteTypedError(w, http.StatusInternalServerError, "store.atomic_fail", "Internal error", 0)
				return
			}
			now := time.Now().UTC().Format(time.RFC3339)
			u := userstore.User{ID: generateUUID(), Username: uname, PasswordHash: phc, Roles: []string{"admin"}, CreatedAt: now, UpdatedAt: now}
			if body.EnableTOTP {
				u.TOTPEnc = "pending"
			}
			if err := users.UpsertUser(u); err != nil {
				code := "store.atomic_fail"
				if os.IsPermission(err) || os.IsNotExist(err) || strings.Contains(strings.ToLower(err.Error()), "permission denied") {
					code = "setup.write_failed"
					// Emit clear hint for permission issues
					info := dirPermInfo(filepath.Dir(cfg.UsersPath))
					Logger(cfg).Error().Str("event", "setup.persist.error").Str("code", code).Str("hint", info).Err(err).Msg("")
				} else {
					Logger(cfg).Error().Str("event", "setup.persist.error").Str("code", code).Err(err).Msg("")
				}
				httpx.WriteErrorWithDetails(w, http.StatusInternalServerError, code, "Service cannot write /etc/nos/users.json", map[string]any{"path": cfg.UsersPath})
				return
			}
			// Success: remove first-boot state so OTP stops printing on restarts (best-effort)
			_ = os.Remove(cfg.FirstBootPath)
			// Remove OTP files (best-effort)
			_ = os.Remove("/tmp/nos-otp")
			_ = os.Remove("/etc/nos/otp")
			_ = os.Remove("/run/nos/firstboot-otp")
			// Remove MOTD hint if present (best-effort)
			_ = os.Remove("/etc/motd.d/10-nithronos-otp")
			// success; return 200 to advance UI reliably
			w.WriteHeader(http.StatusOK)
		})

		// Mark setup as complete - called after all setup steps are done
		sr.With(requireSetupAuth(cfg)).Post("/complete", func(w http.ResponseWriter, r *http.Request) {
			// Check if already complete
			setupCompleteFile := filepath.Join(cfg.EtcDir, "nos", "setup-complete")
			if _, err := os.Stat(setupCompleteFile); err == nil {
				// Already marked complete, just return success
				w.WriteHeader(http.StatusNoContent)
				return
			}

			// Create setup-complete marker file
			dir := filepath.Dir(setupCompleteFile)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				Logger(cfg).Error().Err(err).Str("dir", dir).Msg("Failed to create setup-complete directory")
				httpx.WriteTypedError(w, http.StatusInternalServerError, "setup.write_failed", "Failed to mark setup as complete", 0)
				return
			}
			if err := os.WriteFile(setupCompleteFile, []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o644); err != nil {
				Logger(cfg).Error().Err(err).Str("file", setupCompleteFile).Msg("Failed to write setup-complete file")
				httpx.WriteTypedError(w, http.StatusInternalServerError, "setup.write_failed", "Failed to mark setup as complete", 0)
				return
			}

			// Also remove the firstboot state
			_ = os.Remove(cfg.FirstBootPath)
			_ = os.Remove("/tmp/nos-otp")
			_ = os.Remove("/etc/nos/otp")
			_ = os.Remove("/run/nos/firstboot-otp")
			// Clear setup cookie now that setup is complete
			clearSetupCookie(w)
			w.WriteHeader(http.StatusNoContent)
		})
	})

	// Recovery: local-only endpoint to clear first-boot state and optionally users
	r.Post("/api/v1/setup/recover", func(w http.ResponseWriter, r *http.Request) {
		// Guard: localhost only
		ip := r.RemoteAddr
		if i := strings.LastIndex(ip, ":"); i >= 0 {
			ip = ip[:i]
		}
		if ip != "127.0.0.1" && ip != "::1" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		var body struct {
			Confirm     string `json:"confirm"`
			DeleteUsers bool   `json:"delete_users"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if strings.ToLower(strings.TrimSpace(body.Confirm)) != "yes" {
			httpx.WriteTypedError(w, http.StatusPreconditionRequired, "confirm.required", "confirm=yes required", 0)
			return
		}
		// Best-effort deletes
		_ = os.Remove(cfg.FirstBootPath)
		_ = os.Remove("/tmp/nos-otp")
		_ = os.Remove("/etc/nos/otp")
		_ = os.Remove("/run/nos/firstboot-otp")
		if body.DeleteUsers {
			_ = os.Remove(cfg.UsersPath)
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	// Remove legacy login limiter seed (persisted store is the single source of truth)
	// (intentionally left blank)

	// Serve minimal OpenAPI JSON for v1 at /api/v1/openapi.json
	r.Get("/api/v1/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"openapi":"3.0.3","info":{"title":"NithronOS API","version":"0.9.5-pre-alpha"},"servers":[{"url":"/api/v1"}]}`))
	})

	// (Removed legacy unversioned first-admin handler; canonical handler is under /api/v1/setup in the block above.)

	// Auth (legacy + new store integration)

	r.Post("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Username   string `json:"username"`
			Password   string `json:"password"`
			Code       string `json:"code"`
			RememberMe bool   `json:"rememberMe"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		uname := strings.TrimSpace(body.Username)
		pass := body.Password

		// Apply rate limiting first (before any other checks)
		ip := clientIP(r, cfg)
		loginWin := time.Duration(cfg.RateLoginWindowSec) * time.Second
		if loginWin <= 0 {
			loginWin = 15 * time.Minute
		}
		okIP, _, resetIP := rlStore.Allow("login:ip:"+ip, cfg.RateLoginPer15m, loginWin)
		okUser, _, resetUser := rlStore.Allow("login:user:"+strings.ToLower(uname), cfg.RateLoginPer15m, loginWin)
		if !okIP || !okUser {
			retry := resetIP
			if time.Until(resetUser) > 0 && resetUser.After(retry) {
				retry = resetUser
			}
			Logger(cfg).Warn().Str("event", "rate.limited").Str("key", "login").Str("ip", ip).Int("limit", cfg.RateLoginPer15m).Time("resetAt", retry).Msg("")
			w.Header().Set("Retry-After", strconv.Itoa(int(time.Until(retry).Seconds())))
			httpx.WriteError(w, http.StatusTooManyRequests, `{"error":{"code":"rate.limited","retryAfterSec":`+strconv.Itoa(int(time.Until(retry).Seconds()))+`}}`)
			return
		}

		// During setup, allow login if admin exists (needed for steps 4-7)
		// Only block login if no admin exists yet
		us, _ := userstore.New(cfg.UsersPath)
		if us != nil && !us.HasAdmin() {
			// No admin yet, cannot login
			httpx.WriteTypedError(w, http.StatusForbidden, "setup.required", "System setup required. Please create an admin account first.", 0)
			return
		}
		u, err := users.FindByUsername(uname)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Check account lock
		if u.LockedUntil != "" {
			if t, err := time.Parse(time.RFC3339, u.LockedUntil); err == nil && time.Now().Before(t) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		ph := u.PasswordHash
		ok := false
		if strings.HasPrefix(ph, "dev:") || strings.HasPrefix(ph, "plain:") {
			ok = strings.TrimPrefix(strings.TrimPrefix(ph, "dev:"), "plain:") == pass
		} else {
			ok = pwhash.VerifyPassword(ph, pass)
		}
		if !ok {
			// increment failure; lock after 10
			u.FailedAttempts++
			if u.FailedAttempts >= 10 {
				u.FailedAttempts = 0
				u.LockedUntil = time.Now().Add(15 * time.Minute).UTC().Format(time.RFC3339)
			}
			_ = users.UpsertUser(u)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// success: reset counters
		u.FailedAttempts = 0
		u.LockedUntil = ""
		_ = users.UpsertUser(u)
		if err := issueSessionCookies(w, cfg, u.ID, body.RememberMe); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "session error")
			return
		}
		// persist session record (best-effort)
		_ = sessStore.Upsert(sessions.Session{ID: generateUUID(), UserID: u.ID, Roles: u.Roles, ExpiresAt: time.Now().Add(15 * time.Minute).UTC().Format(time.RFC3339)})
		// bind server-side session
		ua := r.Header.Get("User-Agent")
		ip = clientIP(r, cfg)
		rec, _ := mgr.Create(u.ID, ua, ip, 15*time.Minute)
		_ = issueSessionCookiesSID(w, cfg, u.ID, rec.SID, body.RememberMe)
		issueCSRFCookie(w)
		writeJSON(w, map[string]any{"ok": true})
	})

	// Record refresh events in sessions store (best-effort)
	r.Post("/api/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		if uid, ok := decodeRefreshUID(r, cfg); ok {
			// rotate refresh; revoke all if reuse detected
			old := r.Header.Get("X-Refresh-ID")
			newID, reuse, _ := mgr.RotateRefresh(uid, strings.TrimSpace(old))
			if reuse {
				_ = mgr.RevokeAll(uid)
				clearAuthCookies(w)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = sessStore.Upsert(sessions.Session{ID: generateUUID(), UserID: uid, Roles: []string{"refresh"}, ExpiresAt: time.Now().Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339)})
			if err := issueSessionCookies(w, cfg, uid, true); err == nil {
				w.Header().Set("X-Refresh-ID", newID)
				writeJSON(w, map[string]any{"ok": true})
				return
			}
		}
		w.WriteHeader(http.StatusUnauthorized)
	})

	// Logout: clear cookies and remove persisted sessions for this user (best-effort)
	r.Post("/api/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if uid, ok := decodeSessionUID(r, cfg); ok {
			_ = sessStore.DeleteByUserID(uid)
			_ = mgr.RevokeAll(uid)
		}
		clearAuthCookies(w)
		w.WriteHeader(http.StatusNoContent)
	})

	// Session info (single) for compatibility with nos-client
	r.Get("/api/v1/auth/session", func(w http.ResponseWriter, r *http.Request) {
		uid, ok := decodeSessionUID(r, cfg)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if u, err := users.FindByID(uid); err == nil {
			// Minimal shape expected by FE
			writeJSON(w, map[string]any{
				"user": map[string]any{
					"id":       u.ID,
					"username": u.Username,
					"roles":    u.Roles,
					"isAdmin":  hasRole(u.Roles, "admin"),
				},
				"expiresAt": time.Now().Add(15 * time.Minute).UTC().Format(time.RFC3339),
			})
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	})

	// Protected API group (auth required)
	r.Group(func(pr chi.Router) {
		pr.Use(func(next http.Handler) http.Handler { return requireAuth(next, codec, cfg) })
		// Session endpoints (self scope)
		pr.Get("/api/v1/auth/sessions", func(w http.ResponseWriter, r *http.Request) {
			uid, ok := decodeSessionUID(r, cfg)
			if !ok {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			list := mgr.ListByUser(uid)
			// mark current
			curSID := r.Header.Get("X-SID")
			out := make([]map[string]any, 0, len(list))
			for _, s := range list {
				out = append(out, map[string]any{
					"sid":           s.SID,
					"createdAt":     s.CreatedAt,
					"lastSeenAt":    s.LastSeenAt,
					"ipPrefix":      s.IPHash,
					"uaFingerprint": s.UAHash,
					"current":       s.SID == curSID,
				})
			}
			writeJSON(w, out)
		})
		pr.Post("/api/v1/auth/sessions/revoke", func(w http.ResponseWriter, r *http.Request) {
			uid, ok := decodeSessionUID(r, cfg)
			if !ok {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			var body struct{ Scope, SID string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Scope == "" {
				body.Scope = "current"
			}
			ip := r.RemoteAddr
			if h := r.Header.Get("X-Forwarded-For"); h != "" {
				ip = strings.Split(h, ",")[0]
			}
			switch body.Scope {
			case "current":
				cur := r.Header.Get("X-SID")
				if cur != "" {
					_ = mgr.RevokeSID(cur)
				}
				clearAuthCookies(w)
				Logger(cfg).Info().Str("event", "auth.session.revoke").Str("userId", uid).Str("scope", "current").Str("sid", cur).Str("ip", ip).Msg("")
			case "all":
				_ = mgr.RevokeAll(uid)
				clearAuthCookies(w)
				Logger(cfg).Info().Str("event", "auth.session.revoke").Str("userId", uid).Str("scope", "all").Str("ip", ip).Msg("")
			case "sid":
				if body.SID == "" {
					httpx.WriteError(w, http.StatusBadRequest, "sid required")
					return
				}
				// validate ownership
				owned := false
				for _, s := range mgr.ListByUser(uid) {
					if s.SID == body.SID {
						owned = true
						break
					}
				}
				if !owned {
					httpx.WriteError(w, http.StatusForbidden, "not your session")
					return
				}
				_ = mgr.RevokeSID(body.SID)
				Logger(cfg).Info().Str("event", "auth.session.revoke").Str("userId", uid).Str("scope", "sid").Str("sid", body.SID).Str("ip", ip).Msg("")
			default:
				httpx.WriteError(w, http.StatusBadRequest, "invalid scope")
				return
			}
			writeJSON(w, map[string]any{"ok": true})
		})
	})

	r.Get("/api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		if uid, ok := decodeSessionUID(r, cfg); ok {
			if u, err := users.FindByID(uid); err == nil {
				writeJSON(w, map[string]any{"user": map[string]any{"id": u.ID, "username": u.Username, "roles": u.Roles}})
				return
			}
		}
		if s, ok := codec.DecodeFromRequest(r); ok {
			writeJSON(w, map[string]any{"user": map[string]any{"id": s.UserID, "role": s.Role}})
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	})

	r.Post("/api/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		if uid, ok := decodeRefreshUID(r, cfg); ok {
			if err := issueSessionCookies(w, cfg, uid, true); err == nil {
				writeJSON(w, map[string]any{"ok": true})
				return
			}
		}
		w.WriteHeader(http.StatusUnauthorized)
	})

	// TOTP setup & confirm
	r.Post("/api/v1/auth/totp/setup", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Email, Password string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		u, err := store.GetByEmail(body.Email)
		// TODO: Fix password verification - UserManager should handle this
		if err != nil /*|| !auth.VerifyPassword(auth.DefaultParams, u.PasswordHash, body.Password)*/ {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// TODO: Check if TOTP is enabled via UserManager
		if false /*u.TOTPSecret != ""*/ {
			w.WriteHeader(http.StatusConflict)
			if err := json.NewEncoder(w).Encode(map[string]any{"error": "totp_already_enabled"}); err != nil {
				fmt.Printf("Failed to write response: %v\n", err)
			}
			return
		}
		secret, uri, err := auth.GenerateTOTPSecret("NithronOS", u.Email)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// TODO: Store TOTP secret via UserManager
		// u.TOTPSecret = secret
		_ = store.UpdateUser(u)
		writeJSON(w, map[string]any{"secret": secret, "otpauth": uri})
	})

	r.Post("/api/v1/auth/totp/confirm", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Email, Code string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, err := store.GetByEmail(body.Email)
		// TODO: Check TOTP secret via UserManager
		if err != nil /*|| u.TOTPSecret == ""*/ {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// TODO: Verify TOTP via UserManager
		if false /*!auth.VerifyTOTP(u.TOTPSecret, body.Code)*/ {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	// Protected routes
	r.Group(func(pr chi.Router) {
		pr.Use(func(next http.Handler) http.Handler { return withUser(next, codec) })
		// Require auth via new opaque cookies or legacy session cookie (skip in tests when NOS_TEST_SKIP_AUTH=1)
		if os.Getenv("NOS_TEST_SKIP_AUTH") != "1" {
			pr.Use(func(next http.Handler) http.Handler { return requireAuth(next, codec, cfg) })
		}
		if os.Getenv("NOS_TEST_SKIP_AUTH") != "1" {
			pr.Use(requireCSRF)
		}

		// AdminRequired middleware: resolve current user and assert role
		adminRequired := func(next http.Handler) http.Handler {
			// Skip admin check in tests
			if os.Getenv("NOS_TEST_SKIP_AUTH") == "1" {
				return next
			}
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				uid, ok := decodeSessionUID(r, cfg)
				if !ok {
					if s, ok2 := codec.DecodeFromRequest(r); ok2 {
						uid = s.UserID
						ok = true
					}
				}
				if !ok || uid == "" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				u, err := users.FindByID(uid)
				if err != nil {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				isAdmin := false
				for _, r := range u.Roles {
					if r == "admin" {
						isAdmin = true
						break
					}
				}
				if !isAdmin {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
			})
		}

		// TOTP enroll (logged-in): generate secret, encrypt with secret.key, store pending enc
		pr.Get("/api/v1/auth/totp/enroll", func(w http.ResponseWriter, r *http.Request) {
			uid, ok := decodeSessionUID(r, cfg)
			if !ok {
				if s, ok2 := codec.DecodeFromRequest(r); ok2 {
					uid = s.UserID
					ok = true
				}
			}
			if !ok {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			u, err := users.FindByID(uid)
			if err != nil {
				httpx.WriteError(w, http.StatusNotFound, "user not found")
				return
			}
			secret, uri, err := auth.GenerateTOTPSecret("NithronOS", u.Username)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "totp error")
				return
			}
			enc, err := encryptWithSecretKey(cfg.SecretPath, []byte(secret))
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "encrypt error")
				return
			}
			u.TOTPEnc = enc
			if err := users.UpsertUser(u); err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "persist error")
				return
			}
			writeJSON(w, map[string]any{"otpauth_url": uri, "qr_png_base64": ""})
		})

		// Allow POST for enroll to match nos-client
		pr.Post("/api/v1/auth/totp/enroll", func(w http.ResponseWriter, r *http.Request) {
			// Delegate to GET handler logic by invoking the same code path
			r2 := r.Clone(r.Context())
			r2.Method = http.MethodGet
			pr.ServeHTTP(w, r2)
		})

		// TOTP verify (logged-in): verify code, generate recovery codes and persist hashes
		pr.Post("/api/v1/auth/totp/verify", func(w http.ResponseWriter, r *http.Request) {
			uid, ok := decodeSessionUID(r, cfg)
			if !ok {
				if s, ok2 := codec.DecodeFromRequest(r); ok2 {
					uid = s.UserID
					ok = true
				}
			}
			if !ok {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			u, err := users.FindByID(uid)
			if err != nil {
				httpx.WriteError(w, http.StatusNotFound, "user not found")
				return
			}
			var body struct{ Code string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			if len(body.Code) != 6 {
				httpx.WriteError(w, http.StatusBadRequest, "invalid code")
				return
			}
			secretB, err := decryptWithSecretKey(cfg.SecretPath, u.TOTPEnc)
			if err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid state")
				return
			}
			if !auth.VerifyTOTP(string(secretB), body.Code) {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid code")
				return
			}
			plain, hashes := generateRecoveryCodes()
			u.RecoveryHashes = hashes
			if err := users.UpsertUser(u); err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "persist error")
				return
			}
			writeJSON(w, map[string]any{"ok": true, "recovery_codes": plain})
		})

		pr.Get("/api/v1/disks", func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if runtime.GOOS != "windows" && hasCommand("lsblk") {
				if list, err := disks.Collect(ctx); err == nil {
					// Enrich with SMART when possible
					for i := range list {
						if list[i].Path != "" {
							list[i].Smart = disks.SmartSummaryFor(ctx, list[i].Path)
						}
					}
					writeJSON(w, map[string]any{"disks": list})
					return
				}
			}
			// Mock fallback
			writeJSON(w, map[string]any{"disks": []map[string]any{
				{"name": "sda", "kname": "sda", "path": "/dev/sda", "size": 1000204886016, "rota": true, "type": "disk", "tran": "sata", "vendor": "Mock", "model": "Disk A", "serial": "MOCKA123"},
				{"name": "nvme0n1", "kname": "nvme0n1", "path": "/dev/nvme0n1", "size": 512110190592, "rota": false, "type": "disk", "tran": "nvme", "vendor": "Mock", "model": "NVMe 512G", "serial": "MOCKNVME"},
			}})
		})

		pr.Get("/api/v1/pools", func(w http.ResponseWriter, r *http.Request) {
			list, _ := pools.ListPools(r.Context())
			writeJSON(w, list)
		})

		// Pools: allowed roots for shares (mounted pool paths)
		pr.Get("/api/v1/pools/roots", func(w http.ResponseWriter, r *http.Request) {
			roots, err := poolroots.AllowedRoots()
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, map[string]any{"roots": roots})
		})

		pr.With(adminRequired).Post("/api/v1/pools/plan-create", handlePlanCreateV1)

		// Health: alerts and manual SMART scan
		pr.Get("/api/v1/alerts", handleAlertsGet(cfg))

		// Services health endpoints
		pr.Get("/api/v1/health/services", handleServicesHealth(cfg))
		pr.Get("/api/v1/health/services/{service}", handleServiceHealth(cfg))
		pr.Get("/api/v1/health/services/{service}/logs", handleServiceLogs(cfg))
		pr.With(adminRequired).Post("/api/v1/health/services/{service}/restart", handleServiceRestart(cfg))

		// Monitoring endpoints
		pr.Get("/api/v1/monitoring/logs", handleMonitoringLogs(cfg))
		pr.Get("/api/v1/monitoring/events", handleMonitoringEvents(cfg))
		pr.Get("/api/v1/monitoring/alerts", handleMonitoringAlerts(cfg))
		pr.Get("/api/v1/monitoring/services", handleMonitoringServices(cfg))
		pr.Get("/api/v1/monitoring/system", handleMonitoringSystem(cfg))

		// Scrub endpoints expected by frontend
		pr.Get("/api/v1/scrub/status", func(w http.ResponseWriter, r *http.Request) {
			// Delegate to pools scrub status
			handleScrubStatus(w, r)
		})
		pr.With(adminRequired).Post("/api/v1/scrub/start", func(w http.ResponseWriter, r *http.Request) {
			// Delegate to pools scrub start
			handleScrubStart(w, r)
		})
		pr.With(adminRequired).Post("/api/v1/scrub/cancel", func(w http.ResponseWriter, r *http.Request) {
			// TODO: Implement scrub cancel
			writeJSON(w, map[string]any{"ok": true, "message": "Scrub cancelled"})
		})

		// Balance endpoints
		pr.Get("/api/v1/balance/status", handleBalanceStatus(cfg))
		pr.With(adminRequired).Post("/api/v1/balance/start", handleBalanceStart(cfg))
		pr.With(adminRequired).Post("/api/v1/balance/cancel", handleBalanceCancel(cfg))

		// SMART endpoints
		pr.Get("/api/v1/smart/summary", handleSmartSummary(cfg))
		pr.Get("/api/v1/smart/devices", handleSmartDevices(cfg))
		pr.Get("/api/v1/smart/device/{device}", handleSmartDevice(cfg))
		pr.Get("/api/v1/smart/test/{device}", handleSmartTestDevice(cfg))
		pr.With(adminRequired).Post("/api/v1/smart/scan", handleSmartScan(cfg))
		pr.With(adminRequired).Post("/api/v1/smart/test/{device}", handleSmartTestDevice(cfg))

		// Jobs endpoints
		pr.Get("/api/v1/jobs/recent", handleJobsRecent(cfg))
		pr.Get("/api/v1/jobs/{id}", handleJobGet(cfg))

		// Devices endpoint expected by frontend
		pr.Get("/api/v1/devices", func(w http.ResponseWriter, r *http.Request) {
			// Delegate to existing devices handler
			handleListDevices(w, r)
		})
		pr.With(adminRequired).Post("/api/v1/health/scan", handleHealthScan(cfg))
		pr.With(adminRequired).Post("/api/v1/pools/apply-create", handleApplyCreate(cfg))
		pr.With(adminRequired).Get("/api/v1/pools/discover", handlePoolsDiscover)
		pr.With(adminRequired).Post("/api/v1/pools/import", handlePoolsImport(cfg))
		// Device operations (plan/apply)
		pr.With(adminRequired).Post("/api/v1/pools/{id}/plan-device", handlePlanDevice(cfg))
		pr.With(adminRequired).Post("/api/v1/pools/{id}/apply-device", handleApplyDevice(cfg))
		pr.With(adminRequired).Post("/api/v1/pools/{id}/plan-destroy", handlePlanDestroy(cfg))
		pr.With(adminRequired).Post("/api/v1/pools/{id}/apply-destroy", handleApplyDestroy(cfg))
		pr.With(adminRequired).Post("/api/v1/pools/scrub/start", handleScrubStart)
		pr.With(adminRequired).Get("/api/v1/pools/scrub/status", handleScrubStatus)
		pr.Get("/api/v1/pools/{id}", handlePoolDetail)
		// Mount options (canonical + compatibility with FE path)
		pr.Get("/api/v1/pools/{id}/options", handlePoolOptionsGet(cfg))
		pr.With(adminRequired).Post("/api/v1/pools/{id}/options", handlePoolOptionsPost(cfg))
		// FE expects mount-options nomenclature
		pr.Get("/api/v1/pools/{id}/mount-options", handlePoolOptionsGet(cfg))
		pr.With(adminRequired).Post("/api/v1/pools/{id}/mount-options", handlePoolOptionsPost(cfg))

		pr.Get("/api/v1/schedules", handleSchedulesGet(cfg))
		pr.With(adminRequired).Post("/api/v1/schedules", handleSchedulesPost(cfg))
		pr.Get("/api/v1/pools/tx/{id}/status", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")
			var tx pools.Tx
			if ok, _ := fsatomic.LoadJSON(txPath(id), &tx); !ok {
				httpx.WriteError(w, http.StatusNotFound, "not found")
				return
			}
			writeJSON(w, tx)
		})
		pr.Get("/api/v1/pools/tx/{id}/log", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")
			cursorStr := r.URL.Query().Get("cursor")
			maxStr := r.URL.Query().Get("max")
			cursor, max := 0, 1000
			if i, err := strconv.Atoi(cursorStr); err == nil && i >= 0 {
				cursor = i
			}
			if i, err := strconv.Atoi(maxStr); err == nil && i > 0 && i <= 5000 {
				max = i
			}
			lines, next := readLogTail(id, cursor, max)
			writeJSON(w, map[string]any{"lines": lines, "nextCursor": next})
		})
		pr.Get("/api/v1/pools/tx/{id}/stream", handleTxStream)

		pr.With(adminRequired).Post("/api/v1/pools/create", func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Confirm") != "yes" {
				httpx.WriteError(w, http.StatusPreconditionRequired, "confirm header required")
				return
			}
			var req pools.PlanRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if err := pools.EnsureDevicesFree(r.Context(), req.Devices); err != nil {
				httpx.WriteError(w, http.StatusBadRequest, err.Error())
				return
			}
			client := agentclient.New("/run/nos-agent.sock")
			var resp map[string]any
			err := client.PostJSON(r.Context(), "/v1/btrfs/create", map[string]any{
				"devices": req.Devices,
				"raid":    req.Raid,
				"label":   req.Label,
				"dry_run": false,
			}, &resp)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, resp)
		})

		// Pools: candidates for import
		pr.With(adminRequired).Get("/api/v1/pools/candidates", func(w http.ResponseWriter, r *http.Request) {
			list, err := pools.ListPools(r.Context())
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, list)
		})

		// Pools: import handled by handlePoolsImport(cfg)

		// Shares endpoints are handled by SharesHandler below
		// SMB users proxy
		pr.Get("/api/v1/smb/users", func(w http.ResponseWriter, r *http.Request) {
			client := agentclient.New("/run/nos-agent.sock")
			var out struct {
				Users []string `json:"users"`
			}
			if err := client.GetJSON(r.Context(), "/v1/smb/users", &out); err != nil {
				// Graceful fallback
				writeJSON(w, []string{})
				return
			}
			writeJSON(w, out.Users)
		})
		// Shares endpoints (replaced by v1 API)
		// pr.With(adminRequired).Get("/api/shares", sharesHandler.ListShares)
		// pr.With(adminRequired).Post("/api/shares", sharesHandler.CreateShare)
		// pr.With(adminRequired).Get("/api/shares/{name}", sharesHandler.GetShare)
		// pr.With(adminRequired).Patch("/api/shares/{name}", sharesHandler.UpdateShare)
		// pr.With(adminRequired).Delete("/api/shares/{name}", sharesHandler.DeleteShare)
		// pr.With(adminRequired).Post("/api/shares/{name}/test", sharesHandler.TestShare)

		pr.With(adminRequired).Post("/api/v1/smb/users", func(w http.ResponseWriter, r *http.Request) {
			var body struct{ Username, Password string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			client := agentclient.New("/run/nos-agent.sock")
			var resp map[string]any
			if err := client.PostJSON(r.Context(), "/v1/smb/user-create", map[string]any{"username": body.Username, "password": body.Password}, &resp); err != nil {
				// If agent returned HTTPError 400, propagate 400
				if he, ok := err.(*agentclient.HTTPError); ok && he.Status == http.StatusBadRequest {
					httpx.WriteError(w, http.StatusBadRequest, he.Body)
					return
				}
				httpx.WriteError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, map[string]any{"ok": true})
		})
		/* Duplicate delete handler - removed
		pr.With(adminRequired).Delete("/api/shares/{id}", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")
			// st := shares.NewStore(cfg.SharesPath)
			st := struct{
				GetByID func(string) (interface{}, bool)
				Delete func(string) error
				RemoveByID func(string)
			}{
				GetByID: func(id string) (interface{}, bool) { return nil, false },
				Delete: func(id string) error { return nil },
				RemoveByID: func(id string) {},
			}
			sh, ok := st.GetByID(id)
			if ok {
				// Best-effort: in dev/test on Windows or when the agent socket isn't present, skip agent calls
				var client *agentclient.Client
				if runtime.GOOS != "windows" {
					if _, err := os.Stat("/run/nos-agent.sock"); err == nil {
						client = agentclient.New("/run/nos-agent.sock")
					}
				}
				if sh.Type == "smb" {
					path := filepath.Join(cfg.EtcDir, "samba", "smb.conf.d", "nos-"+sh.Name+".conf")
					if client != nil {
						_ = client.PostJSON(r.Context(), "/v1/fs/write", map[string]any{"path": path, "content": "", "mode": "0644", "owner": "root", "group": "root"}, nil)
					}
				}
				if sh.Type == "nfs" {
					path := filepath.Join(cfg.EtcDir, "exports.d", "nos-"+sh.Name+".exports")
					if client != nil {
						_ = client.PostJSON(r.Context(), "/v1/fs/write", map[string]any{"path": path, "content": ""}, nil)
					}
				}
				_ = st.Delete(id)
				if client != nil {
					_ = client.PostJSON(r.Context(), "/v1/service/reload", map[string]any{"name": "smb"}, nil)
					_ = client.PostJSON(r.Context(), "/v1/service/reload", map[string]any{"name": "nfs"}, nil)
				}
			}
			w.WriteHeader(http.StatusNoContent)
		}) */

		// App management routes
		if appsManager != nil {
			// Start app manager
			go func() {
				if err := appsManager.Start(context.Background()); err != nil {
					fmt.Printf("Failed to start apps manager: %v\n", err)
				}
			}()

			// Catalog and installed apps
			pr.Get("/api/v1/apps/catalog", handleGetCatalog(appsManager))
			pr.Get("/api/v1/apps/installed", handleGetInstalledApps(appsManager))

			// Individual app operations
			pr.Get("/api/v1/apps/{id}", handleGetApp(appsManager))
			pr.Get("/api/v1/apps/{id}/logs", handleGetAppLogs(appsManager))
			pr.Get("/api/v1/apps/{id}/events", handleGetAppEvents(appsManager))

			// App lifecycle operations (admin only)
			pr.With(adminRequired).Post("/api/v1/apps/install", handleInstallApp(appsManager))
			pr.With(adminRequired).Post("/api/v1/apps/{id}/upgrade", handleUpgradeApp(appsManager))
			pr.With(adminRequired).Post("/api/v1/apps/{id}/start", handleStartApp(appsManager))
			pr.With(adminRequired).Post("/api/v1/apps/{id}/stop", handleStopApp(appsManager))
			pr.With(adminRequired).Post("/api/v1/apps/{id}/restart", handleRestartApp(appsManager))
			pr.With(adminRequired).Post("/api/v1/apps/{id}/rollback", handleRollbackApp(appsManager))
			pr.With(adminRequired).Delete("/api/v1/apps/{id}", handleDeleteApp(appsManager))
			pr.With(adminRequired).Post("/api/v1/apps/{id}/health", handleForceHealthCheck(appsManager))

			// Admin operations
			pr.With(adminRequired).Post("/api/v1/apps/catalog/sync", handleSyncCatalogs(appsManager))
		} else {
			// Fallback: provide minimal implementations so FE endpoints exist
			pr.Get("/api/v1/apps/catalog", func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, apps.Catalog(cfg.AppsInstallDir))
			})
			pr.Get("/api/v1/apps/installed", func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, map[string]any{"items": []any{}})
			})
			pr.Get("/api/v1/apps/{id}", func(w http.ResponseWriter, r *http.Request) {
				httpx.WriteError(w, http.StatusNotFound, "App not found")
			})
		}

		// Health endpoints
		healthHandler := NewHealthHandler(agentclient.New(cfg.AgentSocket()))
		pr.Mount("/api/v1/health", healthHandler.Routes())

		// Storage endpoints
		storageHandler := NewStorageHandler(agentclient.New(cfg.AgentSocket()))
		pr.Mount("/api/v1/storage", storageHandler.Routes())

		// Btrfs endpoints
		btrfsHandler := NewBtrfsHandler(agentclient.New(cfg.AgentSocket()))
		pr.Mount("/api/v1/btrfs", btrfsHandler.Routes())

		// Schedule endpoints
		schedulesHandler := NewSchedulesHandler()
		pr.Mount("/api/v1/schedules", schedulesHandler.Routes())

		// Share endpoints (v1 API) - use real implementation
		if sharesHandler != nil {
			pr.Mount("/api/v1/shares", sharesHandler.Routes())
		} else {
			// Fallback to mock handler if real one failed to initialize
			sharesHandlerV1 := NewSharesHandlerV1()
			pr.Mount("/api/v1/shares", sharesHandlerV1.Routes())
		}

		// Jobs endpoints are already defined above

		// Backup endpoints
		if backupHandler != nil {
			pr.Mount("/api/v1/backup", backupHandler.Routes())
		}

		// Sync endpoints (for NithronSync clients)
		syncSharesStorePath := filepath.Join(filepath.Dir(cfg.UsersPath), "shares.json")
		syncSharesStore := shares.NewStore(syncSharesStorePath)
		syncHandler, syncErr := NewSyncHandler(cfg, syncSharesStore, *Logger(cfg))
		if syncErr != nil {
			Logger(cfg).Error().Err(syncErr).Msg("Failed to create sync handler")
		} else {
			// Mount sync API routes
			pr.Mount("/api/v1/sync", syncHandler.Routes())
			
			// Mount WebDAV endpoint for file access
			webdavHandler := NewWebDAVHandler(syncSharesStore, syncHandler.DeviceManager(), *Logger(cfg))
			r.Mount("/dav", webdavHandler)
			
			Logger(cfg).Info().Msg("NithronSync API initialized")
		}

		// Phase 4: Real-time collaboration
		realtimeHandler := NewRealtimeHandler(*Logger(cfg))
		pr.Mount("/api/v1/sync/realtime", realtimeHandler.Routes())
		Logger(cfg).Info().Msg("Real-time collaboration API initialized")

		// Phase 4: End-to-end encryption
		encryptionHandler, encErr := NewEncryptionHandler(cfg, *Logger(cfg))
		if encErr != nil {
			Logger(cfg).Error().Err(encErr).Msg("Failed to create encryption handler")
		} else {
			pr.Mount("/api/v1/sync/encryption", encryptionHandler.Routes())
			Logger(cfg).Info().Msg("Encryption API initialized")
		}

		// Phase 4: Offline-first sync
		offlineHandler, offErr := NewOfflineHandler(cfg, *Logger(cfg))
		if offErr != nil {
			Logger(cfg).Error().Err(offErr).Msg("Failed to create offline handler")
		} else {
			pr.Mount("/api/v1/sync/offline", offlineHandler.Routes())
			Logger(cfg).Info().Msg("Offline sync API initialized")
		}

		// Phase 4: Smart sync (on-demand files)
		smartsyncHandler, smartErr := NewSmartSyncHandler(cfg, *Logger(cfg))
		if smartErr != nil {
			Logger(cfg).Error().Err(smartErr).Msg("Failed to create smart sync handler")
		} else {
			pr.Mount("/api/v1/sync/smartsync", smartsyncHandler.Routes())
			Logger(cfg).Info().Msg("Smart sync API initialized")
		}

		// Notification endpoints
		if notificationManager != nil {
			pr.Mount("/api/v1/notifications", NewNotificationHandler(notificationManager).Routes())
		}

		// Network endpoints (M4)
		netLogger := Logger(cfg)
		netHandler, err := NewNetHandler(*netLogger)
		if err != nil {
			netLogger.Error().Err(err).Msg("Failed to create network handler")
			// Continue without networking features
		} else {
			pr.Mount("/api/v1/net", netHandler.Routes())
			pr.Mount("/api/v1/auth", netHandler.AuthRoutes())
		}

		// Updates endpoints (M5)
		updatesHandler := NewUpdatesHandler(cfg)
		pr.Mount("/api/v1/updates", updatesHandler.Routes())

		// Users management endpoints
		usersHandler := NewUsersHandler(users, cfg)
		pr.With(adminRequired).Mount("/api/v1/users", usersHandler.Routes())

		// Network configuration endpoints
		networkConfigHandler := NewNetworkConfigHandler(cfg)
		pr.With(adminRequired).Mount("/api/v1/network/config", networkConfigHandler.Routes())

		// Appearance settings endpoints
		appearanceHandler := NewAppearanceHandler(cfg)
		pr.Mount("/api/v1/settings/appearance", appearanceHandler.Routes())

		// About/System info endpoints
		aboutHandler := NewAboutHandler(cfg)
		pr.Mount("/api/v1/about", aboutHandler.Routes())

		// Apps catalog
		pr.Get("/api/v1/apps", func(w http.ResponseWriter, r *http.Request) {
			if appsManager != nil {
				catalog, _ := appsManager.GetCatalog()
				writeJSON(w, catalog)
			} else {
				writeJSON(w, apps.Catalog(cfg.AppsInstallDir))
			}
		})

		pr.Get("/api/v1/apps/{id}/status", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")
			if appsManager != nil {
				app, err := appsManager.GetApp(id)
				if err != nil {
					httpx.WriteError(w, http.StatusNotFound, "not found")
					return
				}
				writeJSON(w, app)
			} else {
				for _, a := range apps.Catalog(cfg.AppsInstallDir) {
					if a.ID == id {
						writeJSON(w, a)
						return
					}
				}
				httpx.WriteError(w, http.StatusNotFound, "not found")
			}
		})

		pr.With(adminRequired).Post("/api/v1/apps/install", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				ID     string
				Config map[string]any
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.ID == "" {
				httpx.WriteError(w, http.StatusBadRequest, "id required")
				return
			}
			dir := filepath.Join(cfg.AppsInstallDir, body.ID)
			_ = os.MkdirAll(dir, 0o755)
			compose := apps.ComposeTemplate(body.ID)
			if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(compose), 0o644); err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, err.Error())
				return
			}
			unit := apps.UnitTemplate(body.ID, dir)
			client := agentclient.New("/run/nos-agent.sock")
			_ = client.PostJSON(r.Context(), "/v1/systemd/install-app", map[string]any{"id": body.ID, "unit_text": unit}, nil)
			_ = client.PostJSON(r.Context(), "/v1/app/compose-up", map[string]any{"id": body.ID, "dir": dir}, nil)
			writeJSON(w, map[string]any{"ok": true})
		})

		pr.With(adminRequired).Post("/api/v1/apps/uninstall", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				ID    string
				Force bool
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.ID == "" {
				httpx.WriteError(w, http.StatusBadRequest, "id required")
				return
			}
			dir := filepath.Join(cfg.AppsInstallDir, body.ID)
			client := agentclient.New("/run/nos-agent.sock")
			_ = client.PostJSON(r.Context(), "/v1/app/compose-down", map[string]any{"id": body.ID, "dir": dir}, nil)
			_ = client.PostJSON(r.Context(), "/v1/systemd/disable-app", map[string]any{"id": body.ID}, nil)
			_ = os.Remove(filepath.Join(dir, "docker-compose.yml"))
			_ = os.Remove(dir)
			writeJSON(w, map[string]any{"ok": true})
		})

		pr.Get("/api/v1/remote/status", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]any{"mode": "lan-only", "https": true})
		})

		// Support bundle
		pr.Get("/api/v1/support/bundle", handleSupportBundle(cfg))

		// Firewall legacy routes removed; use /api/v1/net/firewall/*

		// Snapshots
		pr.Get("/api/v1/pools/{id}/snapshots", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")
			list, _ := pools.ListSnapshots(r.Context(), id)
			writeJSON(w, list)
		})

		// Updates: check (redundant with /api/v1/updates/* handler, but retain convenience)
		pr.Get("/api/v1/updates/check", func(w http.ResponseWriter, r *http.Request) {
			client := agentclient.New("/run/nos-agent.sock")
			var planResp map[string]any
			_ = client.PostJSON(r.Context(), "/v1/updates/plan", map[string]any{}, &planResp)
			// attach snapshot targets (best-effort)
			roots, _ := poolroots.AllowedRoots()
			writeJSON(w, map[string]any{"plan": planResp, "snapshot_roots": roots})
		})

		// Updates: apply
		pr.With(adminRequired).Post("/api/v1/updates/apply", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Packages []string `json:"packages"`
				Snapshot bool     `json:"snapshot"`
				Confirm  string   `json:"confirm"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if strings.ToLower(body.Confirm) != "yes" {
				httpx.WriteError(w, http.StatusPreconditionRequired, "confirm\u003dyes required")
				return
			}
			client := agentclient.New("/run/nos-agent.sock")
			// create tx and persist initial state
			txID := generateUUID()
			tx := snapdb.UpdateTx{TxID: txID, StartedAt: time.Now().UTC(), Packages: body.Packages, Reason: "pre-update"}
			_ = snapdb.Append(tx)
			// load snapshot targets via pools roots (simplified: allowed roots)
			roots, _ := poolroots.AllowedRoots()
			if body.Snapshot {
				for _, p := range roots {
					var sresp struct {
						OK                 bool `json:"ok"`
						ID, Type, Location string
					}
					if err := client.PostJSON(r.Context(), "/v1/snapshot/create", map[string]any{"path": p, "mode": "auto", "reason": "pre-update"}, &sresp); err != nil {
						mark := false
						now := time.Now().UTC()
						tx.FinishedAt = &now
						tx.Success = &mark
						tx.Notes = "snapshot failed: " + errString(err)
						_ = snapdb.Append(tx)
						httpx.WriteError(w, http.StatusInternalServerError, "snapshot failed")
						return
					}
					// append target on success
					tx.Targets = append(tx.Targets, snapdb.SnapshotTarget{ID: sresp.ID, Path: p, Type: sresp.Type, Location: sresp.Location, CreatedAt: time.Now().UTC()})
				}
			}
			// perform updates apply on agent
			var applyResp map[string]any
			if err := client.PostJSON(r.Context(), "/v1/updates/apply", map[string]any{"packages": body.Packages}, &applyResp); err != nil {
				mark := false
				now := time.Now().UTC()
				tx.FinishedAt = &now
				tx.Success = &mark
				tx.Notes = "apply failed: " + errString(err)
				_ = snapdb.Append(tx)
				httpx.WriteError(w, http.StatusInternalServerError, "updates apply failed")
				return
			}
			// success
			mark := true
			now := time.Now().UTC()
			tx.FinishedAt = &now
			tx.Success = &mark
			_ = snapdb.Append(tx)
			writeJSON(w, map[string]any{"ok": true, "tx_id": txID, "snapshots_count": len(tx.Targets), "updates_count": len(applyResp)})
		})

		// Snapshots: prune
		pr.With(adminRequired).Post("/api/v1/snapshots/prune", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				KeepPerTarget int `json:"keep_per_target"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.KeepPerTarget <= 0 {
				body.KeepPerTarget = 5
			}
			client := agentclient.New("/run/nos-agent.sock")
			var resp map[string]any
			if err := client.PostJSON(r.Context(), "/v1/snapshot/prune", map[string]any{"keep_per_target": body.KeepPerTarget}, &resp); err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, resp)
		})

		// Updates: rollback
		pr.With(adminRequired).Post("/api/v1/updates/rollback", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				TxID    string `json:"tx_id"`
				Confirm string `json:"confirm"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if strings.ToLower(body.Confirm) != "yes" {
				httpx.WriteError(w, http.StatusPreconditionRequired, "confirm\u003dyes required")
				return
			}
			orig, err := snapdb.FindByTx(body.TxID)
			if err != nil {
				httpx.WriteError(w, http.StatusNotFound, "tx not found")
				return
			}
			client := agentclient.New("/run/nos-agent.sock")
			// start rollback tx record
			roll := snapdb.UpdateTx{TxID: generateUUID(), StartedAt: time.Now().UTC(), Packages: orig.Packages, Reason: "rollback"}
			for _, t := range orig.Targets {
				var resp map[string]any
				if err := client.PostJSON(r.Context(), "/v1/snapshot/rollback", map[string]any{
					"path": t.Path, "snapshot_id": t.ID, "type": t.Type,
				}, &resp); err != nil {
					mark := false
					now := time.Now().UTC()
					roll.FinishedAt = &now
					roll.Success = &mark
					roll.Notes = "rollback failed for target " + t.Path + ": " + err.Error()
					_ = snapdb.Append(roll)
					httpx.WriteError(w, http.StatusInternalServerError, "rollback failed")
					return
				}
			}
			// persist rollback record and adjust ordering by writing a small "touch"
			now := time.Now().UTC()
			roll.FinishedAt = &now
			okMark := true
			roll.Success = &okMark
			roll.Notes = "rollback of " + orig.TxID
			_ = snapdb.Append(roll)
			writeJSON(w, map[string]any{"ok": true})
		})

		// Snapshots DB: recent
		pr.Get("/api/v1/snapshots/recent", func(w http.ResponseWriter, r *http.Request) {
			list, err := snapdb.ListRecent(20)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, err.Error())
				return
			}
			// project limited fields
			out := make([]map[string]any, 0, len(list))
			for _, tx := range list {
				miniTargets := make([]map[string]any, 0, len(tx.Targets))
				for _, t := range tx.Targets {
					miniTargets = append(miniTargets, map[string]any{
						"id": t.ID, "type": t.Type, "location": t.Location,
					})
				}
				ok := false
				if tx.Success != nil {
					ok = *tx.Success
				}
				out = append(out, map[string]any{
					"tx_id":    tx.TxID,
					"time":     tx.StartedAt,
					"packages": tx.Packages,
					"targets":  miniTargets,
					"success":  ok,
				})
			}
			writeJSON(w, out)
		})

		// Back-compat: verify-totp path expected by FE
		pr.Post("/api/v1/auth/verify-totp", func(w http.ResponseWriter, r *http.Request) {
			// Delegate to /api/v1/auth/totp/verify handler
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/api/v1/auth/totp/verify"
			pr.ServeHTTP(w, r2)
		})

		// Snapshots DB: by tx id
		pr.Get("/api/v1/snapshots/{tx_id}", func(w http.ResponseWriter, r *http.Request) {
			txID := chi.URLParam(r, "tx_id")
			tx, err := snapdb.FindByTx(txID)
			if err != nil {
				httpx.WriteError(w, http.StatusNotFound, "tx not found")
				return
			}
			writeJSON(w, tx)
		})

		pr.With(adminRequired).Post("/api/v1/pools/{id}/snapshots", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")
			var body struct {
				Subvol string
				Name   string
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			client := agentclient.New("/run/nos-agent.sock")
			var resp map[string]any
			err := client.PostJSON(r.Context(), "/v1/btrfs/snapshot", map[string]any{"path": body.Subvol, "name": body.Name}, &resp)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, err.Error())
				return
			}
			_ = id // unused for now
			writeJSON(w, resp)
		})
	})

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
				uid, ok := decodeSessionUID(r, cfg)
				if !ok {
					if s, ok2 := codec.DecodeFromRequest(r); ok2 {
						uid = s.UserID
						ok = true
					}
				}

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
		nr.Use(func(next http.Handler) http.Handler { return requireAuth(next, codec, cfg) })
		nr.Get("/interfaces", systemConfigHandler.ListInterfaces)
		nr.Get("/interfaces/{iface}", systemConfigHandler.GetInterface)
		nr.Post("/interfaces/{iface}", systemConfigHandler.ConfigureInterface)
	})

	// Telemetry endpoints to match FE contract: /api/v1/telemetry/consent
	r.Route("/api/v1/telemetry", func(tr chi.Router) {
		tr.Use(func(next http.Handler) http.Handler { return requireAuth(next, codec, cfg) })
		tr.Get("/consent", systemConfigHandler.GetTelemetryConsent)
		tr.Post("/consent", systemConfigHandler.SetTelemetryConsent)
	})

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
	return r
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

func generateUUID() string {
	const hex = "0123456789abcdef"
	var b [16]byte
	n := time.Now().UnixNano()
	for i := 0; i < 16; i++ {
		b[i] = byte(n >> (i * 8))
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	out := make([]byte, 36)
	idx := 0
	for i := 0; i < 16; i++ {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out[idx] = '-'
			idx++
		}
		out[idx] = hex[b[i]>>4]
		idx++
		out[idx] = hex[b[i]&0x0f]
		idx++
	}
	return string(out)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// legacy in-memory rate limiter removed; persisted ratelimit store governs throttling

// setup token helpers using secret.key
func setupEncodeToken(cfg config.Config, payload map[string]any) (string, error) {
	key, err := os.ReadFile(cfg.SecretPath)
	if err != nil {
		return "", err
	}
	sc := securecookie.New(key, nil)
	sc.MaxAge(600)
	return sc.Encode("nos_setup", payload)
}

func setupDecodeToken(cfg config.Config, tok string) (map[string]any, error) {
	key, err := os.ReadFile(cfg.SecretPath)
	if err != nil {
		return nil, err
	}
	sc := securecookie.New(key, nil)
	var out map[string]any
	if err := sc.Decode("nos_setup", tok, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// simple validators used during setup
func validUsername(s string) bool {
	if len(s) < 3 || len(s) > 32 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			continue
		}
		return false
	}
	return true
}

func validPassword(p string) bool {
	if len(p) < 12 {
		return false
	}
	hasLower, hasUpper, hasDigit, hasOther := false, false, false, false
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch {
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= '0' && c <= '9':
			hasDigit = true
		default:
			hasOther = true
		}
	}
	count := 0
	if hasLower {
		count++
	}
	if hasUpper {
		count++
	}
	if hasDigit {
		count++
	}
	if hasOther {
		count++
	}
	return count >= 3
}

func mustJSON(v any) []byte {
	b, _ := json.MarshalIndent(v, "", "  ")
	return b
}

// clientIP extracts client IP. If TrustProxy is true, use last untrusted hop from X-Forwarded-For; otherwise use RemoteAddr.
func clientIP(r *http.Request, cfg config.Config) string {
	ip := r.RemoteAddr
	if i := strings.LastIndex(ip, ":"); i >= 0 {
		ip = ip[:i]
	}
	if !(cfg.TrustProxy || RuntimeTrustProxy()) {
		return ip
	}
	h := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if h == "" {
		return ip
	}
	parts := strings.Split(h, ",")
	// take the last non-empty token
	for i := len(parts) - 1; i >= 0; i-- {
		p := strings.TrimSpace(parts[i])
		if p != "" {
			return p
		}
	}
	return ip
}

// genOTP6 generates a 6-digit OTP.
func genOTP6() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "000000"
	}
	n := (uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])) % 1000000
	return fmt.Sprintf("%06d", n)
}

func dirPermInfo(path string) string {
	fi, err := os.Stat(path)
	if err != nil {
		return ""
	}
	st := fi.Mode().Perm()
	// ls -ld-like summary (very simplified)
	ownerGid := ""
	if out, e := exec.Command("/usr/bin/stat", "-c", "%U:%G", path).CombinedOutput(); e == nil {
		ownerGid = strings.TrimSpace(string(out))
	}
	return fmt.Sprintf("ls -ld %s => mode %o owner %s; recommend: chown -R nos:nos %s && chmod 0750 %s", path, st, ownerGid, path, path)
}

// Health monitoring handlers

// SystemHealthResponse represents system health metrics
type SystemHealthResponse struct {
	CPU       float64     `json:"cpu"`
	Load1     float64     `json:"load1"`
	Load5     float64     `json:"load5"`
	Load15    float64     `json:"load15"`
	Memory    MemoryInfo  `json:"memory"`
	Swap      SwapInfo    `json:"swap"`
	Uptime    int64       `json:"uptimeSec"`
	TempCPU   *float64    `json:"tempCpu"`
	Timestamp int64       `json:"timestamp"`
	Network   NetworkInfo `json:"network"`
	DiskIO    DiskIOStats `json:"diskIO"`
}

// MemoryInfo represents memory usage
type MemoryInfo struct {
	Total     uint64  `json:"total"`
	Used      uint64  `json:"used"`
	Free      uint64  `json:"free"`
	Available uint64  `json:"available"`
	UsagePct  float64 `json:"usagePct"`
	Cached    uint64  `json:"cached"`
	Buffers   uint64  `json:"buffers"`
}

// SwapInfo represents swap usage
type SwapInfo struct {
	Total    uint64  `json:"total"`
	Used     uint64  `json:"used"`
	Free     uint64  `json:"free"`
	UsagePct float64 `json:"usagePct"`
}

// NetworkInfo represents network statistics
type NetworkInfo struct {
	BytesRecv   uint64 `json:"bytesRecv"`
	BytesSent   uint64 `json:"bytesSent"`
	PacketsRecv uint64 `json:"packetsRecv"`
	PacketsSent uint64 `json:"packetsSent"`
	RxSpeed     uint64 `json:"rxSpeed"`
	TxSpeed     uint64 `json:"txSpeed"`
}

// DiskIOStats represents disk I/O statistics
type DiskIOStats struct {
	ReadBytes  uint64 `json:"readBytes"`
	WriteBytes uint64 `json:"writeBytes"`
	ReadOps    uint64 `json:"readOps"`
	WriteOps   uint64 `json:"writeOps"`
	ReadSpeed  uint64 `json:"readSpeed"`
	WriteSpeed uint64 `json:"writeSpeed"`
}

// handleMetricsStream emits SystemHealthResponse via SSE at 1Hz
func handleMetricsStream(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		ctx := r.Context()
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		// Send first event immediately
		send := func() {
			// Reuse handleSystemHealth logic by constructing the payload here
			// Minimal duplication: compute a fresh snapshot
			payload := captureSystemHealth()
			b, _ := json.Marshal(payload)
			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(b)
			_, _ = w.Write([]byte("\n\n"))
			flusher.Flush()
		}

		send()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				send()
			}
		}
	}
}

// captureSystemHealth builds a SystemHealthResponse snapshot quickly
func captureSystemHealth() SystemHealthResponse {
	h := SystemHealthResponse{Timestamp: time.Now().Unix()}
	if cpuPercent, err := cpu.Percent(100*time.Millisecond, false); err == nil && len(cpuPercent) > 0 {
		h.CPU = cpuPercent[0]
	}
	if loadAvg, err := load.Avg(); err == nil {
		h.Load1 = loadAvg.Load1
		h.Load5 = loadAvg.Load5
		h.Load15 = loadAvg.Load15
	}
	if vmStat, err := mem.VirtualMemory(); err == nil {
		h.Memory = MemoryInfo{
			Total:     vmStat.Total,
			Used:      vmStat.Used,
			Free:      vmStat.Free,
			Available: vmStat.Available,
			UsagePct:  vmStat.UsedPercent,
			Cached:    vmStat.Cached,
			Buffers:   vmStat.Buffers,
		}
	}
	if swapStat, err := mem.SwapMemory(); err == nil {
		h.Swap = SwapInfo{Total: swapStat.Total, Used: swapStat.Used, Free: swapStat.Free, UsagePct: swapStat.UsedPercent}
	}
	if uptime, err := host.Uptime(); err == nil {
		h.Uptime = int64(uptime)
	}
	if netStats, err := net.IOCounters(false); err == nil && len(netStats) > 0 {
		current := netStats[0]
		now := time.Now()
		h.Network = NetworkInfo{BytesRecv: current.BytesRecv, BytesSent: current.BytesSent, PacketsRecv: current.PacketsRecv, PacketsSent: current.PacketsSent}
		if !lastNetStatsTime.IsZero() {
			duration := now.Sub(lastNetStatsTime).Seconds()
			if duration > 0 {
				h.Network.RxSpeed = uint64(float64(current.BytesRecv-lastNetStats.BytesRecv) / duration)
				h.Network.TxSpeed = uint64(float64(current.BytesSent-lastNetStats.BytesSent) / duration)
			}
		}
		lastNetStats = current
		lastNetStatsTime = now
	}
	if diskStats, err := disk.IOCounters(); err == nil {
		var totalRead, totalWrite, totalReadOps, totalWriteOps uint64
		for _, stat := range diskStats {
			totalRead += stat.ReadBytes
			totalWrite += stat.WriteBytes
			totalReadOps += stat.ReadCount
			totalWriteOps += stat.WriteCount
		}
		now := time.Now()
		h.DiskIO = DiskIOStats{ReadBytes: totalRead, WriteBytes: totalWrite, ReadOps: totalReadOps, WriteOps: totalWriteOps}
		if !lastDiskStatsTime.IsZero() {
			duration := now.Sub(lastDiskStatsTime).Seconds()
			if duration > 0 {
				h.DiskIO.ReadSpeed = uint64(float64(totalRead-lastDiskStats.ReadBytes) / duration)
				h.DiskIO.WriteSpeed = uint64(float64(totalWrite-lastDiskStats.WriteBytes) / duration)
			}
		}
		lastDiskStats = disk.IOCountersStat{ReadBytes: totalRead, WriteBytes: totalWrite}
		lastDiskStatsTime = now
	}
	if runtime.GOOS == "linux" {
		if temps, err := host.SensorsTemperatures(); err == nil {
			for _, temp := range temps {
				if temp.SensorKey == "coretemp_core_0" || temp.SensorKey == "cpu_thermal" {
					h.TempCPU = &temp.Temperature
					break
				}
			}
		}
	}
	return h
}

// DiskHealthResponse represents a disk's health information
type DiskHealthResponse struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Model      string      `json:"model"`
	Serial     string      `json:"serial"`
	SizeBytes  uint64      `json:"sizeBytes"`
	State      string      `json:"state"`
	TempC      *float64    `json:"tempC"`
	UsagePct   float64     `json:"usagePct"`
	Smart      SmartStatus `json:"smart"`
	Filesystem string      `json:"filesystem"`
	MountPoint string      `json:"mountPoint"`
}

// SmartStatus represents SMART health status
type SmartStatus struct {
	Passed     bool                   `json:"passed"`
	Attributes map[string]interface{} `json:"attrs,omitempty"`
	TestStatus string                 `json:"testStatus"`
}

// Network speed calculation cache
var (
	lastNetStats      net.IOCountersStat
	lastNetStatsTime  time.Time
	lastDiskStats     disk.IOCountersStat
	lastDiskStatsTime time.Time
)

// handleSystemHealth handles GET /api/health/system
func handleSystemHealth(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		health := SystemHealthResponse{
			Timestamp: time.Now().Unix(),
		}

		// CPU usage
		if cpuPercent, err := cpu.Percent(100*time.Millisecond, false); err == nil && len(cpuPercent) > 0 {
			health.CPU = cpuPercent[0]
		}

		// Load averages
		if loadAvg, err := load.Avg(); err == nil {
			health.Load1 = loadAvg.Load1
			health.Load5 = loadAvg.Load5
			health.Load15 = loadAvg.Load15
		}

		// Memory
		if vmStat, err := mem.VirtualMemory(); err == nil {
			health.Memory = MemoryInfo{
				Total:     vmStat.Total,
				Used:      vmStat.Used,
				Free:      vmStat.Free,
				Available: vmStat.Available,
				UsagePct:  vmStat.UsedPercent,
				Cached:    vmStat.Cached,
				Buffers:   vmStat.Buffers,
			}
		}

		// Swap
		if swapStat, err := mem.SwapMemory(); err == nil {
			health.Swap = SwapInfo{
				Total:    swapStat.Total,
				Used:     swapStat.Used,
				Free:     swapStat.Free,
				UsagePct: swapStat.UsedPercent,
			}
		}

		// Uptime
		if uptime, err := host.Uptime(); err == nil {
			health.Uptime = int64(uptime)
		}

		// Network stats with speed calculation
		if netStats, err := net.IOCounters(false); err == nil && len(netStats) > 0 {
			current := netStats[0]
			now := time.Now()

			health.Network = NetworkInfo{
				BytesRecv:   current.BytesRecv,
				BytesSent:   current.BytesSent,
				PacketsRecv: current.PacketsRecv,
				PacketsSent: current.PacketsSent,
			}

			// Calculate speed if we have previous stats
			if !lastNetStatsTime.IsZero() {
				duration := now.Sub(lastNetStatsTime).Seconds()
				if duration > 0 {
					health.Network.RxSpeed = uint64(float64(current.BytesRecv-lastNetStats.BytesRecv) / duration)
					health.Network.TxSpeed = uint64(float64(current.BytesSent-lastNetStats.BytesSent) / duration)
				}
			}

			lastNetStats = current
			lastNetStatsTime = now
		}

		// Disk I/O stats with speed calculation
		if diskStats, err := disk.IOCounters(); err == nil {
			var totalRead, totalWrite, totalReadOps, totalWriteOps uint64
			for _, stat := range diskStats {
				totalRead += stat.ReadBytes
				totalWrite += stat.WriteBytes
				totalReadOps += stat.ReadCount
				totalWriteOps += stat.WriteCount
			}

			now := time.Now()
			health.DiskIO = DiskIOStats{
				ReadBytes:  totalRead,
				WriteBytes: totalWrite,
				ReadOps:    totalReadOps,
				WriteOps:   totalWriteOps,
			}

			// Calculate speed if we have previous stats
			if !lastDiskStatsTime.IsZero() {
				duration := now.Sub(lastDiskStatsTime).Seconds()
				if duration > 0 {
					health.DiskIO.ReadSpeed = uint64(float64(totalRead-lastDiskStats.ReadBytes) / duration)
					health.DiskIO.WriteSpeed = uint64(float64(totalWrite-lastDiskStats.WriteBytes) / duration)
				}
			}

			lastDiskStats = disk.IOCountersStat{
				ReadBytes:  totalRead,
				WriteBytes: totalWrite,
			}
			lastDiskStatsTime = now
		}

		// CPU temperature (platform-specific, may not be available)
		if runtime.GOOS == "linux" {
			if temps, err := host.SensorsTemperatures(); err == nil {
				for _, temp := range temps {
					if temp.SensorKey == "coretemp_core_0" || temp.SensorKey == "cpu_thermal" {
						health.TempCPU = &temp.Temperature
						break
					}
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(health); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}
}

// handleDiskHealth handles GET /api/health/disks
func handleDiskHealth(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var disks []DiskHealthResponse

		// Get disk partitions
		partitions, err := disk.Partitions(false)
		if err != nil {
			// Return empty array on error
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode([]DiskHealthResponse{}); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
			return
		}

		for _, partition := range partitions {
			// Skip certain filesystem types
			if partition.Fstype == "tmpfs" || partition.Fstype == "devtmpfs" {
				continue
			}

			diskInfo := DiskHealthResponse{
				ID:         partition.Device,
				Name:       partition.Device,
				Filesystem: partition.Fstype,
				MountPoint: partition.Mountpoint,
				State:      "healthy", // Default state
				Smart: SmartStatus{
					Passed:     true,
					TestStatus: "passed",
				},
			}

			// Get usage statistics
			if usage, err := disk.Usage(partition.Mountpoint); err == nil {
				diskInfo.SizeBytes = usage.Total
				diskInfo.UsagePct = usage.UsedPercent
			}

			// Try to get disk info (model, serial)
			// This would require additional system calls or parsing /sys/block
			// For now, we'll use basic info
			diskInfo.Model = "Unknown"
			diskInfo.Serial = "Unknown"

			// Determine health state based on usage
			if diskInfo.UsagePct > 90 {
				diskInfo.State = "critical"
			} else if diskInfo.UsagePct > 80 {
				diskInfo.State = "warning"
			}

			disks = append(disks, diskInfo)
		}

		// Ensure we always return an array, even if empty
		if disks == nil {
			disks = []DiskHealthResponse{}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(disks); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}
}

func hasRole(roles []string, role string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}
