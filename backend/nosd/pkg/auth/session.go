package auth

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/gorilla/securecookie"
)

const SessionCookieName = "nos_sess"
const CSRFCookieName = "nos_csrf"

type SessionCodec struct {
	sc *securecookie.SecureCookie
}

func NewSessionCodec(hashKey, blockKey []byte) *SessionCodec {
	sc := securecookie.New(hashKey, blockKey)
	sc.MaxAge(86400 * 30) // 30 days
	return &SessionCodec{sc: sc}
}

func (c *SessionCodec) EncodeToCookie(w http.ResponseWriter, s Session) error {
	val, err := c.sc.Encode(SessionCookieName, s)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    val,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
	})
	return nil
}

func (c *SessionCodec) DecodeFromRequest(r *http.Request) (Session, bool) {
	ck, err := r.Cookie(SessionCookieName)
	if err != nil {
		return Session{}, false
	}
	var s Session
	if err := c.sc.Decode(SessionCookieName, ck.Value, &s); err != nil {
		return Session{}, false
	}
	return s, true
}

func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
		MaxAge:   -1,
	})
}

func IssueCSRF(w http.ResponseWriter) string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	token := base64.RawURLEncoding.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{
		Name:  CSRFCookieName,
		Value: token,
		Path:  "/",
		// Not HttpOnly so JS can mirror to header if needed
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
	})
	return token
}

// Session is the payload of the legacy signed session cookie.
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
}
