package firstboot

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"math/big"
	"sync"
	"time"
)

const (
	// DefaultTTL is how long a code stays valid.
	DefaultTTL = 15 * time.Minute
	// DefaultMaxAttempts is how many wrong guesses a code survives before it
	// is replaced with a new one.
	DefaultMaxAttempts = 5
	// CodeLength is the number of digits in a code.
	CodeLength = 6
)

// VerifyResult is the outcome of Manager.Verify.
type VerifyResult int

const (
	// Valid means the code matched the current, unexpired code.
	Valid VerifyResult = iota
	// Invalid means the code didn't match.
	Invalid
	// Expired means the code matched a code that has expired (a new one has
	// been issued).
	Expired
	// NotRequired means setup is already complete; no code is accepted.
	NotRequired
)

// Announcer is told about every new code and when setup completes, so the code
// can be shown where a person at the machine will see it.
type Announcer interface {
	// Announce is called with the current code on every check; it should
	// only do work when something it shows (code, addresses) changed.
	Announce(st *State)
	// Clear removes whatever Announce displayed; it must be cheap when there
	// is nothing to remove.
	Clear()
}

// Options configure a Manager.
type Options struct {
	// HasAdmin reports whether the first administrator exists. While it
	// returns false the manager keeps a valid code available.
	HasAdmin func() bool
	// Announcer shows codes on the console; nil disables announcing.
	Announcer   Announcer
	TTL         time.Duration
	MaxAttempts int
	// Now and Generate are test seams.
	Now      func() time.Time
	Generate func() (string, error)
}

// Manager keeps exactly one valid setup code while no administrator exists.
// It is safe for concurrent use.
type Manager struct {
	store *Store
	opts  Options
	mu    sync.Mutex
}

// NewManager returns a Manager for the state file at path.
func NewManager(path string, opts Options) *Manager {
	if opts.TTL <= 0 {
		opts.TTL = DefaultTTL
	}
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = DefaultMaxAttempts
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Generate == nil {
		opts.Generate = GenerateCode
	}
	if opts.HasAdmin == nil {
		opts.HasAdmin = func() bool { return false }
	}
	return &Manager{store: New(path), opts: opts}
}

// GenerateCode returns a uniformly random 6-digit code from crypto/rand.
func GenerateCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("generate setup code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// Required reports whether setup still needs a code (no administrator yet).
func (m *Manager) Required() bool { return !m.opts.HasAdmin() }

// Current returns the valid code, issuing a new one when there is none, it
// has expired, or it has used up its attempts. It returns (nil, nil) once an
// administrator exists.
func (m *Manager) Current() (*State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.currentLocked()
}

func (m *Manager) currentLocked() (*State, error) {
	if m.opts.HasAdmin() {
		m.clearLocked()
		return nil, nil
	}
	st, err := m.store.Load()
	if err != nil {
		return nil, err
	}
	if st == nil || st.Expired(m.opts.Now()) || st.Attempts >= m.opts.MaxAttempts {
		return m.rotateLocked()
	}
	m.announceLocked(st)
	return st, nil
}

// Rotate replaces the current code with a new one (for example from the
// local recovery endpoint).
func (m *Manager) Rotate() (*State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.opts.HasAdmin() {
		return nil, nil
	}
	return m.rotateLocked()
}

func (m *Manager) rotateLocked() (*State, error) {
	code, err := m.opts.Generate()
	if err != nil {
		return nil, err
	}
	now := m.opts.Now().UTC()
	st := &State{OTP: code, IssuedAt: now, ExpiresAt: now.Add(m.opts.TTL)}
	if err := m.store.Save(st); err != nil {
		return nil, err
	}
	m.announceLocked(st)
	return st, nil
}

// Verify checks code against the current code. A wrong guess counts against
// the code; after MaxAttempts wrong guesses the code is replaced.
func (m *Manager) Verify(code string) (VerifyResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.opts.HasAdmin() {
		return NotRequired, nil
	}
	st, err := m.store.Load()
	if err != nil {
		return Invalid, err
	}
	if st == nil {
		if _, err := m.rotateLocked(); err != nil {
			return Invalid, err
		}
		return Invalid, nil
	}
	match := len(code) == len(st.OTP) && subtle.ConstantTimeCompare([]byte(code), []byte(st.OTP)) == 1
	if st.Expired(m.opts.Now()) {
		if _, err := m.rotateLocked(); err != nil {
			return Invalid, err
		}
		if match {
			return Expired, nil
		}
		return Invalid, nil
	}
	if match && st.Attempts < m.opts.MaxAttempts {
		return Valid, nil
	}
	st.Attempts++
	if st.Attempts >= m.opts.MaxAttempts {
		if _, err := m.rotateLocked(); err != nil {
			return Invalid, err
		}
		return Invalid, nil
	}
	if err := m.store.Save(st); err != nil {
		return Invalid, err
	}
	return Invalid, nil
}

// Consume removes the code once the first administrator has been created.
func (m *Manager) Consume() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clearLocked()
	return m.store.Remove()
}

func (m *Manager) clearLocked() {
	if m.opts.Announcer != nil {
		m.opts.Announcer.Clear()
	}
}

func (m *Manager) announceLocked(st *State) {
	if m.opts.Announcer != nil && st != nil {
		m.opts.Announcer.Announce(st)
	}
}

// Run keeps a valid code available and on the console until an administrator
// exists or ctx is cancelled. It checks every interval (and right away).
func (m *Manager) Run(ctx context.Context, interval time.Duration, onErr func(error)) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if _, err := m.Current(); err != nil && onErr != nil {
			onErr(err)
		}
		if !m.Required() {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
