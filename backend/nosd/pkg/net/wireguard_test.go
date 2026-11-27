package net

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

type fakeWGAgent struct{ applies []map[string]any }

func (f *fakeWGAgent) GetJSON(_ context.Context, _ string, out any) error {
	return json.Unmarshal([]byte(`{"running":true,"peers":[]}`), out)
}

func (f *fakeWGAgent) PostJSON(_ context.Context, path string, body any, out any) error {
	if path != "/v1/wireguard/apply" {
		return errors.New("unexpected " + path)
	}
	b, _ := json.Marshal(body)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	f.applies = append(f.applies, m)
	return json.Unmarshal([]byte(`{"ok":true,"public_key":"xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg="}`), out)
}

func TestWireGuardThroughTheAgent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wireguard.json")
	agent := &fakeWGAgent{}
	wm, err := NewWireGuardManager(path, agent)
	if err != nil {
		t.Fatal(err)
	}
	if err := wm.Enable("10.8.0.0/24", 51820, "", nil); !errors.Is(err, ErrEndpointRequired) {
		t.Fatalf("no endpoint: %v", err)
	}
	if err := wm.Enable("10.8.0.0/24", 51821, "nas.example.org", nil); err != nil {
		t.Fatal(err)
	}
	if a := agent.applies[0]; a["address"] != "10.8.0.1/24" || a["listen_port"].(float64) != 51821 {
		t.Fatalf("apply: %+v", a)
	}
	peer, err := wm.AddPeer("phone", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Address = 10.8.0.2/32", "Endpoint = nas.example.org:51821", "AllowedIPs = 10.8.0.0/24", "PresharedKey = ", "PublicKey = xTIBA5"} {
		if !strings.Contains(peer.Config, want) {
			t.Errorf("client config lacks %q:\n%s", want, peer.Config)
		}
	}
	if strings.Contains(peer.Config, "0.0.0.0/0") {
		t.Error("the client must not route all its traffic through the tunnel")
	}
	last := agent.applies[len(agent.applies)-1]
	peers := last["peers"].([]any)
	if len(peers) != 1 || peers[0].(map[string]any)["preshared_key"] == "" {
		t.Fatalf("peer sent to the agent: %+v", last)
	}

	// A restart keeps the tunnel and its devices.
	wm2, err := NewWireGuardManager(path, agent)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := wm2.GetState()
	if !st.Enabled || len(st.Peers) != 1 || st.PublicKey == "" || wm2.psk[st.Peers[0].ID] == "" {
		t.Fatalf("after restart: %+v", st)
	}
	if err := wm2.RemovePeer(st.Peers[0].ID); err != nil {
		t.Fatal(err)
	}
	if last := agent.applies[len(agent.applies)-1]; len(last["peers"].([]any)) != 0 {
		t.Fatalf("removed peer still sent: %+v", last)
	}
}
