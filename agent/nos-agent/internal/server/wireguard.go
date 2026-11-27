package server

// WireGuard for nosd, which runs as the nos user and could neither write
// /etc/wireguard nor run systemctl or read the tunnel's statistics. The
// server's private key is made and kept here (/etc/wireguard, root only);
// nosd sends the desired tunnel (port, address, peers' public keys) and gets
// the public key back.

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	wgDir     = "/etc/wireguard"
	wgIface   = "wg0"
	wgUnit    = "wg-quick@wg0"
	wgKeyFile = "nos-server.key"
)

// WireGuardPeerSpec is one client device.
type WireGuardPeerSpec struct {
	PublicKey    string   `json:"public_key"`
	PresharedKey string   `json:"preshared_key,omitempty"`
	AllowedIPs   []string `json:"allowed_ips"`
}

// WireGuardApplyRequest is the whole tunnel; enabled=false stops it.
type WireGuardApplyRequest struct {
	Enabled    bool                `json:"enabled"`
	ListenPort int                 `json:"listen_port"`
	Address    string              `json:"address"` // the server's address in the tunnel, CIDR
	Peers      []WireGuardPeerSpec `json:"peers"`
}

// wgHost runs commands and touches files; tests replace it.
type wgHost interface {
	Run(name string, args ...string) ([]byte, error)
	RunInput(input, name string, args ...string) ([]byte, error)
	ReadFile(path string) ([]byte, error)
	WriteFile0600(path string, data []byte) error
}

type realWGHost struct{}

func (realWGHost) Run(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

func (realWGHost) RunInput(input, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(input)
	return cmd.Output()
}

func (realWGHost) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func (realWGHost) WriteFile0600(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".nos-tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

var newWGHost = func() wgHost { return realWGHost{} }

func validWGKey(k string) bool {
	b, err := base64.StdEncoding.DecodeString(k)
	return err == nil && len(b) == 32
}

func validateWGApply(req WireGuardApplyRequest) error {
	if !req.Enabled {
		return nil
	}
	if req.ListenPort < 1 || req.ListenPort > 65535 {
		return fmt.Errorf("listen port must be 1-65535")
	}
	if _, _, err := net.ParseCIDR(req.Address); err != nil {
		return fmt.Errorf("address must be a CIDR, e.g. 10.8.0.1/24")
	}
	if len(req.Peers) > 1000 {
		return fmt.Errorf("too many peers")
	}
	for i, p := range req.Peers {
		if !validWGKey(p.PublicKey) {
			return fmt.Errorf("peer %d: bad public key", i)
		}
		if p.PresharedKey != "" && !validWGKey(p.PresharedKey) {
			return fmt.Errorf("peer %d: bad preshared key", i)
		}
		if len(p.AllowedIPs) == 0 {
			return fmt.Errorf("peer %d: no allowed IPs", i)
		}
		for _, a := range p.AllowedIPs {
			if _, _, err := net.ParseCIDR(a); err != nil {
				return fmt.Errorf("peer %d: bad allowed IP %q", i, a)
			}
		}
	}
	return nil
}

// serverKey returns the private key, making it on first use.
func serverKey(h wgHost) (string, error) {
	path := filepath.Join(wgDir, wgKeyFile)
	if b, err := h.ReadFile(path); err == nil && validWGKey(strings.TrimSpace(string(b))) {
		return strings.TrimSpace(string(b)), nil
	}
	out, err := h.Run("wg", "genkey")
	if err != nil {
		return "", fmt.Errorf("wg genkey: %s", strings.TrimSpace(string(out)))
	}
	key := strings.TrimSpace(string(out))
	if !validWGKey(key) {
		return "", fmt.Errorf("wg genkey gave no key")
	}
	if err := h.WriteFile0600(path, []byte(key+"\n")); err != nil {
		return "", err
	}
	return key, nil
}

func renderWGConfig(req WireGuardApplyRequest, privateKey string) string {
	var b strings.Builder
	b.WriteString("# Managed by nOS (nos-agent). Changes here are overwritten.\n")
	fmt.Fprintf(&b, "[Interface]\nAddress = %s\nListenPort = %d\nPrivateKey = %s\n", req.Address, req.ListenPort, privateKey)
	for _, p := range req.Peers {
		fmt.Fprintf(&b, "\n[Peer]\nPublicKey = %s\n", p.PublicKey)
		if p.PresharedKey != "" {
			fmt.Fprintf(&b, "PresharedKey = %s\n", p.PresharedKey)
		}
		fmt.Fprintf(&b, "AllowedIPs = %s\n", strings.Join(p.AllowedIPs, ", "))
	}
	return b.String()
}

func applyWireGuard(h wgHost, req WireGuardApplyRequest) (string, error) {
	if !req.Enabled {
		if out, err := h.Run("systemctl", "disable", "--now", wgUnit); err != nil {
			return "", fmt.Errorf("stop WireGuard: %s", strings.TrimSpace(string(out)))
		}
		return "", nil
	}
	key, err := serverKey(h)
	if err != nil {
		return "", err
	}
	pub, err := h.RunInput(key+"\n", "wg", "pubkey")
	if err != nil {
		return "", fmt.Errorf("wg pubkey failed")
	}
	if err := h.WriteFile0600(filepath.Join(wgDir, wgIface+".conf"), []byte(renderWGConfig(req, key))); err != nil {
		return "", err
	}
	if out, err := h.Run("systemctl", "enable", wgUnit); err != nil {
		return "", fmt.Errorf("enable WireGuard: %s", strings.TrimSpace(string(out)))
	}
	// wg-quick@'s reload applies peer changes without dropping the tunnel.
	if out, err := h.Run("systemctl", "reload-or-restart", wgUnit); err != nil {
		return "", fmt.Errorf("start WireGuard: %s", strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(pub)), nil
}

func handleWireGuardApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req WireGuardApplyRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validateWGApply(req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	pub, err := applyWireGuard(newWGHost(), req)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	logAuthPriv(fmt.Sprintf("wireguard.apply enabled=%v peers=%d", req.Enabled, len(req.Peers)))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "public_key": pub})
}

// WireGuardPeerStatus is one peer's live state (wg show dump).
type WireGuardPeerStatus struct {
	PublicKey     string `json:"public_key"`
	Endpoint      string `json:"endpoint,omitempty"`
	LastHandshake int64  `json:"last_handshake"` // unix seconds, 0 never
	BytesRX       int64  `json:"bytes_rx"`
	BytesTX       int64  `json:"bytes_tx"`
}

// parseWGDump reads `wg show wg0 dump`: the first line is the interface,
// then one line per peer (pubkey psk endpoint allowed-ips handshake rx tx keepalive).
func parseWGDump(out string) []WireGuardPeerStatus {
	peers := []WireGuardPeerStatus{}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, l := range lines[min(1, len(lines)):] {
		f := strings.Split(l, "\t")
		if len(f) < 7 {
			continue
		}
		hs, _ := strconv.ParseInt(f[4], 10, 64)
		rx, _ := strconv.ParseInt(f[5], 10, 64)
		tx, _ := strconv.ParseInt(f[6], 10, 64)
		ep := f[2]
		if ep == "(none)" {
			ep = ""
		}
		peers = append(peers, WireGuardPeerStatus{PublicKey: f[0], Endpoint: ep, LastHandshake: hs, BytesRX: rx, BytesTX: tx})
	}
	return peers
}

func handleWireGuardStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	h := newWGHost()
	out, err := h.Run("wg", "show", wgIface, "dump")
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"running": false, "peers": []WireGuardPeerStatus{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"running": true, "peers": parseWGDump(string(out))})
}
