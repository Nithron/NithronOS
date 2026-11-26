package firstboot

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// DefaultIssuePath is where the console banner is written. The nosd package
// links /etc/issue.d/50-nos-setup.issue to it (tmpfiles.d), so agetty
// shows it above every login prompt, and a systemd path unit runs
// `agetty --reload` whenever it changes.
const DefaultIssuePath = "/run/nos/firstboot.issue"

// IssueAnnouncer shows the setup code on the local console (tty and serial)
// through an agetty issue snippet, and logs each new code once.
type IssueAnnouncer struct {
	Path string
	// Log receives one line per new code (nosd's stdout goes to the journal
	// and the console). Nil disables logging.
	Log io.Writer
	// Addrs lists the addresses to show; nil uses the machine's interfaces.
	Addrs func() []string
	// Location is the time zone for the expiry time; nil uses time.Local.
	Location *time.Location

	mu       sync.Mutex
	lastCode string
}

// Announce writes the banner when its content changed and logs new codes.
func (a *IssueAnnouncer) Announce(st *State) {
	a.mu.Lock()
	defer a.mu.Unlock()
	addrs := a.addrs()
	if a.Log != nil && st.OTP != a.lastCode {
		_, _ = fmt.Fprintf(a.Log, "nOS first-boot setup code: %s (valid until %s); open %s\n",
			st.OTP, a.expiry(st), strings.Join(urls(addrs), " or "))
	}
	a.lastCode = st.OTP
	content := a.render(st, addrs)
	if old, err := os.ReadFile(a.Path); err == nil && bytes.Equal(old, content) {
		return
	}
	if err := writeFileAtomic(a.Path, content, 0o644); err != nil && a.Log != nil {
		_, _ = fmt.Fprintf(a.Log, "first-boot: cannot write console banner %s: %v\n", a.Path, err)
	}
}

// Clear removes the banner.
func (a *IssueAnnouncer) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastCode = ""
	if err := os.Remove(a.Path); err != nil && !errors.Is(err, os.ErrNotExist) && a.Log != nil {
		_, _ = fmt.Fprintf(a.Log, "first-boot: cannot remove console banner %s: %v\n", a.Path, err)
	}
}

func (a *IssueAnnouncer) addrs() []string {
	if a.Addrs != nil {
		return a.Addrs()
	}
	return LocalAddrs()
}

func (a *IssueAnnouncer) expiry(st *State) string {
	loc := a.Location
	if loc == nil {
		loc = time.Local
	}
	return st.ExpiresAt.In(loc).Format("15:04 MST")
}

// render builds the issue snippet. agetty treats backslash as an escape
// character, so any backslash is doubled.
func (a *IssueAnnouncer) render(st *State, addrs []string) []byte {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  nOS first-boot setup\n")
	b.WriteString("  --------------------------\n")
	if u := urls(addrs); len(u) > 0 {
		b.WriteString("  Open in a browser:     " + u[0] + "\n")
		for _, x := range u[1:] {
			b.WriteString("                         " + x + "\n")
		}
	} else {
		b.WriteString("  No network address yet. Connect this machine to your network;\n")
		b.WriteString("  the address appears here once it has one.\n")
	}
	fmt.Fprintf(&b, "  One-time setup code:   %s %s\n", st.OTP[:3], st.OTP[3:])
	fmt.Fprintf(&b, "  Valid until:           %s (a new code appears here when it expires)\n", a.expiry(st))
	b.WriteString("\n")
	return []byte(strings.ReplaceAll(b.String(), `\`, `\\`))
}

func urls(addrs []string) []string {
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if strings.Contains(a, ":") {
			out = append(out, "https://["+a+"]/")
		} else {
			out = append(out, "https://"+a+"/")
		}
	}
	return out
}

// LocalAddrs returns the machine's global unicast addresses on physical-ish
// interfaces (no loopback, container bridges or veths), IPv4 first.
func LocalAddrs() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var v4, v6 []string
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 || virtualIface(ifc.Name) {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, ad := range addrs {
			ipn, ok := ad.(*net.IPNet)
			if !ok || !ipn.IP.IsGlobalUnicast() {
				continue
			}
			if ip4 := ipn.IP.To4(); ip4 != nil {
				v4 = append(v4, ip4.String())
			} else {
				v6 = append(v6, ipn.IP.String())
			}
		}
	}
	sort.Strings(v4)
	sort.Strings(v6)
	// Keep the banner short: at most three addresses.
	out := append(v4, v6...)
	if len(out) > 3 {
		out = out[:3]
	}
	return out
}

func virtualIface(name string) bool {
	for _, p := range []string{"docker", "br-", "veth", "virbr", "lxc", "cni", "flannel", "tun", "wg"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".issue-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
