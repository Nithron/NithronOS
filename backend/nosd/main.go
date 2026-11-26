package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"nithronos/backend/nosd/internal/config"
	"nithronos/backend/nosd/internal/ratelimit"
	"nithronos/backend/nosd/internal/server"
	"nithronos/backend/nosd/internal/shares"
)

func main() {
	cfg := config.Load("/etc/nos/config.yaml")
	server.SetRuntimeCORSOrigin(cfg.CORSOrigin)
	server.SetRuntimeTrustProxy(cfg.TrustProxy)
	server.SetLogLevel(cfg.LogLevel)
	ensureSecret(cfg.SecretPath)
	ensureAgentToken("/etc/nos/agent-token")

	// Run shares migration
	if err := shares.RunMigration(); err != nil {
		server.Logger(cfg).Error().Err(err).Msg("Failed to run shares migration")
	}
	r := server.NewRouter(cfg)

	// graceful shutdown context
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// First-boot setup code: until the first administrator exists, keep a
	// valid code and show it on the console, replacing it when it expires.
	fb := server.FirstBootManager(cfg)
	go fb.Run(ctx, 30*time.Second, func(err error) {
		server.Logger(cfg).Error().Str("event", "setup.otp.store").Err(err).Msg("first-boot setup code unavailable")
	})

	// stores to flush on shutdown
	rl := ratelimit.New(cfg.RateLimitPath)

	srv := &http.Server{
		Addr:              cfg.Bind,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	server.Logger(cfg).Info().Msgf("nosd listening on http://%s", cfg.Bind)

	go func() {
		// SIGHUP hot reload (Unix only)
		if runtime.GOOS != "windows" {
			// Only SIGHUP reloads the config. signal.Notify without a signal
			// list relays every signal, including the SIGURG the Go runtime
			// sends itself for preemption, so the config was reloaded constantly.
			ch := make(chan os.Signal, 1)
			signal.Notify(ch, syscall.SIGHUP)
			for range ch {
				old := cfg
				cfg = config.Load("/etc/nos/config.yaml")
				// Apply safe fields
				server.SetRuntimeCORSOrigin(cfg.CORSOrigin)
				server.SetRuntimeTrustProxy(cfg.TrustProxy)
				server.SetLogLevel(cfg.LogLevel)
				logConfigDiff(old, cfg)
			}
		}
	}()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		start := time.Now()
		server.Logger(cfg).Info().Msg("shutdown: begin")
		t0 := time.Now()
		_ = rl.Flush()
		rlMs := time.Since(t0).Milliseconds()
		sdCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = srv.Shutdown(sdCtx)
		cancel()
		server.Logger(cfg).Info().Msgf("shutdown: http done; ratelimit=%dms total=%dms", rlMs, time.Since(start).Milliseconds())
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			server.Logger(cfg).Fatal().Err(err).Msg("server exited")
		}
	}
}

func logConfigDiff(old, cur config.Config) {
	// minimal diff of hot-reloadable fields
	if old.CORSOrigin != cur.CORSOrigin {
		server.Logger(cur).Info().Str("event", "config.reload").Str("field", "cors.origin").Str("old", old.CORSOrigin).Str("new", cur.CORSOrigin).Msg("")
	}
	if old.TrustProxy != cur.TrustProxy {
		server.Logger(cur).Info().Str("event", "config.reload").Str("field", "trustProxy").Bool("old", old.TrustProxy).Bool("new", cur.TrustProxy).Msg("")
	}
	if old.LogLevel != cur.LogLevel {
		server.Logger(cur).Info().Str("event", "config.reload").Str("field", "logLevel").Str("old", old.LogLevel.String()).Str("new", cur.LogLevel.String()).Msg("")
	}
}

func ensureSecret(path string) {
	if path == "" {
		return
	}
	if _, err := os.Stat(path); err == nil {
		return
	}
	// create parent dirs
	if dir := dirOf(path); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o600)
	fp := hex.EncodeToString(b)
	if len(fp) > 8 {
		fp = fp[:8]
	}
	fmt.Printf("generated secret key at %s (fp=%s)\n", path, fp)
}

func dirOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			if i == 0 {
				return "/"
			}
			return p[:i]
		}
	}
	return "."
}

// ensureAgentToken makes sure the agent bootstrap token exists as 64 hex
// characters. Earlier versions wrote 32 raw bytes, which the agent sends in a
// JSON string (invalid UTF-8 gets replaced), so registration rarely matched;
// such tokens are replaced.
func ensureAgentToken(path string) {
	if path == "" {
		return
	}
	if b, err := os.ReadFile(path); err == nil && isHexToken(strings.TrimSpace(string(b))) {
		return
	}
	if dir := dirOf(path); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return
	}
	_ = os.WriteFile(path, []byte(hex.EncodeToString(b[:])+"\n"), 0o600)
}

func isHexToken(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
