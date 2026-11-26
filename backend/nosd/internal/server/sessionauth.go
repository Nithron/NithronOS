package server

import (
	"context"
	"net/http"

	"nos/backend/nosd/internal/auth/session"
	"nos/backend/nosd/internal/config"
)

type signedIn struct{ UID, SID string }

type ctxSessionKey struct{}

// withSession identifies the signed-in user. A request counts as signed in
// only when its nos_session cookie is valid AND names a session that is still
// active on the server, so signing out, revoking a session or deleting a user
// takes effect at once. (Before, any unexpired cookie was accepted and the
// server-side check explicitly did not enforce.)
//
// Handlers read the result with signedInUser / signedInUID. X-UID and X-SID
// carry the same identity to older handlers; client-sent values are dropped.
func withSession(mgr *session.Manager, cfg config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Del("X-UID")
			r.Header.Del("X-SID")
			if uid, sid, ok := decodeSessionParts(r, cfg); ok && mgr.Active(sid, uid) {
				r.Header.Set("X-UID", uid)
				r.Header.Set("X-SID", sid)
				ctx := context.WithValue(r.Context(), ctxSessionKey{}, signedIn{UID: uid, SID: sid})
				ctx = context.WithValue(ctx, ctxUserID, uid)
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// signedInUser returns the verified session of r (see withSession).
func signedInUser(r *http.Request) (signedIn, bool) {
	s, ok := r.Context().Value(ctxSessionKey{}).(signedIn)
	return s, ok && s.UID != ""
}

// signedInUID returns the verified user id of r (see withSession).
func signedInUID(r *http.Request) (string, bool) {
	s, ok := signedInUser(r)
	return s.UID, ok
}
