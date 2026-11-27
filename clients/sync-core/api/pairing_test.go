package api

import (
	"strings"
	"testing"
)

func TestParsePairingLink(t *testing.T) {
	key := strings.Repeat("Ab", 32)
	p, err := ParsePairingLink("nos://sync?server=https%3A%2F%2Fnas.local%2F&token=nos_dt_abc&refresh=nos_rt_def&key=" + key + "&name=Laptop")
	if err != nil {
		t.Fatal(err)
	}
	if p.ServerURL != "https://nas.local" || p.DeviceToken != "nos_dt_abc" || p.RefreshToken != "nos_rt_def" ||
		p.ServerKey != strings.ToLower(key) || p.DeviceName != "Laptop" {
		t.Fatalf("%+v", p)
	}
	for _, bad := range []string{
		"https://nas.local",
		"nos://discover?server=https://nas.local",
		"nos://sync?token=nos_dt_abc",
		"nos://sync?server=ftp://x&token=nos_dt_abc",
		"nos://sync?server=https://nas.local&token=abc",
		"nos://sync?server=https://nas.local&token=nos_dt_abc&key=short",
	} {
		if _, err := ParsePairingLink(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
