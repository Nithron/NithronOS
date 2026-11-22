package firstboot

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

type recAnnouncer struct {
	mu      sync.Mutex
	codes   []string
	cleared int
}

func (r *recAnnouncer) Announce(st *State) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.codes) == 0 || r.codes[len(r.codes)-1] != st.OTP {
		r.codes = append(r.codes, st.OTP)
	}
}
func (r *recAnnouncer) Clear() { r.mu.Lock(); r.cleared++; r.mu.Unlock() }

func seqGen(codes ...string) func() (string, error) {
	i := 0
	return func() (string, error) {
		c := codes[i%len(codes)]
		i++
		return c, nil
	}
}

func newTestManager(t *testing.T, admin *bool, gen func() (string, error)) (*Manager, *fakeClock, *recAnnouncer, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state", "firstboot.json")
	clk := &fakeClock{t: time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)}
	ann := &recAnnouncer{}
	m := NewManager(path, Options{
		HasAdmin:  func() bool { return *admin },
		Announcer: ann,
		Now:       clk.Now,
		Generate:  gen,
	})
	return m, clk, ann, path
}

func TestCurrentIssuesAndReusesCode(t *testing.T) {
	admin := false
	m, _, ann, path := newTestManager(t, &admin, seqGen("111111", "222222"))
	st, err := m.Current()
	if err != nil || st == nil || st.OTP != "111111" {
		t.Fatalf("first: %v %+v", err, st)
	}
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("state file: %v %v", err, fi)
	}
	st2, _ := m.Current()
	if st2.OTP != "111111" {
		t.Fatalf("code should be reused, got %s", st2.OTP)
	}
	if len(ann.codes) != 1 || ann.codes[0] != "111111" {
		t.Fatalf("announced %v", ann.codes)
	}
}

// The reported bug: after 15 minutes the code expired and nothing issued a
// new one until nosd was restarted.
func TestExpiredCodeIsReplacedAutomatically(t *testing.T) {
	admin := false
	m, clk, ann, _ := newTestManager(t, &admin, seqGen("111111", "222222"))
	if _, err := m.Current(); err != nil {
		t.Fatal(err)
	}
	clk.Advance(DefaultTTL)
	st, err := m.Current()
	if err != nil || st == nil || st.OTP != "222222" {
		t.Fatalf("after expiry: %v %+v", err, st)
	}
	if !st.ExpiresAt.Equal(clk.Now().Add(DefaultTTL)) {
		t.Fatalf("new expiry %v", st.ExpiresAt)
	}
	if strings.Join(ann.codes, ",") != "111111,222222" {
		t.Fatalf("announced %v", ann.codes)
	}
}

func TestVerify(t *testing.T) {
	admin := false
	m, clk, _, _ := newTestManager(t, &admin, seqGen("111111", "222222", "333333"))
	if _, err := m.Current(); err != nil {
		t.Fatal(err)
	}
	if r, _ := m.Verify("111111"); r != Valid {
		t.Fatalf("valid code: %v", r)
	}
	if r, _ := m.Verify("999999"); r != Invalid {
		t.Fatalf("wrong code: %v", r)
	}
	if r, _ := m.Verify("11111"); r != Invalid {
		t.Fatalf("short code: %v", r)
	}
	// Expired: the right code is reported as expired and a new code issued.
	clk.Advance(DefaultTTL + time.Second)
	if r, _ := m.Verify("111111"); r != Expired {
		t.Fatalf("expired code: %v", r)
	}
	st, _ := m.Current()
	if st.OTP != "222222" {
		t.Fatalf("expected rotation on expiry, got %s", st.OTP)
	}
	admin = true
	if r, _ := m.Verify("222222"); r != NotRequired {
		t.Fatalf("after setup: %v", r)
	}
}

