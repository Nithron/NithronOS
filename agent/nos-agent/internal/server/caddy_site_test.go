package server

import (
	"errors"
	"strings"
	"testing"
)

// validateFails is a host whose `caddy validate` refuses the config.
type validateFails struct{ *fakeAppHost }

func (v validateFails) Run(name string, args ...string) ([]byte, error) {
	v.ran = append(v.ran, name+" "+strings.Join(args, " "))
	if name == "caddy" {
		return []byte("bad config"), errors.New("exit 1")
	}
	return nil, nil
}

func TestPublicSite(t *testing.T) {
	h := newFakeAppHost()
	if _, err := setPublicSite(h, PublicSiteRequest{Domain: "NAS.Example.org", Email: "me@example.org"}); err != nil {
		t.Fatal(err)
	}
	block := h.files[publicSitePath()]
	if !strings.Contains(block, "nas.example.org {") || !strings.Contains(block, "tls me@example.org") || !strings.Contains(block, "import nos_site") {
		t.Fatalf("site block:\n%s", block)
	}
	if got := strings.Join(h.ran, ";"); !strings.Contains(got, "caddy validate") || !strings.Contains(got, "reload-or-restart caddy") {
		t.Fatalf("ran: %s", got)
	}
	// Removing it.
	if _, err := setPublicSite(h, PublicSiteRequest{}); err != nil || len(h.gone) == 0 || h.gone[len(h.gone)-1] != publicSitePath() {
		t.Fatalf("remove: %v %v", err, h.gone)
	}
	// Bad input never reaches the file (it becomes Caddyfile text).
	for _, bad := range []PublicSiteRequest{
		{Domain: "evil.org {\n}\n:9999", Email: "me@example.org"},
		{Domain: "*.example.org", Email: "me@example.org"},
		{Domain: "localhost", Email: "me@example.org"},
		{Domain: "nas.example.org", Email: "me@example.org\n  import /etc/passwd"},
	} {
		if code, err := setPublicSite(newFakeAppHost(), bad); err == nil || code != 400 {
			t.Errorf("%+v: %d %v", bad, code, err)
		}
	}
	// A config Caddy refuses is taken back: no new site block stays.
	f := newFakeAppHost()
	if _, err := setPublicSite(validateFails{f}, PublicSiteRequest{Domain: "nas.example.org", Email: "me@example.org"}); err == nil {
		t.Fatal("a refused config was accepted")
	}
	if len(f.gone) == 0 || f.gone[len(f.gone)-1] != publicSitePath() {
		t.Fatalf("refused site block not removed: %v", f.gone)
	}
}
