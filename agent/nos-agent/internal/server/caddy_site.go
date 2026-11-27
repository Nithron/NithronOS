package server

// POST /v1/caddy/public-site: the HTTPS settings' public domain. nOS's
// Caddyfile imports Caddyfile.d/site-*.caddy; this writes site-public.caddy
// (the domain with a Let's Encrypt certificate, serving the same web
// interface as the LAN site) or removes it when the domain is empty, then
// validates and reloads Caddy. A configuration Caddy refuses is taken back.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	reSiteDomain = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)
	reSiteEmail  = regexp.MustCompile(`^[A-Za-z0-9._%+-]{1,64}@[A-Za-z0-9.-]{1,253}\.[A-Za-z]{2,63}$`)
)

// PublicSiteRequest sets (domain and email) or removes (empty domain) the
// public site.
type PublicSiteRequest struct {
	Domain string `json:"domain"`
	Email  string `json:"email"`
}

func publicSitePath() string { return filepath.Join(caddyAppsDir, "site-public.caddy") }

func publicSiteBlock(domain, email string) string {
	return fmt.Sprintf(`# Managed by nOS (nos-agent): public HTTPS
%s {
  tls %s
  import nos_site
}
`, domain, email)
}

func setPublicSite(h appHost, req PublicSiteRequest) (int, error) {
	domain := strings.ToLower(strings.TrimSpace(req.Domain))
	if domain == "" {
		if err := h.Remove(publicSitePath()); err != nil {
			return http.StatusInternalServerError, err
		}
		_, code, err := reloadCaddy(h)
		return code, err
	}
	if !reSiteDomain.MatchString(domain) || len(domain) > 253 {
		return http.StatusBadRequest, fmt.Errorf("invalid domain")
	}
	if !reSiteEmail.MatchString(req.Email) {
		return http.StatusBadRequest, fmt.Errorf("invalid email")
	}
	old, errOld := h.ReadFile(publicSitePath())
	if err := h.WriteFile(publicSitePath(), []byte(publicSiteBlock(domain, req.Email))); err != nil {
		return http.StatusInternalServerError, err
	}
	if _, code, err := reloadCaddy(h); err != nil {
		// Put back what was there, so Caddy keeps working.
		if errOld == nil {
			_ = h.WriteFile(publicSitePath(), old)
		} else {
			_ = h.Remove(publicSitePath())
		}
		_, _, _ = reloadCaddy(h)
		return code, err
	}
	return 0, nil
}

func handlePublicSite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req PublicSiteRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if code, err := setPublicSite(newAppHost(), req); err != nil {
		writeErr(w, code, err.Error())
		return
	}
	logAuthPriv("caddy.public-site " + req.Domain)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
