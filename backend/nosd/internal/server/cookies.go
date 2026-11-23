package server

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"time"

	"nithronos/backend/nosd/internal/config"

	"github.com/gorilla/securecookie"
)

const (
	cookieSession = "nos_session"
	cookieRefresh = "nos_refresh"
	cookieCSRF    = "nos_csrf"
)

// issueSessionCookies sets nos_session (15m) and optionally rotates/sets nos_refresh (7d)
func issueSessionCookies(w http.ResponseWriter, cfg config.Config, uid string, keepRefresh bool) error {
	now := time.Now().UTC()
	// session token
	sess := map[string]any{"uid": uid, "exp": now.Add(15 * time.Minute).Unix()}
	sVal, err := encodeOpaque(cfg, cookieSession, sess)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: cookieSession, Value: sVal, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, Expires: now.Add(15 * time.Minute)})
	// refresh
	if keepRefresh {
		return issueRefreshCookie(w, cfg, uid, generateUUID())
	}
	return nil
}

// issueRefreshCookie sets nos_refresh. rid identifies this refresh token: each
// refresh spends it and issues a new one, so a replayed (stolen) token is
// detected (see the /api/v1/auth/refresh handler).
func issueRefreshCookie(w http.ResponseWriter, cfg config.Config, uid, rid string) error {
	exp := time.Now().UTC().Add(7 * 24 * time.Hour)
	ref := map[string]any{"uid": uid, "rid": rid, "iat": time.Now().UnixMilli(), "exp": exp.Unix()}
	rVal, err := encodeOpaque(cfg, cookieRefresh, ref)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: cookieRefresh, Value: rVal, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, Expires: exp})
	return nil
}

func clearAuthCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: cookieSession, Value: "", Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: cookieRefresh, Value: "", Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: cookieCSRF, Value: "", Path: "/", Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
}

// decodeSessionUID validates nos_session and returns uid string
func decodeSessionUID(r *http.Request, cfg config.Config) (string, bool) {
	ck, err := r.Cookie(cookieSession)
	if err != nil {
		return "", false
	}
	var m map[string]any
	if err := decodeOpaque(cfg, cookieSession, ck.Value, &m); err != nil {
		return "", false
	}
	// exp check (support multiple numeric/string types)
	expUnix, ok := asInt64(m["exp"])
	if !ok || time.Now().UTC().Unix() > expUnix {
		return "", false
	}
	if uid, ok := m["uid"].(string); ok && uid != "" {
		return uid, true
	}
	return "", false
}

// decodeRefreshUID validates nos_refresh and returns uid string
func decodeRefreshUID(r *http.Request, cfg config.Config) (string, bool) {
	uid, _, _, ok := decodeRefresh(r, cfg)
	return uid, ok
}

// decodeRefresh validates nos_refresh and returns its uid, refresh id and
// issue time in Unix ms. Cookies issued before refresh rotation existed have
// no rid/iat; the refresh handler refuses them (iat 0 is never after the
// user's last revocation), so users sign in once more after upgrading.
func decodeRefresh(r *http.Request, cfg config.Config) (uid, rid string, iatMs int64, ok bool) {
	ck, err := r.Cookie(cookieRefresh)
	if err != nil {
		return "", "", 0, false
	}
	var m map[string]any
	if err := decodeOpaque(cfg, cookieRefresh, ck.Value, &m); err != nil {
		return "", "", 0, false
	}
	expUnix, ok := asInt64(m["exp"])
	if !ok || time.Now().UTC().Unix() > expUnix {
		return "", "", 0, false
	}
	uid, _ = m["uid"].(string)
	if uid == "" {
		return "", "", 0, false
	}
	rid, _ = m["rid"].(string)
	iatMs, _ = asInt64(m["iat"])
	return uid, rid, iatMs, true
}

func issueCSRFCookie(w http.ResponseWriter) {
	b := securecookie.GenerateRandomKey(32)
	http.SetCookie(w, &http.Cookie{Name: cookieCSRF, Value: encodeBase64(b), Path: "/", Secure: true, SameSite: http.SameSiteLaxMode, Expires: time.Now().Add(24 * time.Hour)})
}

// issueSessionCookiesSID sets nos_session with server-side sid binding
func issueSessionCookiesSID(w http.ResponseWriter, cfg config.Config, uid, sid string, keepRefresh bool) error {
	now := time.Now().UTC()
	sess := map[string]any{"uid": uid, "sid": sid, "exp": now.Add(15 * time.Minute).Unix()}
	sVal, err := encodeOpaque(cfg, cookieSession, sess)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: cookieSession, Value: sVal, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, Expires: now.Add(15 * time.Minute)})
	if keepRefresh {
		return issueRefreshCookie(w, cfg, uid, generateUUID())
	}
	return nil
}

// decodeSessionParts returns uid and sid (when present) from nos_session
func decodeSessionParts(r *http.Request, cfg config.Config) (string, string, bool) {
	ck, err := r.Cookie(cookieSession)
	if err != nil {
		return "", "", false
	}
	var m map[string]any
	if err := decodeOpaque(cfg, cookieSession, ck.Value, &m); err != nil {
		return "", "", false
	}
	expUnix, ok := asInt64(m["exp"])
	if !ok || time.Now().UTC().Unix() > expUnix {
		return "", "", false
	}
	uid, ok1 := m["uid"].(string)
	sid, _ := m["sid"].(string)
	if uid == "" {
		ok1 = false
	}
	return uid, sid, ok1
}

func encodeOpaque(cfg config.Config, name string, payload map[string]any) (string, error) {
	key, _ := json.Marshal([]byte{}) // dummy to satisfy interface
	_ = key
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

func encodeBase64(b []byte) string {
	// lightweight encoder without pulling extra deps
	v, _ := securecookie.EncodeMulti("b64", b, securecookie.CodecsFromPairs()...)
	return v
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
