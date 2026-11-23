package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"strings"
	"sync"
	"time"

	"nithronos/backend/nosd/internal/fsatomic"
)

type Record struct {
	SID        string `json:"sid"`
	UID        string `json:"uid"`
	UAHash     string `json:"ua_hash"`
	IPHash     string `json:"ip_hash"`
	Exp        string `json:"exp"`
	CreatedAt  string `json:"created_at"`
	LastSeenAt string `json:"last_seen_at"`
}

type diskFile struct {
	Version     int                 `json:"version"`
	Sessions    []Record            `json:"sessions"`
	UsedRefresh map[string][]string `json:"used_refresh"`
	// RevokedBefore is, per user, the time (Unix ms) of the last "revoke all";
	// refresh tokens issued before it are no longer accepted.
	RevokedBefore map[string]int64 `json:"revoked_before,omitempty"`
}

type Manager struct {
	path          string
	mu            sync.RWMutex
	sidToRec      map[string]Record
	userToSids    map[string]map[string]struct{}
	usedRefresh   map[string]map[string]struct{} // uid -> rtid set
	revokedBefore map[string]int64               // uid -> unix ms of last RevokeAll
}

func New(path string) *Manager {
	m := &Manager{path: path, sidToRec: map[string]Record{}, userToSids: map[string]map[string]struct{}{}, usedRefresh: map[string]map[string]struct{}{}, revokedBefore: map[string]int64{}}
	_ = m.load()
	return m
}

func (m *Manager) load() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var df diskFile
	ok, err := fsatomic.LoadJSON(m.path, &df)
	if err != nil || !ok {
		return err
	}
	m.sidToRec = map[string]Record{}
	m.userToSids = map[string]map[string]struct{}{}
	for _, r := range df.Sessions {
		m.sidToRec[r.SID] = r
		if m.userToSids[r.UID] == nil {
			m.userToSids[r.UID] = map[string]struct{}{}
		}
		m.userToSids[r.UID][r.SID] = struct{}{}
	}
	m.usedRefresh = map[string]map[string]struct{}{}
	for uid, list := range df.UsedRefresh {
		set := map[string]struct{}{}
		for _, id := range list {
			set[id] = struct{}{}
		}
		m.usedRefresh[uid] = set
	}
	m.revokedBefore = map[string]int64{}
	for uid, t := range df.RevokedBefore {
		m.revokedBefore[uid] = t
	}
	return nil
}

func (m *Manager) persistLocked() error {
	sessions := make([]Record, 0, len(m.sidToRec))
	for _, r := range m.sidToRec {
		sessions = append(sessions, r)
	}
	used := make(map[string][]string, len(m.usedRefresh))
	for uid, set := range m.usedRefresh {
		var ids []string
		for id := range set {
			ids = append(ids, id)
		}
		used[uid] = ids
	}
	return fsatomic.SaveJSON(context.TODO(), m.path, diskFile{Version: 1, Sessions: sessions, UsedRefresh: used, RevokedBefore: m.revokedBefore}, 0o600)
}

func (m *Manager) Create(uid, ua, ip string, ttl time.Duration) (Record, error) {
	sid := generateULID()
	now := time.Now().UTC()
	rec := Record{SID: sid, UID: uid, UAHash: sha256Hex(ua), IPHash: sha256Hex(maskIP(ip)), Exp: now.Add(ttl).Format(time.RFC3339), CreatedAt: now.Format(time.RFC3339), LastSeenAt: now.Format(time.RFC3339)}
	m.mu.Lock()
	m.sidToRec[sid] = rec
	if m.userToSids[uid] == nil {
		m.userToSids[uid] = map[string]struct{}{}
	}
	m.userToSids[uid][sid] = struct{}{}
	err := m.persistLocked()
	m.mu.Unlock()
	return rec, err
}

