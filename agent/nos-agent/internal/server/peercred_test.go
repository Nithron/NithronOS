package server

import "testing"

// Regression (S7): the ISO made the agent socket mode 666, so any local user
// could run root actions (wipe disks, write files). Only root and the nos
// service account are allowed now.
func TestPeerAllowed(t *testing.T) {
	nos := serviceAccount{uid: 998, gid: 997}
	cases := []struct {
		name string
		cred peerCred
		want bool
	}{
		{"root", peerCred{uid: 0, gid: 0, known: true}, true},
		{"nos user", peerCred{uid: 998, gid: 100, known: true}, true},
		{"nos group", peerCred{uid: 1500, gid: 997, known: true}, true},
		{"ordinary user", peerCred{uid: 1000, gid: 1000, known: true}, false},
		{"unknown peer", peerCred{}, false},
	}
	for _, tc := range cases {
		if got := peerAllowed(tc.cred, nos); got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
	if peerAllowed(peerCred{uid: 1000, gid: 1000, known: true}, serviceAccount{uid: -1, gid: -1}) {
		t.Error("missing service account must not allow everyone")
	}
}
