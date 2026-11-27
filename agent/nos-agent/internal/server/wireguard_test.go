package server

import (
	"errors"
	"strings"
	"testing"
)

type fakeWG struct {
	files map[string]string
	ran   []string
}

func (f *fakeWG) Run(name string, args ...string) ([]byte, error) {
	f.ran = append(f.ran, name+" "+strings.Join(args, " "))
	if name == "wg" && args[0] == "genkey" {
		return []byte("yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=\n"), nil
	}
	return nil, nil
}
func (f *fakeWG) RunInput(_, name string, args ...string) ([]byte, error) {
	if name == "wg" && args[0] == "pubkey" {
		return []byte("xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=\n"), nil
	}
	return nil, errors.New("unexpected")
}
func (f *fakeWG) ReadFile(p string) ([]byte, error) {
	if s, ok := f.files[p]; ok {
		return []byte(s), nil
	}
	return nil, errors.New("missing")
}
func (f *fakeWG) WriteFile0600(p string, d []byte) error { f.files[p] = string(d); return nil }

const peerKey = "HIgo9xNzJMWLKASShiTqIybxZ0U3wGLiUeJ1PKf8ykw="

// The server key is made once and kept by the agent; nosd gets the public
// key. Peers are checked before anything is written.
func TestApplyWireGuard(t *testing.T) {
	f := &fakeWG{files: map[string]string{}}
	req := WireGuardApplyRequest{Enabled: true, ListenPort: 51820, Address: "10.8.0.1/24",
		Peers: []WireGuardPeerSpec{{PublicKey: peerKey, AllowedIPs: []string{"10.8.0.2/32"}}}}
	if err := validateWGApply(req); err != nil {
		t.Fatal(err)
	}
	pub, err := applyWireGuard(f, req)
	if err != nil || pub != "xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=" {
		t.Fatalf("apply: %q %v", pub, err)
	}
	conf := f.files["/etc/wireguard/wg0.conf"]
	for _, want := range []string{"ListenPort = 51820", "Address = 10.8.0.1/24", "PrivateKey = yAnz5TF", "PublicKey = " + peerKey, "AllowedIPs = 10.8.0.2/32"} {
		if !strings.Contains(conf, want) {
			t.Errorf("wg0.conf lacks %q:\n%s", want, conf)
		}
	}
	// A second apply reuses the key.
	f.ran = nil
	if _, err := applyWireGuard(f, req); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(f.ran, ";"), "genkey") {
		t.Fatal("the server key was made again")
	}
	if got := strings.Join(f.ran, ";"); !strings.Contains(got, "reload-or-restart wg-quick@wg0") {
		t.Fatalf("ran: %s", got)
	}

	for name, bad := range map[string]WireGuardApplyRequest{
		"port":    {Enabled: true, ListenPort: 0, Address: "10.8.0.1/24"},
		"address": {Enabled: true, ListenPort: 51820, Address: "10.8.0.1"},
		"key":     {Enabled: true, ListenPort: 51820, Address: "10.8.0.1/24", Peers: []WireGuardPeerSpec{{PublicKey: "x\nPostUp = rm -rf /", AllowedIPs: []string{"10.8.0.2/32"}}}},
		"ips":     {Enabled: true, ListenPort: 51820, Address: "10.8.0.1/24", Peers: []WireGuardPeerSpec{{PublicKey: peerKey, AllowedIPs: []string{"10.8.0.2/32\nPostUp=x"}}}},
	} {
		if validateWGApply(bad) == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestParseWGDump(t *testing.T) {
	out := "priv\tpub\t51820\toff\n" + peerKey + "\t(none)\t203.0.113.5:4242\t10.8.0.2/32\t1795700000\t1200\t3400\toff\n"
	got := parseWGDump(out)
	if len(got) != 1 || got[0].Endpoint != "203.0.113.5:4242" || got[0].LastHandshake != 1795700000 || got[0].BytesTX != 3400 {
		t.Fatalf("%+v", got)
	}
	if len(parseWGDump("priv\tpub\t51820\toff\n")) != 0 {
		t.Fatal("no peers")
	}
}
