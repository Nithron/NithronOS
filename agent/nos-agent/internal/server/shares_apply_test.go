package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

type fakeSharesHost struct {
	users  map[string]bool
	groups map[string]bool
	files  map[string]string
	ran    []string
	active map[string]bool
}

func newFakeSharesHost() *fakeSharesHost {
	return &fakeSharesHost{
		users:  map[string]bool{"alice": true, "bob": true},
		groups: map[string]bool{},
		files:  map[string]string{},
		active: map[string]bool{"smbd": true, "nfs-server": true},
	}
}

func (f *fakeSharesHost) UserOK(n string) bool           { return f.users[n] }
func (f *fakeSharesHost) GroupExists(n string) bool      { return f.groups[n] }
func (f *fakeSharesHost) Active(unit string) bool        { return f.active[unit] }
func (f *fakeSharesHost) EnsureDir(string) (bool, error) { return false, nil }
func (f *fakeSharesHost) WriteFile(p string, d []byte) error {
	f.files[p] = string(d)
	return nil
}
func (f *fakeSharesHost) Run(name string, args ...string) ([]byte, error) {
	f.ran = append(f.ran, name+" "+strings.Join(args, " "))
	if name == "groupadd" {
		f.groups[args[len(args)-1]] = true
	}
	if name == "getent" {
		if !f.groups[args[1]] {
			return nil, errors.New("exit status 2")
		}
		return []byte(args[1] + ":x:997:\n"), nil
	}
	return nil, nil
}

func (f *fakeSharesHost) ranLike(prefix string) []string {
	var out []string
	for _, r := range f.ran {
		if strings.HasPrefix(r, prefix) {
			out = append(out, r)
		}
	}
	return out
}

