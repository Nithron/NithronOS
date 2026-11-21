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
	"syscall"
	"time"

	"nithronos/backend/nosd/internal/config"
	"nithronos/backend/nosd/internal/ratelimit"
	"nithronos/backend/nosd/internal/server"
	"nithronos/backend/nosd/internal/sessions"
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
	sess := sessions.New(cfg.SessionsPath)

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
			ch := make(chan os.Signal, 1)
			signal.Notify(ch)
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
		t1 := time.Now()
		_ = sess.Flush()
		sessMs := time.Since(t1).Milliseconds()
		sdCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = srv.Shutdown(sdCtx)
		cancel()
		server.Logger(cfg).Info().Msgf("shutdown: http done; ratelimit=%dms sessions=%dms total=%dms", rlMs, sessMs, time.Since(start).Milliseconds())
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

func ensureAgentToken(path string) {
	if path == "" {
		return
	}
	if _, err := os.Stat(path); err == nil {
		return
	}
	if dir := dirOf(path); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return
	}
	_ = os.WriteFile(path, b[:], 0o600)
}
