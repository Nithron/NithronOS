package server

import (
	"net"
	"net/http"
	"strings"

	"nithronos/backend/nosd/internal/config"
)

// isLocalRequest reports whether r was made directly from this machine: the
// TCP peer is a loopback address and the request carries no proxy headers.
// Requests that come through Caddy also arrive from loopback, but Caddy always
// adds X-Forwarded-For, so they are not local.
func isLocalRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return false
	}
	for _, h := range []string{"X-Forwarded-For", "X-Real-IP", "True-Client-IP", "Forwarded"} {
		if r.Header.Get(h) != "" {
			return false
		}
	}
	return true
}

// clientIP returns the address used for rate limits and logs. X-Forwarded-For
// is honoured only when proxy trust is configured (nosd listens on loopback
// behind Caddy by default); then the last valid entry, the one the trusted
// proxy appended, is used, so a client can't choose its own address.
func clientIP(r *http.Request, cfg config.Config) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if !cfg.TrustProxy && !RuntimeTrustProxy() {
		return host
	}
	parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(parts) - 1; i >= 0; i-- {
		if p := strings.TrimSpace(parts[i]); net.ParseIP(p) != nil {
			return p
		}
	}
	return host
}