func TestApplySharesWritesOneFilePerService(t *testing.T) {
	h := newFakeSharesHost()
	req := SharesApplyRequest{
		Users:  []string{"alice", "bob", "carol"},
		Groups: []applyGroup{{Name: "family", Members: []string{"bob", "carol"}}},
		Shares: []applyShare{
			{Name: "media", Path: "/srv/shares/media", Description: "Films",
				SMB:     &applySMB{Enabled: true, TimeMachine: true, Recycle: &applyRecyc{Enabled: true}},
				NFS:     &applyNFS{Enabled: true, Networks: []string{"10.0.0.0/8"}, ReadOnly: true},
				Owners:  []string{"user:alice"},
				Readers: []string{"group:family", "user:carol"}},
			{Name: "public", Path: "/srv/shares/public", SMB: &applySMB{Enabled: true, Guest: true}},
			{Name: "everyone", Path: "/srv/shares/everyone", SMB: &applySMB{Enabled: true}, Owners: []string{"group:users"}},
			{Name: "nfsonly", Path: "/mnt/pool/nfsonly", NFS: &applyNFS{Enabled: true, Networks: []string{"192.168.1.0/24", "host.lan"}}},
		},
	}
	if err := validateSharesApply(&req); err != nil {
		t.Fatal(err)
	}
	resp, err := applyShares(h, req)
	if err != nil {
		t.Fatal(err)
	}

	smb := h.files[smbIncludePath]
	for _, want := range []string{
		"[media]\n   path = /srv/shares/media\n   comment = Films\n",
		"   valid users = alice @nos-family\n   read list = @nos-family\n",
		"   vfs objects = catia fruit streams_xattr recycle\n   fruit:time machine = yes\n",
		"   recycle:repository = .recycle\n",
		"[public]\n   path = /srv/shares/public\n   browseable = yes\n   read only = no\n   inherit acls = yes\n   guest ok = yes\n",
		"[everyone]\n   path = /srv/shares/everyone\n   browseable = yes\n   read only = no\n   inherit acls = yes\n   valid users = @nos-users\n",
	} {
		if !strings.Contains(smb, want) {
			t.Errorf("smb include lacks %q:\n%s", want, smb)
		}
	}
	if strings.Contains(smb, "[nfsonly]") {
		t.Error("an NFS-only share must not appear in the Samba file")
	}
	// carol has no account yet: skipped, with a warning.
	if strings.Contains(smb, "carol") || !slices.ContainsFunc(resp.Warnings, func(w string) bool { return strings.Contains(w, "carol") }) {
		t.Errorf("unknown user must be skipped with a warning: %v\n%s", resp.Warnings, smb)
	}

	exports := h.files[nfsExportsPath]
	for _, want := range []string{
		"\"/srv/shares/media\" 10.0.0.0/8(ro,sync,no_subtree_check,root_squash)\n",
		"\"/mnt/pool/nfsonly\" 192.168.1.0/24(rw,sync,no_subtree_check,root_squash) host.lan(rw,sync,no_subtree_check,root_squash)\n",
	} {
		if !strings.Contains(exports, want) {
			t.Errorf("exports lacks %q:\n%s", want, exports)
		}
	}
	if strings.Contains(exports, "public") {
		t.Error("an SMB-only share must not be exported over NFS")
	}

	// Groups: nos-users with every user that has an account, nos-family.
	if got := h.ranLike("gpasswd"); !slices.Contains(got, "gpasswd -M alice,bob nos-users") || !slices.Contains(got, "gpasswd -M bob nos-family") {
		t.Errorf("group sync: %v", got)
	}

	// /srv/shares folders get their ACL replaced; others only get grants added.
	acls := h.ranLike("setfacl")
	if !slices.ContainsFunc(acls, func(r string) bool {
		return strings.HasPrefix(r, "setfacl --set u::rwx,g::rwx,o::---,u:nos:rwx,u:alice:rwx,g:nos-family:r-x,m::rwx,d:u::rwx,") && strings.HasSuffix(r, " /srv/shares/media")
	}) {
		t.Errorf("media ACL: %v", acls)
	}
	if !slices.ContainsFunc(acls, func(r string) bool {
		return strings.Contains(r, "u:nobody:rwx") && strings.HasSuffix(r, "/srv/shares/public")
	}) {
		t.Errorf("guest share must grant nobody: %v", acls)
	}
	if !slices.ContainsFunc(acls, func(r string) bool {
		return strings.Contains(r, "g:nos-users:rwx") && strings.HasSuffix(r, "/srv/shares/everyone")
	}) {
		t.Errorf("open share must grant nos-users: %v", acls)
	}
	if !slices.ContainsFunc(acls, func(r string) bool {
		return strings.HasPrefix(r, "setfacl -m ") && !strings.Contains(r, "::") && strings.HasSuffix(r, "/mnt/pool/nfsonly")
	}) {
		t.Errorf("unmanaged path must only get grants added: %v", acls)
	}
	if !slices.Contains(h.ran, "systemctl reload smbd") || !slices.Contains(h.ran, "exportfs -ra") {
		t.Errorf("services not reloaded: %v", h.ran)
	}
}

func TestApplySharesNobodyResolvable(t *testing.T) {
	h := newFakeSharesHost()
	req := SharesApplyRequest{Shares: []applyShare{{Name: "x", Path: "/srv/shares/x", SMB: &applySMB{Enabled: true}, Owners: []string{"user:ghost"}}}}
	if _, err := applyShares(h, req); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.files[smbIncludePath], "[x]\n   path = /srv/shares/x\n   browseable = yes\n   read only = no\n   inherit acls = yes\n   available = no\n") {
		t.Errorf("a share nobody can use must be off, not open:\n%s", h.files[smbIncludePath])
	}
}

