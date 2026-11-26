package validate

import (
	"strings"
	"testing"
)

func TestShareFields(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"ShareName ok", ShareName("media_2024")},
		{"ShareName newline", errIfNil(ShareName("x]\n[global"))},
		{"PathUnder srv", PathUnder(ShareRoots, "/srv/shares/media")},
		{"PathUnder mnt", PathUnder(ShareRoots, "/mnt/pool1")},
		{"PathUnder etc", errIfNil(PathUnder(ShareRoots, "/etc"))},
		{"PathUnder dotdot", errIfNil(PathUnder(ShareRoots, "/srv/../etc"))},
		{"PathUnder newline", errIfNil(PathUnder(ShareRoots, "/srv/a\n   path = /"))},
		{"Description ok", Description("Family photos, 2024")},
		{"Description newline", errIfNil(Description("x\n   guest ok = yes"))},
		{"Description long", errIfNil(Description(strings.Repeat("a", 257)))},
		{"Principal user", Principal("alice")},
		{"Principal group", Principal("@family")},
		{"Principal space", errIfNil(Principal("alice bob"))},
		{"Principal newline", errIfNil(Principal("a\nb"))},
		{"NFSClient cidr", NFSClient("192.168.1.0/24")},
		{"NFSClient host", NFSClient("*.lan")},
		{"NFSClient v6", NFSClient("fd00::/8")},
		{"NFSClient opts", errIfNil(NFSClient("*(rw,no_root_squash)"))},
		{"NFSClient space", errIfNil(NFSClient("a b"))},
	}
	for _, c := range cases {
		if c.err != nil {
			t.Errorf("%s: %v", c.name, c.err)
		}
	}
}

// errIfNil turns "this must fail" into an error when it didn't.
func errIfNil(err error) error {
	if err == nil {
		return errExpected
	}
	return nil
}
