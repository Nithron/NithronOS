package server

// POST /v1/shares/apply makes the host match nosd's share list: Unix groups
// for nOS groups, the share folders and their ACLs, one Samba include file and
// one NFS exports file, then reloads smbd and the NFS exports.
//
// nosd sends the whole desired state every time, so the files are rewritten
// from it and never drift. Everything is validated again here: nosd is the
// only caller, but these values end up in root-owned configuration.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const (
	smbIncludePath  = "/etc/samba/smb.conf.d/nos-shares.conf"
	nfsExportsPath  = "/etc/exports.d/nos.exports"
	managedShareDir = "/srv/shares"
	// unixGroupPrefix maps nOS group g to the Unix group nos-g.
	unixGroupPrefix = "nos-"
	// allUsersGroup holds every nOS user; group:users in a share means it.
	allUsersGroup = "nos-users"
	// nosdUser runs nosd, which serves NithronSync and WebDAV from the shares.
	nosdUser = "nos"
	// Principals outside [firstRegularUID, lastRegularUID] (root, daemons,
	// nobody) are never granted.
	firstRegularUID = 1000
	lastRegularUID  = 59999
)

var shareRoots = []string{"/srv", "/mnt"}

var (
	reShareName   = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)
	reAccountName = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
	reGroupName   = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,27}$`)
	reNFSClient   = regexp.MustCompile(`^[A-Za-z0-9.*?:_/\[\]-]{1,255}$`)
	reRecycleDir  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
)

type applySMB struct {
	Enabled     bool        `json:"enabled"`
	Guest       bool        `json:"guest"`
	TimeMachine bool        `json:"time_machine"`
	Recycle     *applyRecyc `json:"recycle,omitempty"`
}

type applyRecyc struct {
	Enabled   bool   `json:"enabled"`
	Directory string `json:"directory"`
}

type applyNFS struct {
	Enabled  bool     `json:"enabled"`
	Networks []string `json:"networks"`
	ReadOnly bool     `json:"read_only"`
}

type applyShare struct {
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Description string    `json:"description"`
	SMB         *applySMB `json:"smb,omitempty"`
	NFS         *applyNFS `json:"nfs,omitempty"`
	Owners      []string  `json:"owners"`
	Readers     []string  `json:"readers"`
	Apps        []string  `json:"apps"` // app ids that may read, through nos-app-<id>
}

type applyGroup struct {
	Name    string   `json:"name"`
	Members []string `json:"members"`
}

// SharesApplyRequest is the desired state.
type SharesApplyRequest struct {
	Shares []applyShare `json:"shares"`
	Groups []applyGroup `json:"groups"`
	Users  []string     `json:"users"` // every nOS user
}

// SharesApplyResponse lists what couldn't be done; the rest was applied.
type SharesApplyResponse struct {
	OK       bool     `json:"ok"`
	Warnings []string `json:"warnings"`
}

func (s applyShare) smbOn() bool { return s.SMB != nil && s.SMB.Enabled }
func (s applyShare) nfsOn() bool { return s.NFS != nil && s.NFS.Enabled }

func validateSharesApply(req *SharesApplyRequest) error {
	names := map[string]bool{}
	for _, s := range req.Shares {
		if !reShareName.MatchString(s.Name) {
			return fmt.Errorf("share name %q", s.Name)
		}
		if names[strings.ToLower(s.Name)] {
			return fmt.Errorf("share name %q used twice", s.Name)
		}
		names[strings.ToLower(s.Name)] = true
		if err := validateSharePath(s.Path); err != nil {
			return fmt.Errorf("share %s: %w", s.Name, err)
		}
		if len(s.Description) > 256 || strings.IndexFunc(s.Description, unicode.IsControl) >= 0 {
			return fmt.Errorf("share %s: bad description", s.Name)
		}
		for _, p := range append(slices.Clone(s.Owners), s.Readers...) {
			if _, _, err := parsePrincipal(p); err != nil {
				return fmt.Errorf("share %s: %w", s.Name, err)
			}
		}
		for _, id := range s.Apps {
			if err := validAppGroupID(id); err != nil {
				return fmt.Errorf("share %s: %w", s.Name, err)
			}
		}
		if s.SMB != nil && s.SMB.Recycle != nil && s.SMB.Recycle.Enabled && !validRecycleDir(s.SMB.Recycle.Directory) {
			return fmt.Errorf("share %s: bad recycle directory", s.Name)
		}
		if s.nfsOn() {
			if len(s.NFS.Networks) == 0 {
				return fmt.Errorf("share %s: NFS needs at least one client network", s.Name)
			}
			for _, c := range s.NFS.Networks {
				if !reNFSClient.MatchString(c) {
					return fmt.Errorf("share %s: bad NFS client %q", s.Name, c)
				}
			}
		}
	}
	for _, g := range req.Groups {
		if !reGroupName.MatchString(g.Name) || g.Name == "users" || strings.HasPrefix(g.Name, "app-") {
			return fmt.Errorf("group name %q", g.Name)
		}
		for _, m := range g.Members {
			if !reAccountName.MatchString(m) {
				return fmt.Errorf("group %s: member %q", g.Name, m)
			}
		}
	}
	for _, u := range req.Users {
		if !reAccountName.MatchString(u) {
			return fmt.Errorf("user %q", u)
		}
	}
	return nil
}

func validRecycleDir(d string) bool {
	return d == "" || (reRecycleDir.MatchString(d) && d != "." && d != "..")
}

// validateSharePath accepts a clean absolute path below /srv or /mnt without
// characters that would end a line or a quoted value in smb.conf or exports.
func validateSharePath(p string) error {
	if !filepath.IsAbs(p) || filepath.Clean(p) != p {
		return errors.New("path must be absolute and clean")
	}
	if strings.ContainsAny(p, "\"\\") || strings.IndexFunc(p, unicode.IsControl) >= 0 {
		return errors.New("path contains a forbidden character")
	}
	for _, root := range shareRoots {
		if strings.HasPrefix(p, root+"/") {
			return nil
		}
	}
	return fmt.Errorf("path must be inside %s", strings.Join(shareRoots, " or "))
}

func parsePrincipal(p string) (kind, name string, err error) {
	switch {
	case strings.HasPrefix(p, "user:"):
		kind, name = "user", strings.TrimPrefix(p, "user:")
		if reAccountName.MatchString(name) {
			return kind, name, nil
		}
	case strings.HasPrefix(p, "group:"):
		// group:users is every nOS user (nos-users).
		kind, name = "group", strings.TrimPrefix(p, "group:")
		if reGroupName.MatchString(name) {
			return kind, name, nil
		}
	}
	return "", "", fmt.Errorf("principal %q", p)
}

// hostAccounts answers which principals exist on the host. Only regular
// accounts count: a share can never be granted to root or a daemon.
type hostAccounts interface {
	UserOK(name string) bool
	GroupExists(name string) bool
}

// granted resolves principals to setfacl/Samba names: users as-is, nOS
// groups as nos-<name>. Unknown users are skipped with a warning.
func granted(principals []string, acc hostAccounts, warn func(string, ...any)) (users, groups []string) {
	for _, p := range principals {
		kind, name, err := parsePrincipal(p)
		if err != nil {
			continue
		}
		if kind == "group" {
			if !acc.GroupExists(unixGroupPrefix + name) {
				warn("group %s does not exist; skipped", name)
				continue
			}
			groups = append(groups, unixGroupPrefix+name)
			continue
		}
		if !acc.UserOK(name) {
			warn("user %s has no SMB/Unix account yet; skipped", name)
			continue
		}
		users = append(users, name)
	}
	return users, groups
}

// renderSMB returns the Samba include file for the SMB-enabled shares.
func renderSMB(shares []applyShare, acc hostAccounts, warn func(string, ...any)) string {
	var b strings.Builder
	b.WriteString("# Managed by nOS (nos-agent). Changes here are overwritten.\n")
	for _, s := range shares {
		if !s.smbOn() {
			continue
		}
		fmt.Fprintf(&b, "\n[%s]\n", s.Name)
		fmt.Fprintf(&b, "   path = %s\n", s.Path)
		if s.Description != "" {
			fmt.Fprintf(&b, "   comment = %s\n", s.Description)
		}
		b.WriteString("   browseable = yes\n   read only = no\n   inherit acls = yes\n")
		switch {
		case s.SMB.Guest:
			b.WriteString("   guest ok = yes\n")
		default:
			ou, og := granted(s.Owners, acc, warn)
			ru, rg := granted(s.Readers, acc, warn)
			valid := append(append(append(append([]string{}, ou...), atGroups(og)...), ru...), atGroups(rg)...)
			if len(valid) == 0 {
				// Nobody resolvable: deny rather than fall back to everyone.
				warn("share %s: no owner or reader has an account; SMB access is off", s.Name)
				b.WriteString("   available = no\n")
				continue
			}
			fmt.Fprintf(&b, "   valid users = %s\n", strings.Join(valid, " "))
			if readers := append(append([]string{}, ru...), atGroups(rg)...); len(readers) > 0 {
				fmt.Fprintf(&b, "   read list = %s\n", strings.Join(readers, " "))
			}
		}
		var vfs []string
		if s.SMB.TimeMachine {
			vfs = append(vfs, "catia", "fruit", "streams_xattr")
		}
		if s.SMB.Recycle != nil && s.SMB.Recycle.Enabled {
			vfs = append(vfs, "recycle")
		}
		if len(vfs) > 0 {
			fmt.Fprintf(&b, "   vfs objects = %s\n", strings.Join(vfs, " "))
		}
		if s.SMB.TimeMachine {
			b.WriteString("   fruit:time machine = yes\n")
		}
		if s.SMB.Recycle != nil && s.SMB.Recycle.Enabled {
			dir := s.SMB.Recycle.Directory
			if dir == "" {
				dir = ".recycle"
			}
			fmt.Fprintf(&b, "   recycle:repository = %s\n   recycle:keeptree = yes\n   recycle:versions = yes\n   recycle:touch = yes\n", dir)
		}
	}
	return b.String()
}

func atGroups(groups []string) []string {
	out := make([]string, len(groups))
	for i, g := range groups {
		out[i] = "@" + g
	}
	return out
}

// renderExports returns the exports(5) file for the NFS-enabled shares.
func renderExports(shares []applyShare) string {
	var b strings.Builder
	b.WriteString("# Managed by nOS (nos-agent). Changes here are overwritten.\n")
	for _, s := range shares {
		if !s.nfsOn() {
			continue
		}
		mode := "rw"
		if s.NFS.ReadOnly {
			mode = "ro"
		}
		fmt.Fprintf(&b, "%q", s.Path)
		for _, c := range s.NFS.Networks {
			fmt.Fprintf(&b, " %s(%s,sync,no_subtree_check,root_squash)", c, mode)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// shareACL is the setfacl --set spec for a share folder: root and nosd (which
// serves NithronSync and WebDAV) always, owners rwx, readers r-x, guests as
// nobody, and nobody else. The same entries become the defaults for new files.
func shareACL(s applyShare, acc hostAccounts, warn func(string, ...any)) string {
	entries := []string{"u::rwx", "g::rwx", "o::---", "u:" + nosdUser + ":rwx"}
	add := func(kind, name, perm string) { entries = append(entries, kind+":"+name+":"+perm) }
	ou, og := granted(s.Owners, acc, func(string, ...any) {})
	ru, rg := granted(s.Readers, acc, func(string, ...any) {})
	for _, u := range ou {
		add("u", u, "rwx")
	}
	for _, g := range og {
		add("g", g, "rwx")
	}
	for _, u := range ru {
		if !slices.Contains(ou, u) {
			add("u", u, "r-x")
		}
	}
	for _, g := range rg {
		if !slices.Contains(og, g) {
			add("g", g, "r-x")
		}
	}
	for _, id := range s.Apps {
		if g := appGroupName(id); acc.GroupExists(g) {
			add("g", g, "r-x")
		}
	}
	if s.SMB != nil && s.SMB.Enabled && s.SMB.Guest {
		add("u", "nobody", "rwx")
	}
	entries = append(entries, "m::rwx")
	all := slices.Clone(entries)
	for _, e := range entries {
		all = append(all, "d:"+e)
	}
	return strings.Join(all, ",")
}

// sharesHost does the system work; tests replace it.
type sharesHost interface {
	hostAccounts
	Run(name string, args ...string) ([]byte, error)
	WriteFile(path string, data []byte) error
	EnsureDir(path string) (created bool, err error)
	Active(unit string) bool
}

type realSharesHost struct{}

func (realSharesHost) UserOK(name string) bool {
	u, err := user.Lookup(name)
	if err != nil {
		return false
	}
	uid, err := strconv.Atoi(u.Uid)
	return err == nil && uid >= firstRegularUID && uid <= lastRegularUID
}

func (realSharesHost) GroupExists(name string) bool {
	_, err := user.LookupGroup(name)
	return err == nil
}

func (realSharesHost) Run(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

func (realSharesHost) WriteFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".nos-tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// EnsureDir creates a missing share folder: a Btrfs subvolume when the parent
// is on Btrfs (so it can be snapshotted on its own), a directory otherwise.
// An existing path must be a real directory, not a symlink.
func (h realSharesHost) EnsureDir(path string) (bool, error) {
	if fi, err := os.Lstat(path); err == nil {
		if !fi.IsDir() {
			return false, fmt.Errorf("%s exists and is not a directory", path)
		}
		if real, err := filepath.EvalSymlinks(path); err != nil || real != path {
			return false, fmt.Errorf("%s goes through a symlink", path)
		}
		return false, nil
	}
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return false, err
	}
	if out, err := exec.Command("stat", "-f", "-c", "%T", parent).Output(); err == nil && strings.TrimSpace(string(out)) == "btrfs" {
		out, err := exec.Command("btrfs", "subvolume", "create", path).CombinedOutput()
		if err == nil {
			return true, nil
		}
		logAuthPriv("shares: btrfs subvolume create " + path + ": " + strings.TrimSpace(string(out)))
	}
	return true, os.Mkdir(path, 0o770)
}

func (h realSharesHost) Active(unit string) bool {
	return exec.Command("systemctl", "is-active", "--quiet", unit).Run() == nil
}

var newSharesHost = func() sharesHost { return realSharesHost{} }

func handleSharesApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req SharesApplyRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validateSharesApply(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := applyShares(newSharesHost(), req)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func applyShares(h sharesHost, req SharesApplyRequest) (SharesApplyResponse, error) {
	resp := SharesApplyResponse{OK: true, Warnings: []string{}}
	warn := func(f string, a ...any) { resp.Warnings = append(resp.Warnings, fmt.Sprintf(f, a...)) }

	syncGroups(h, req, warn)

	shares := slices.Clone(req.Shares)
	sort.Slice(shares, func(i, j int) bool { return shares[i].Name < shares[j].Name })
	for _, s := range shares {
		applyShareFolder(h, s, warn)
	}

	smb := renderSMB(shares, h, warn)
	if err := checkSMB(h, smb); err != nil {
		return resp, err
	}
	if err := h.WriteFile(smbIncludePath, []byte(smb)); err != nil {
		return resp, fmt.Errorf("write %s: %w", smbIncludePath, err)
	}
	if err := h.WriteFile(nfsExportsPath, []byte(renderExports(shares))); err != nil {
		return resp, fmt.Errorf("write %s: %w", nfsExportsPath, err)
	}

	if h.Active("smbd") {
		if out, err := h.Run("systemctl", "reload", "smbd"); err != nil {
			warn("reload smbd: %s", strings.TrimSpace(string(out)))
		}
	} else {
		warn("smbd is not running; SMB shares apply when it starts")
	}
	if h.Active("nfs-server") {
		if out, err := h.Run("exportfs", "-ra"); err != nil {
			warn("exportfs -ra: %s", strings.TrimSpace(string(out)))
		}
	} else if slices.ContainsFunc(shares, applyShare.nfsOn) {
		warn("nfs-server is not running; NFS shares apply when it starts")
	}
	logAuthPriv(fmt.Sprintf("shares.apply shares=%d groups=%d warnings=%d", len(shares), len(req.Groups), len(resp.Warnings)))
	return resp, nil
}

// checkSMB runs testparm on the rendered file before it replaces the old one.
func checkSMB(h sharesHost, smb string) error {
	tmp, err := os.CreateTemp("", "nos-smb-*.conf")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(smb); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()
	out, err := h.Run("testparm", "-s", "--suppress-prompt", tmp.Name())
	if err != nil {
		var ee *exec.Error
		if errors.As(err, &ee) {
			return nil // samba not installed: nothing will read the file
		}
		return fmt.Errorf("samba rejected the share configuration: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// syncGroups creates nos-<name> for every nOS group and sets its members to
// the group's users that have accounts; nos-users gets every such user.
func syncGroups(h sharesHost, req SharesApplyRequest, warn func(string, ...any)) {
	set := func(group string, members []string) {
		if !h.GroupExists(group) {
			if out, err := h.Run("groupadd", group); err != nil {
				warn("groupadd %s: %s", group, strings.TrimSpace(string(out)))
				return
			}
		}
		var ok []string
		for _, m := range members {
			if h.UserOK(m) {
				ok = append(ok, m)
			}
		}
		sort.Strings(ok)
		if out, err := h.Run("gpasswd", "-M", strings.Join(ok, ","), group); err != nil {
			warn("gpasswd %s: %s", group, strings.TrimSpace(string(out)))
		}
	}
	set(allUsersGroup, req.Users)
	for _, g := range req.Groups {
		set(unixGroupPrefix+g.Name, g.Members)
	}
	// Apps read through their own system group, which has no members.
	seen := map[string]bool{}
	for _, s := range req.Shares {
		for _, id := range s.Apps {
			if seen[id] {
				continue
			}
			seen[id] = true
			if _, err := ensureAppGroup(h.Run, id); err != nil {
				warn("app %s: %v", id, err)
			}
		}
	}
}

// appGroupPrefix names the group an app reads shares through: nos-app-<id>.
const appGroupPrefix = "nos-app-"

// maxAppGroupID keeps nos-app-<id> within Unix's 32-character group names.
const maxAppGroupID = 32 - len(appGroupPrefix)

func appGroupName(appID string) string { return appGroupPrefix + appID }

func validAppGroupID(id string) error {
	if !reAppID.MatchString(id) || len(id) > maxAppGroupID {
		return fmt.Errorf("app id %q (lowercase letters, digits and '-', at most %d)", id, maxAppGroupID)
	}
	return nil
}

// ensureAppGroup creates the app's system group if it is missing and returns
// its GID. The group has no members: the app's containers join it with
// group_add, so a share opened to the app is readable by them and nobody else.
func ensureAppGroup(run func(string, ...string) ([]byte, error), appID string) (string, error) {
	if err := validAppGroupID(appID); err != nil {
		return "", err
	}
	name := appGroupName(appID)
	out, err := run("getent", "group", name)
	if err != nil {
		if out, err := run("groupadd", "--system", name); err != nil {
			return "", fmt.Errorf("groupadd %s: %s", name, strings.TrimSpace(string(out)))
		}
		if out, err = run("getent", "group", name); err != nil {
			return "", fmt.Errorf("getent %s: %s", name, strings.TrimSpace(string(out)))
		}
	}
	// name:x:gid:members
	fields := strings.Split(strings.TrimSpace(string(out)), ":")
	if len(fields) < 3 {
		return "", fmt.Errorf("getent %s: unexpected %q", name, strings.TrimSpace(string(out)))
	}
	if _, err := strconv.Atoi(fields[2]); err != nil {
		return "", fmt.Errorf("getent %s: unexpected gid %q", name, fields[2])
	}
	return fields[2], nil
}

// applyShareFolder creates the folder and sets its permissions. Folders under
// /srv/shares are nOS's: their ACL is replaced. Elsewhere (shares from older
// versions, pool paths) the grants are only added, nothing is taken away.
func applyShareFolder(h sharesHost, s applyShare, warn func(string, ...any)) {
	managed := strings.HasPrefix(s.Path, managedShareDir+"/")
	if _, err := h.EnsureDir(s.Path); err != nil {
		warn("share %s: %v", s.Name, err)
		return
	}
	acl := shareACL(s, h, warn)
	if managed {
		if out, err := h.Run("chmod", "2770", s.Path); err != nil {
			warn("share %s: chmod: %s", s.Name, strings.TrimSpace(string(out)))
		}
		if out, err := h.Run("setfacl", "--set", acl, s.Path); err != nil {
			warn("share %s: setfacl: %s", s.Name, strings.TrimSpace(string(out)))
		}
		return
	}
	var add []string
	for _, e := range strings.Split(acl, ",") {
		if !strings.Contains(e, "::") { // keep the owner/group/other/mask bits
			add = append(add, e)
		}
	}
	if out, err := h.Run("setfacl", "-m", strings.Join(add, ","), s.Path); err != nil {
		warn("share %s: setfacl: %s", s.Name, strings.TrimSpace(string(out)))
	}
}