func (m *Manager) Verify(sid, ua, ip string) (string, bool) {
	m.mu.Lock()
	rec, ok := m.sidToRec[sid]
	if !ok {
		m.mu.Unlock()
		return "", false
	}
	if t, err := time.Parse(time.RFC3339, rec.Exp); err != nil || time.Now().UTC().After(t) {
		m.mu.Unlock()
		return "", false
	}
	if rec.UAHash != sha256Hex(ua) {
		m.mu.Unlock()
		return "", false
	}
	if rec.IPHash != sha256Hex(maskIP(ip)) {
		m.mu.Unlock()
		return "", false
	}
	// update last seen and persist (best-effort)
	rec.LastSeenAt = time.Now().UTC().Format(time.RFC3339)
	m.sidToRec[sid] = rec
	_ = m.persistLocked()
	m.mu.Unlock()
	return rec.UID, true
}

func (m *Manager) RevokeSID(sid string) error {
	m.mu.Lock()
	if rec, ok := m.sidToRec[sid]; ok {
		delete(m.sidToRec, sid)
		if set := m.userToSids[rec.UID]; set != nil {
			delete(set, sid)
		}
	}
	err := m.persistLocked()
	m.mu.Unlock()
	return err
}

func (m *Manager) RevokeAll(uid string) error {
	m.mu.Lock()
	// Refresh tokens issued until now stop working too.
	m.revokedBefore[uid] = time.Now().UnixMilli()
	if set := m.userToSids[uid]; set != nil {
		for sid := range set {
			delete(m.sidToRec, sid)
		}
		delete(m.userToSids, uid)
	}
	err := m.persistLocked()
	m.mu.Unlock()
	return err
}

func (m *Manager) RotateRefresh(uid, old string) (newID string, reuse bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.usedRefresh[uid] == nil {
		m.usedRefresh[uid] = map[string]struct{}{}
	}
	if _, seen := m.usedRefresh[uid][old]; seen {
		return "", true, nil
	}
	m.usedRefresh[uid][old] = struct{}{}
	newID = generateULID()
	if err = m.persistLocked(); err != nil {
		return "", false, err
	}
	return newID, false, nil
}

func (m *Manager) ListByUser(uid string) []Record {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []Record{}
	if set := m.userToSids[uid]; set != nil {
		for sid := range set {
			out = append(out, m.sidToRec[sid])
		}
	}
	return out
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func maskIP(ip string) string {
	host := ip
	if i := strings.IndexByte(ip, ':'); i >= 0 {
		host = ip[:i]
	}
	if net.ParseIP(host) == nil {
		return host
	}
	if strings.Contains(host, ":") {
		// IPv6: /64
		parts := strings.Split(host, ":")
		if len(parts) >= 4 {
			return strings.Join(parts[:4], ":")
		}
		return host
	}
	// IPv4: /24
	parts := strings.Split(host, ".")
	if len(parts) == 4 {
		return parts[0] + "." + parts[1] + "." + parts[2] + ".0"
	}
	return host
}

func generateULID() string {
	// 48-bit timestamp (ms since epoch) + 80 bits randomness -> 26 chars Base32 Crockford
	// Simplified generator sufficient for our use-case
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	now := time.Now().UTC().UnixMilli()
	var data [16]byte
	// timestamp big-endian in first 6 bytes
	for i := 5; i >= 0; i-- {
		data[i] = byte(now & 0xff)
		now >>= 8
	}
	// randomness
	t := time.Now().UnixNano()
	for i := 6; i < 16; i++ {
		data[i] = byte(t >> (8 * (i - 6)))
	}
	// encode 128 bits to 26 chars (base32 crockford without I,L,O,U)
	out := make([]byte, 26)
	var v uint16
	bits := 0
	idx := 0
	for i := 0; i < 16; i++ {
		v = (v << 8) | uint16(data[i])
		bits += 8
		for bits >= 5 {
			bits -= 5
			out[idx] = alphabet[(v>>bits)&0x1f]
			idx++
			if idx == 26 {
				return string(out)
			}
		}
	}
	if idx < 26 {
		out[idx] = alphabet[(v<<(5-bits))&0x1f]
	}
	return string(out)
}

// RefreshAllowed reports whether a refresh token for uid issued at issuedAtMs
// (Unix ms) is still valid, i.e. was issued after the user's last RevokeAll.
func (m *Manager) RefreshAllowed(uid string, issuedAtMs int64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return issuedAtMs > m.revokedBefore[uid]
}
