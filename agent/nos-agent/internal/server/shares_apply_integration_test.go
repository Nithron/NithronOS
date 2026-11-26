package server

// Runs the real share apply as root against a real Samba. It changes the
// host (users, groups, /srv/shares, /etc/samba), so it only runs when
// NOS_SHARES_INTEGRATION=1, in a throwaway container:
//
//   scripts/test-shares-integration.sh

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func sh(t *testing.T, name string, args ...string) string {
	t.Helper()
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return string(out)
}

func smbclient(user, pass, share, cmd string) (string, error) {
	args := []string{"//127.0.0.1/" + share, "-c", cmd}
	if user == "" {
		args = append(args, "-N")
	} else {
		args = append(args, "-U", user+"%"+pass)
	}
	out, err := exec.Command("smbclient", args...).CombinedOutput()
	return string(out), err
}

func TestSharesIntegration(t *testing.T) {
	if os.Getenv("NOS_SHARES_INTEGRATION") != "1" {
		t.Skip("set NOS_SHARES_INTEGRATION=1 (in a throwaway container) to run")
	}
	mux := buildMux()
	post := func(path string, body any) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b)))
		return rr
	}

	// SMB accounts. The second password is the old command-injection payload.
	const alicePass, bobPass = "alice-pass-1", `$(touch /tmp/pwned)x`
	for user, pass := range map[string]string{"alice": alicePass, "bob": bobPass} {
		if rr := post("/v1/smb/user-create", map[string]string{"username": user, "password": pass}); rr.Code != http.StatusOK {
			t.Fatalf("create %s: %d %s", user, rr.Code, rr.Body.String())
		}
	}
	if _, err := os.Stat("/tmp/pwned"); err == nil {
		t.Fatal("the SMB password was run as a shell command")
	}
	if rr := post("/v1/smb/user-create", map[string]string{"username": "root", "password": "rootpass1"}); rr.Code != http.StatusBadRequest {
		t.Fatalf("SMB password for root: %d", rr.Code)
	}

	req := SharesApplyRequest{
		Users:  []string{"alice", "bob"},
		Groups: []applyGroup{{Name: "family", Members: []string{"bob"}}},
		Shares: []applyShare{
			{Name: "media", Path: "/srv/shares/media", Description: "Films",
				SMB:    &applySMB{Enabled: true, Recycle: &applyRecyc{Enabled: true}},
				Owners: []string{"user:alice"}, Readers: []string{"group:family"}},
			{Name: "public", Path: "/srv/shares/public", SMB: &applySMB{Enabled: true, Guest: true}},
			{Name: "everyone", Path: "/srv/shares/everyone", SMB: &applySMB{Enabled: true}, Owners: []string{"group:users"}},
			{Name: "closed", Path: "/srv/shares/closed", SMB: &applySMB{Enabled: true}},
		},
	}
	rr := post("/v1/shares/apply", req)
	if rr.Code != http.StatusOK {
		t.Fatalf("apply: %d %s", rr.Code, rr.Body.String())
	}
	t.Logf("apply: %s", rr.Body.String())

	if out := sh(t, "getent", "group", "nos-family"); !strings.HasSuffix(strings.TrimSpace(out), ":bob") {
		t.Fatalf("nos-family: %s", out)
	}
	acl := sh(t, "getfacl", "-p", "/srv/shares/media")
	for _, want := range []string{"user:alice:rwx", "group:nos-family:r-x", "user:nos:rwx", "other::---", "default:user:alice:rwx"} {
		if !strings.Contains(acl, want) {
			t.Errorf("media ACL lacks %s:\n%s", want, acl)
		}
	}
	if out := sh(t, "testparm", "-s", "--suppress-prompt"); !strings.Contains(out, "[media]") || !strings.Contains(out, "valid users = alice @nos-family") {
		t.Fatalf("Samba does not read the shares:\n%s", out)
	}

	// smbd reads the configuration at start; (re)start it now.
	_ = exec.Command("pkill", "smbd").Run()
	time.Sleep(time.Second)
	sh(t, "smbd", "-D")
	time.Sleep(2 * time.Second)

	if err := os.WriteFile("/tmp/f.txt", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := smbclient("alice", alicePass, "media", "lcd /tmp; put f.txt"); err != nil {
		t.Fatalf("owner can't write: %v\n%s", err, out)
	}
	if out, err := smbclient("bob", bobPass, "media", "ls"); err != nil || !strings.Contains(out, "f.txt") {
		t.Fatalf("reader can't list: %v\n%s", err, out)
	}
	if out, err := smbclient("bob", bobPass, "media", "lcd /tmp; put f.txt g.txt"); err == nil && !strings.Contains(out, "NT_STATUS_ACCESS_DENIED") {
		t.Fatalf("reader could write:\n%s", out)
	}
	if out, err := smbclient("bob", bobPass, "everyone", "lcd /tmp; put f.txt"); err != nil {
		t.Fatalf("group:users can't write: %v\n%s", err, out)
	}
	if out, err := smbclient("", "", "public", "ls"); err != nil {
		t.Fatalf("guest can't open the guest share: %v\n%s", err, out)
	}
	if out, err := smbclient("alice", alicePass, "closed", "ls"); err == nil && !strings.Contains(out, "NT_STATUS") {
		t.Fatalf("a share without owners or readers was open:\n%s", out)
	}
	// The file alice wrote got the share's default ACL: bob may read it.
	if fa := sh(t, "getfacl", "-p", "/srv/shares/media/f.txt"); !strings.Contains(fa, "group:nos-family:r-x") {
		t.Errorf("new file did not inherit the share ACL:\n%s", fa)
	}

	// Removing a share removes it from Samba; its files stay.
	req.Shares = req.Shares[1:]
	if rr := post("/v1/shares/apply", req); rr.Code != http.StatusOK {
		t.Fatalf("apply after removal: %d %s", rr.Code, rr.Body.String())
	}
	if out := sh(t, "testparm", "-s", "--suppress-prompt"); strings.Contains(out, "[media]") {
		t.Fatalf("removed share still configured:\n%s", out)
	}
	if _, err := os.Stat("/srv/shares/media/f.txt"); err != nil {
		t.Fatalf("removing a share must keep its files: %v", err)
	}
}
