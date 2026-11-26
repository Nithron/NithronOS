package server

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"

	"nos/backend/nosd/internal/config"
)

func zerologMiddleware(logger *zerolog.Logger, cfg config.Config) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &statusWriter{ResponseWriter: w, status: 200}
			next.ServeHTTP(ww, r)
			dur := time.Since(start)
			reqID := middleware.GetReqID(r.Context())
			uid := r.Header.Get("X-UID")
			ip := clientIP(r, cfg)
			evt := logger.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", ww.status).
				Dur("duration", dur).
				Str("ip", ip)
			if reqID != "" {
				evt = evt.Str("request_id", reqID)
			}
			if uid != "" {
				evt = evt.Str("uid", uid)
			}
			if cl := r.Header.Get("Content-Length"); cl != "" {
				if n, err := strconv.Atoi(cl); err == nil {
					evt = evt.Int("content_length", n)
				}
			}
			evt.Msg("http")
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Flush, Hijack and Unwrap pass through to the real writer. Without them
// every server-sent event stream (metrics, update progress, app logs) failed
// with "streaming unsupported" and WebSocket upgrades were impossible.
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("the response writer can't be hijacked")
	}
	return h.Hijack()
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
