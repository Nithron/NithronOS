package server

import (
	"encoding/base64"
	"net/http"
	"os"
	"strconv"
	"time"

	"nos/backend/nosd/internal/auth/session"
	"nos/backend/nosd/internal/config"

	"github.com/gorilla/securecookie"
)

const (
	cookieSession = "nos_session"
	cookieRefresh = "nos_refresh"
	cookieCSRF    = "nos_csrf"
)

// Lifetimes come from the config (sessions.accessTTL and refreshTTL, which
// used to be ignored): the access cookie is short; the server-side session
// and the refresh cookie last RefreshTTL and are extended on every refresh.

// issueAccessCookie sets nos_session, which names the server-side session.
func issueAccessCookie(w http.ResponseWriter, cfg config.Config, uid, sid string) error {
	exp := time.Now().UTC().Add(cfg.AccessTTL())
	val, err := encodeOpaque(cfg, cookieSession, map[string]any{"uid": uid, "sid": sid, "exp": exp.Unix()})
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: cookieSession, Value: val, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, Expires: exp})
	return nil
}

// issueRefreshCookie sets nos_refresh for session sid with its current
// refresh id. Without "remember me" it is a browser-session cookie, so closing
// the browser signs the user out.
func issueRefreshCookie(w http.ResponseWriter, cfg config.Config, uid, sid, rid string, persistent bool) error {
	exp := time.Now().UTC().Add(cfg.RefreshTTL())
	val, err := encodeOpaque(cfg, cookieRefresh, map[string]any{"uid": uid, "sid": sid, "rid": rid, "exp": exp.Unix()})
	if err != nil {
		return err
	}
	ck := &http.Cookie{Name: cookieRefresh, Value: val, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode}
	if persistent {
		ck.Expires = exp
	}
	http.SetCookie(w, ck)
	return nil
}

// issueAuthCookies sets both cookies for a session.
func issueAuthCookies(w http.ResponseWriter, cfg config.Config, rec session.Record) error {
	if err := issueAccessCookie(w, cfg, rec.UID, rec.SID); err != nil {
		return err
	}
	return issueRefreshCookie(w, cfg, rec.UID, rec.SID, rec.RefreshID, rec.Persistent)
}

func clearAuthCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: cookieSession, Value: "", Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: cookieRefresh, Value: "", Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: cookieCSRF, Value: "", Path: "/", Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
}

// issueCSRFCookie sets the CSRF token the UI echoes in X-CSRF-Token. It
// lives as long as the session can be refreshed; a refresh keeps the current
// value (requests in flight still carry it) and extends it; sign-in always
// starts a new one, so a planted cookie can't fix it. (It used to expire
// after 24 hours while sessions refresh for 7 days, so every change failed
// with "Missing CSRF token" after a day.)
func issueCSRFCookie(w http.ResponseWriter, r *http.Request, ttl time.Duration, keep bool) {
	value := ""
	if ck, err := r.Cookie(cookieCSRF); keep && err == nil && len(ck.Value) >= 32 {
		value = ck.Value
	} else {
		value = encodeBase64(securecookie.GenerateRandomKey(32))
	}
	http.SetCookie(w, &http.Cookie{Name: cookieCSRF, Value: value, Path: "/", Secure: true, SameSite: http.SameSiteLaxMode, Expires: time.Now().Add(ttl)})
}

// decodeRefresh validates nos_refresh and returns its user, session and
// refresh ids. Cookies from before sessions were enforced have no sid and are
// refused, so users sign in once more after upgrading.
func decodeRefresh(r *http.Request, cfg config.Config) (uid, sid, rid string, ok bool) {
	m, ok := decodeCookie(r, cfg, cookieRefresh)
	if !ok {
		return "", "", "", false
	}
	uid, _ = m["uid"].(string)
	sid, _ = m["sid"].(string)
	rid, _ = m["rid"].(string)
	if uid == "" || sid == "" || rid == "" {
		return "", "", "", false
	}
	return uid, sid, rid, true
}

// decodeSessionParts validates the signature and expiry of nos_session and
// returns its user and session ids. It does not check that the session is
// still active; withSession does (use signedInUser in handlers).
func decodeSessionParts(r *http.Request, cfg config.Config) (uid, sid string, ok bool) {
	m, ok := decodeCookie(r, cfg, cookieSession)
	if !ok {
		return "", "", false
	}
	uid, _ = m["uid"].(string)
	sid, _ = m["sid"].(string)
	if uid == "" || sid == "" {
		return "", "", false
	}
	return uid, sid, true
}

// decodeCookie checks a signed cookie and its exp claim.
func decodeCookie(r *http.Request, cfg config.Config, name string) (map[string]any, bool) {
	ck, err := r.Cookie(name)
	if err != nil {
		return nil, false
	}
	var m map[string]any
	if err := decodeOpaque(cfg, name, ck.Value, &m); err != nil {
		return nil, false
	}
	if exp, ok := asInt64(m["exp"]); !ok || time.Now().UTC().Unix() > exp {
		return nil, false
	}
	return m, true
}

func encodeOpaque(cfg config.Config, name string, payload map[string]any) (string, error) {
	secret, err := os.ReadFile(cfg.SecretPath)
	if err != nil || len(secret) == 0 {
		secret = cfg.SessionHashKey
	}
	sc := securecookie.New(secret, nil)
	sc.MaxAge(0)
	return sc.Encode(name, payload)
}

func decodeOpaque(cfg config.Config, name string, val string, out *map[string]any) error {
	secret, err := os.ReadFile(cfg.SecretPath)
	if err != nil || len(secret) == 0 {
		secret = cfg.SessionHashKey
	}
	sc := securecookie.New(secret, nil)
	sc.MaxAge(0)
	return sc.Decode(name, val, out)
}

// encodeBase64 encodes a CSRF token. (It used securecookie.EncodeMulti
// without codecs, which always fails, so every CSRF token was "".)
func encodeBase64(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func asInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case int:
		return int64(x), true
	case float64:
		return int64(x), true
	case string:
		if n, err := strconv.ParseInt(x, 10, 64); err == nil {
			return n, true
		}
	}
	return 0, false
}