func TestTooManyWrongGuessesRotates(t *testing.T) {
	admin := false
	m, _, _, _ := newTestManager(t, &admin, seqGen("111111", "222222"))
	if _, err := m.Current(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < DefaultMaxAttempts; i++ {
		if r, _ := m.Verify("000000"); r != Invalid {
			t.Fatalf("guess %d: %v", i, r)
		}
	}
	// The old code no longer works, even though it was correct.
	if r, _ := m.Verify("111111"); r != Invalid {
		t.Fatalf("old code after lockout: %v", r)
	}
	if st, _ := m.Current(); st.OTP != "222222" {
		t.Fatalf("expected new code, got %s", st.OTP)
	}
}

func TestNoCodeOnceAdminExists(t *testing.T) {
	admin := true
	m, _, ann, path := newTestManager(t, &admin, seqGen("111111"))
	st, err := m.Current()
	if err != nil || st != nil {
		t.Fatalf("expected no code: %v %+v", err, st)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("no state file expected: %v", err)
	}
	if ann.cleared == 0 {
		t.Fatal("console banner should be cleared")
	}
}

func TestConsumeRemovesState(t *testing.T) {
	admin := false
	m, _, ann, path := newTestManager(t, &admin, seqGen("111111"))
	if _, err := m.Current(); err != nil {
		t.Fatal(err)
	}
	if err := m.Consume(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("state should be gone: %v", err)
	}
	if ann.cleared == 0 {
		t.Fatal("banner should be cleared")
	}
}

func TestLoadLegacySchemas(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "firstboot.json")
	created := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	_ = os.WriteFile(p, []byte(`{"otp":"123456","created_at":"`+created+`","used":false}`), 0o600)
	st, err := New(p).Load()
	if err != nil || st == nil || st.OTP != "123456" || st.Expired(time.Now()) {
		t.Fatalf("legacy: %v %+v", err, st)
	}
	_ = os.WriteFile(p, []byte(`{"otp":"123456","created_at":"`+created+`","used":true}`), 0o600)
	if st, _ := New(p).Load(); st != nil {
		t.Fatalf("used legacy state should count as none: %+v", st)
	}
	// The ISO's old generate-otp script wrote issued_at/expires_at as strings.
	_ = os.WriteFile(p, []byte(`{"firstBoot":true,"otp":"654321","issued_at":"2026-11-21T10:00:00Z","expires_at":"2026-11-21T10:15:00Z"}`), 0o600)
	st, err = New(p).Load()
	if err != nil || st == nil || st.OTP != "654321" || !st.ExpiresAt.Equal(time.Date(2026, 11, 21, 10, 15, 0, 0, time.UTC)) {
		t.Fatalf("script schema: %v %+v", err, st)
	}
}

func TestSaveFailsUnderNonDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write anywhere")
	}
	dir := t.TempDir()
	notDir := filepath.Join(dir, "state")
	if err := os.WriteFile(notDir, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	admin := false
	m := NewManager(filepath.Join(notDir, "firstboot.json"), Options{HasAdmin: func() bool { return admin }})
	if _, err := m.Current(); err == nil {
		t.Fatal("expected an error when the state dir can't be created")
	}
}

func TestGenerateCode(t *testing.T) {
	re := regexp.MustCompile(`^\d{6}$`)
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		c, err := GenerateCode()
		if err != nil || !re.MatchString(c) {
			t.Fatalf("bad code %q: %v", c, err)
		}
		seen[c] = true
	}
	if len(seen) < 45 {
		t.Fatalf("codes not random enough: %d distinct of 50", len(seen))
	}
}

func TestIssueAnnouncer(t *testing.T) {
	dir := t.TempDir()
	var log bytes.Buffer
	a := &IssueAnnouncer{
		Path:     filepath.Join(dir, "firstboot.issue"),
		Log:      &log,
		Addrs:    func() []string { return []string{"192.168.1.20", "fd00::5"} },
		Location: time.UTC,
	}
	st := &State{OTP: "123456", ExpiresAt: time.Date(2026, 11, 21, 12, 15, 0, 0, time.UTC)}
	a.Announce(st)
	b, err := os.ReadFile(a.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"https://192.168.1.20/", "https://[fd00::5]/", "123 456", "12:15 UTC"} {
		if !bytes.Contains(b, []byte(want)) {
			t.Fatalf("banner missing %q:\n%s", want, b)
		}
	}
	if !strings.Contains(log.String(), "setup code: 123456") {
		t.Fatalf("log: %q", log.String())
	}
	// Same code again: no second log line, file untouched.
	fi1, _ := os.Stat(a.Path)
	a.Announce(st)
	fi2, _ := os.Stat(a.Path)
	if strings.Count(log.String(), "setup code") != 1 || !fi1.ModTime().Equal(fi2.ModTime()) {
		t.Fatalf("unchanged code should not rewrite or relog")
	}
	a.Clear()
	if _, err := os.Stat(a.Path); !os.IsNotExist(err) {
		t.Fatalf("banner should be removed: %v", err)
	}
	a.Clear() // idempotent
}

func TestIssueAnnouncerNoNetwork(t *testing.T) {
	a := &IssueAnnouncer{Path: filepath.Join(t.TempDir(), "x.issue"), Addrs: func() []string { return nil }}
	a.Announce(&State{OTP: "123456", ExpiresAt: time.Now().Add(time.Minute)})
	b, _ := os.ReadFile(a.Path)
	if !bytes.Contains(b, []byte("No network address yet")) {
		t.Fatalf("banner:\n%s", b)
	}
}
