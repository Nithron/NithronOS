// Package session keeps the server-side record of every signed-in browser.
//
// A session is created at sign-in and lives as long as its refresh token. The
// short-lived nos_session cookie carries the session id, and every request is
// checked against this store, so signing out, revoking a session or deleting
// a user takes effect immediately rather than when the cookie expires.
//
// Each session has one current refresh id. A refresh must present it and
// gets a new one; presenting an older id means the refresh token was copied,
// and all of the user's sessions end.
package session

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"nos/backend/nosd/internal/fsatomic"
)

const (
	fileVersion = 2
	// RefreshGrace is how long the previous refresh id is still accepted
	// after a rotation, so two tabs refreshing at the same moment don't look
	// like a stolen token.
	RefreshGrace = 30 * time.Second
	// lastSeenPersistEvery limits how often activity alone rewrites the file.
	lastSeenPersistEvery = 5 * time.Minute
	maxUserAgent         = 200
)

var (
	// ErrSessionInvalid: the session doesn't exist, has expired, was revoked
	// or belongs to someone else.
	ErrSessionInvalid = errors.New("session invalid")
	// ErrRefreshReuse: an old refresh id was presented; all of the user's
	// sessions have been revoked.
	ErrRefreshReuse = errors.New("refresh token reused")
)

// Record is one signed-in browser or device.
type Record struct {
	SID       string    `json:"sid"`
	UID       string    `json:"uid"`
	IPPrefix  string    `json:"ip_prefix"`  // /24 (IPv4) or /64 (IPv6) at sign-in
	UserAgent string    `json:"user_agent"` // shown in the session list
	CreatedAt time.Time `json:"created_at"`
	LastSeen  time.Time `json:"last_seen"`
	ExpiresAt time.Time `json:"expires_at"`
	// Persistent: "remember me" was chosen, so the refresh cookie outlives
	// the browser session.
	Persistent    bool      `json:"persistent"`
	RefreshID     string    `json:"refresh_id"`
	PrevRefreshID string    `json:"prev_refresh_id,omitempty"`
	RotatedAt     time.Time `json:"rotated_at,omitzero"`
}

type diskFile struct {
	Version  int      `json:"version"`
	Sessions []Record `json:"sessions"`
}

// Manager is safe for concurrent use.
type Manager struct {
	path        string
	mu          sync.Mutex
	sessions    map[string]Record
	lastPersist time.Time
}

// New loads the sessions stored at path. Files from earlier versions (which
// had no refresh ids per session) are ignored, so users sign in once more.
func New(path string) *Manager {
	m := &Manager{path: path, sessions: map[string]Record{}}
	var df diskFile
	if ok, err := fsatomic.LoadJSON(path, &df); err == nil && ok && df.Version == fileVersion {
		for _, r := range df.Sessions {
			m.sessions[r.SID] = r
		}
	}
	return m
}

// Create starts a session for uid that lasts ttl.
func (m *Manager) Create(uid, userAgent, remoteAddr string, ttl time.Duration, persistent bool) (Record, error) {
	now := time.Now().UTC()
	if len(userAgent) > maxUserAgent {
		userAgent = userAgent[:maxUserAgent]
	}
	rec := Record{
		SID: newID(), UID: uid, IPPrefix: maskIP(remoteAddr), UserAgent: userAgent,
		CreatedAt: now, LastSeen: now, ExpiresAt: now.Add(ttl),
		Persistent: persistent, RefreshID: newID(),
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[rec.SID] = rec
	return rec, m.persistLocked()
}

// Active reports whether sid is a live session of uid, and records activity.
func (m *Manager) Active(sid, uid string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.liveLocked(sid, uid)
	if !ok {
		return false
	}
	rec.LastSeen = time.Now().UTC()
	m.sessions[sid] = rec
	if time.Since(m.lastPersist) > lastSeenPersistEvery {
		_ = m.persistLocked() // activity only; losing it is harmless
	}
	return true
}

// Get returns a live session.
func (m *Manager) Get(sid string) (Record, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.sessions[sid]
	if !ok || !time.Now().Before(rec.ExpiresAt) {
		return Record{}, false
	}
	return rec, true
}

// Refresh spends refreshID on session sid of uid: it returns the session with
// a new refresh id and its lifetime extended by ttl.
func (m *Manager) Refresh(sid, uid, refreshID string, ttl time.Duration) (Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.liveLocked(sid, uid)
	if !ok || refreshID == "" {
		return Record{}, ErrSessionInvalid
	}
	now := time.Now().UTC()
	switch {
	case refreshID == rec.RefreshID:
		rec.PrevRefreshID, rec.RefreshID, rec.RotatedAt = rec.RefreshID, newID(), now
	case refreshID == rec.PrevRefreshID && now.Sub(rec.RotatedAt) <= RefreshGrace:
		// A concurrent refresh already rotated it: hand out the current id.
	default:
		m.revokeAllLocked(uid)
		_ = m.persistLocked()
		return Record{}, ErrRefreshReuse
	}
	rec.LastSeen, rec.ExpiresAt = now, now.Add(ttl)
	m.sessions[sid] = rec
	return rec, m.persistLocked()
}

// RevokeSID ends one session.
func (m *Manager) RevokeSID(sid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sid)
	return m.persistLocked()
}

// RevokeAll ends every session of uid.
func (m *Manager) RevokeAll(uid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revokeAllLocked(uid)
	return m.persistLocked()
}

// RevokeOthers ends every session of uid except keepSID (after a password
// change, the browser that changed it stays signed in).
func (m *Manager) RevokeOthers(uid, keepSID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for sid, r := range m.sessions {
		if r.UID == uid && sid != keepSID {
			delete(m.sessions, sid)
		}
	}
	return m.persistLocked()
}

// ListByUser returns uid's live sessions, most recently used first.
func (m *Manager) ListByUser(uid string) []Record {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Record{}
	now := time.Now()
	for _, r := range m.sessions {
		if r.UID == uid && now.Before(r.ExpiresAt) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeen.After(out[j].LastSeen) })
	return out
}

func (m *Manager) liveLocked(sid, uid string) (Record, bool) {
	rec, ok := m.sessions[sid]
	if !ok || uid == "" || rec.UID != uid || !time.Now().Before(rec.ExpiresAt) {
		return Record{}, false
	}
	return rec, true
}

func (m *Manager) revokeAllLocked(uid string) {
	for sid, r := range m.sessions {
		if r.UID == uid {
			delete(m.sessions, sid)
		}
	}
}

// persistLocked writes the live sessions; expired ones are dropped here.
func (m *Manager) persistLocked() error {
	now := time.Now()
	list := make([]Record, 0, len(m.sessions))
	for sid, r := range m.sessions {
		if !now.Before(r.ExpiresAt) {
			delete(m.sessions, sid)
			continue
		}
		list = append(list, r)
	}
	m.lastPersist = now
	return fsatomic.SaveJSON(m.path, diskFile{Version: fileVersion, Sessions: list}, 0o600)
}

// newID returns 128 random bits as 26 base32 characters.
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("session: crypto/rand failed: " + err.Error())
	}
	return strings.TrimRight(base32.StdEncoding.EncodeToString(b[:]), "=")
}

// maskIP returns the /24 (IPv4) or /64 (IPv6) network of a remote address.
func maskIP(remoteAddr string) string {
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	ip := net.ParseIP(host)
	switch {
	case ip == nil:
		return ""
	case ip.To4() != nil:
		return ip.Mask(net.CIDRMask(24, 32)).String()
	default:
		return ip.Mask(net.CIDRMask(64, 128)).String()
	}
}