func TestValidateSharesApply(t *testing.T) {
	ok := applyShare{Name: "ok", Path: "/srv/shares/ok"}
	bad := []applyShare{
		{Name: "bad]\n[global", Path: "/srv/shares/x"},
		{Name: "x", Path: "/etc"},
		{Name: "x", Path: "/srv/shares/../../etc"},
		{Name: "x", Path: "/srv/shares/a\nb"},
		{Name: "x", Path: "/srv/shares/x", Description: "a\n   path = /"},
		{Name: "x", Path: "/srv/shares/x", Owners: []string{"user:root;"}},
		{Name: "x", Path: "/srv/shares/x", Owners: []string{"alice"}},
		{Name: "x", Path: "/srv/shares/x", NFS: &applyNFS{Enabled: true}},
		{Name: "x", Path: "/srv/shares/x", NFS: &applyNFS{Enabled: true, Networks: []string{"* (rw,no_root_squash)"}}},
		{Name: "x", Path: "/srv/shares/x", SMB: &applySMB{Enabled: true, Recycle: &applyRecyc{Enabled: true, Directory: ".."}}},
	}
	for _, s := range bad {
		req := SharesApplyRequest{Shares: []applyShare{ok, s}}
		if err := validateSharesApply(&req); err == nil {
			t.Errorf("accepted %+v", s)
		}
	}
	dup := SharesApplyRequest{Shares: []applyShare{ok, {Name: "OK", Path: "/srv/shares/ok2"}}}
	if validateSharesApply(&dup) == nil {
		t.Error("share names differing only in case must be refused (Samba ignores case)")
	}
	if validateSharesApply(&SharesApplyRequest{Groups: []applyGroup{{Name: "users"}}}) == nil {
		t.Error("group users is reserved")
	}
	if validateSharesApply(&SharesApplyRequest{Shares: []applyShare{{Name: "x", Path: "/srv/shares/x", Readers: []string{"group:users"}}}}) != nil {
		t.Error("group:users (every user) must be accepted in a share")
	}
}

func TestSharesApplyRoute(t *testing.T) {
	h := newFakeSharesHost()
	old := newSharesHost
	newSharesHost = func() sharesHost { return h }
	t.Cleanup(func() { newSharesHost = old })

	mux := buildMux()
	body, _ := json.Marshal(SharesApplyRequest{Shares: []applyShare{{Name: "a", Path: "/srv/shares/a", SMB: &applySMB{Enabled: true}}}})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/shares/apply", bytes.NewReader(body)))
	if rr.Code != http.StatusOK || !strings.Contains(h.files[smbIncludePath], "[a]") {
		t.Fatalf("apply: %d %s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/shares/apply", strings.NewReader(`{"shares":[{"name":"a","path":"/etc"}]}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad path: %d", rr.Code)
	}
}

// An app the admin opens a share to reads it through its own system group,
// separate from people; the app's install gets that group's GID.
func TestShareOpenedToAnApp(t *testing.T) {
	h := newFakeSharesHost()
	req := SharesApplyRequest{Users: []string{"alice"}, Shares: []applyShare{
		{Name: "media", Path: "/srv/shares/media", Owners: []string{"user:alice"}, Apps: []string{"nai"}},
		{Name: "private", Path: "/srv/shares/private", Owners: []string{"user:alice"}},
	}}
	if err := validateSharesApply(&req); err != nil {
		t.Fatal(err)
	}
	if _, err := applyShares(h, req); err != nil {
		t.Fatal(err)
	}
	if got := h.ranLike("groupadd --system nos-app-nai"); len(got) != 1 {
		t.Fatalf("app group not created as a system group: %v", h.ran)
	}
	var mediaACL, privateACL string
	for _, r := range h.ranLike("setfacl") {
		if strings.HasSuffix(r, "/srv/shares/media") {
			mediaACL = r
		}
		if strings.HasSuffix(r, "/srv/shares/private") {
			privateACL = r
		}
	}
	if !strings.Contains(mediaACL, "g:nos-app-nai:r-x") || !strings.Contains(mediaACL, "d:g:nos-app-nai:r-x") {
		t.Fatalf("media must be readable (and inherit) for the app: %s", mediaACL)
	}
	if strings.Contains(privateACL, "nos-app-nai") {
		t.Fatalf("a share not opened to the app must not list it: %s", privateACL)
	}

	// The install's group op returns the same group's GID.
	gid, err := ensureAppGroup(h.Run, "nai")
	if err != nil || gid != "997" {
		t.Fatalf("gid: %q %v", gid, err)
	}

	for _, bad := range []string{"NAI", "a-very-long-app-id-over-24", "../x"} {
		r := SharesApplyRequest{Shares: []applyShare{{Name: "m", Path: "/srv/shares/m", Apps: []string{bad}}}}
		if validateSharesApply(&r) == nil {
			t.Errorf("app id %q accepted", bad)
		}
	}
	r := SharesApplyRequest{Groups: []applyGroup{{Name: "app-nai"}}}
	if validateSharesApply(&r) == nil {
		t.Error("a people group may not take an app group's name")
	}
}
