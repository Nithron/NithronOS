package server

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Pool create and destroy as one operation each: nosd sends what the pool
// should be, the agent checks it and runs the commands itself (no shell, no
// command lines from nosd). nosd used to send back the shell-quoted plan it
// had shown, which the agent's command check refused, and wrote a literal
// "UUID=<uuid>" line into /etc/fstab.

type poolCreateRequest struct {
	Name         string   `json:"name"`
	Devices      []string `json:"devices"`
	RaidData     string   `json:"raid_data"`
	RaidMeta     string   `json:"raid_meta"`
	Mountpoint   string   `json:"mountpoint"`
	MountOptions string   `json:"mount_options"`
	// Keyfile, when set, puts each device under LUKS2 opened with this key
	// (made if missing).
	Keyfile string `json:"keyfile,omitempty"`
	// Force wipes devices that have a filesystem or partition table.
	Force bool `json:"force"`
	// DryRun checks the request and lists the steps without running them.
	DryRun bool `json:"dry_run"`
}

type poolDestroyRequest struct {
	Mountpoint string `json:"mountpoint"`
	// Wipe removes the filesystem signatures from the pool's devices.
	Wipe   bool `json:"wipe"`
	DryRun bool `json:"dry_run"`
}

// poolStep is one command of an operation, as planned or as run.
type poolStep struct {
	ID          string `json:"id"`
	Command     string `json:"command"`
	Destructive bool   `json:"destructive,omitempty"`
	Status      string `json:"status"` // planned, ok, error
	Output      string `json:"output,omitempty"`
}

type poolOpResponse struct {
	OK         bool       `json:"ok"`
	Error      string     `json:"error,omitempty"`
	Warnings   []string   `json:"warnings,omitempty"`
	UUID       string     `json:"uuid,omitempty"`
	Mountpoint string     `json:"mountpoint"`
	Fstab      string     `json:"fstab,omitempty"`
	Devices    []string   `json:"devices,omitempty"`
	Steps      []poolStep `json:"steps"`
}

const (
	defaultPoolMountOptions = "noatime,compress=zstd:3"
	poolKeyDir              = "/etc/nos/keys"
	poolOpTimeout           = 10 * time.Minute
	maxPoolDevices          = 16
)

var (
	poolNameRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)
	mountOptionsRe = regexp.MustCompile(`^[A-Za-z0-9=:,_.+-]{1,256}$`)
	// minimum devices per btrfs profile
	poolProfiles = map[string]int{"single": 1, "raid0": 2, "raid1": 2, "raid10": 4}
)

// poolCommand runs one command; tests replace it.
var poolCommand = func(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin", "LANG=C", "LC_ALL=C"}
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// poolDeviceCheck accepts block devices; tests replace it.
var poolDeviceCheck = isBlockDevice

// mountsPath is /proc/self/mountinfo; tests point it at a file.
var mountsPath = "/proc/self/mountinfo"

type mountEntry struct{ target, fstype, source string }

func readMounts() ([]mountEntry, error) {
	f, err := os.Open(mountsPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var out []mountEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		// id parent major:minor root target opts [optional...] - fstype source superopts
		left, right, ok := strings.Cut(sc.Text(), " - ")
		if !ok {
			continue
		}
		l, r := strings.Fields(left), strings.Fields(right)
		if len(l) < 5 || len(r) < 2 {
			continue
		}
		out = append(out, mountEntry{target: unescapeMount(l[4]), fstype: r[0], source: unescapeMount(r[1])})
	}
	return out, sc.Err()
}

func unescapeMount(s string) string {
	return strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`).Replace(s)
}

// validPoolMountpoint: a clean path one or more levels below /srv or /mnt.
func validPoolMountpoint(p string) bool {
	return p != "" && filepath.Clean(p) == p && isAllowedMountPath(p) && p != "/srv" && p != "/mnt"
}

func (req *poolCreateRequest) validate() error {
	if !poolNameRe.MatchString(req.Name) {
		return errors.New("name: lowercase letters, digits, - and _, at most 32")
	}
	if len(req.Devices) == 0 || len(req.Devices) > maxPoolDevices {
		return fmt.Errorf("between 1 and %d devices", maxPoolDevices)
	}
	seen := map[string]bool{}
	for _, d := range req.Devices {
		if !validDevice(d) || filepath.Clean(d) != d || !poolDeviceCheck(d) {
			return fmt.Errorf("%s is not a block device", d)
		}
		if seen[d] {
			return fmt.Errorf("%s listed twice", d)
		}
		seen[d] = true
	}
	if req.RaidData == "" {
		req.RaidData = defaultProfile(len(req.Devices))
	}
	if req.RaidMeta == "" {
		req.RaidMeta = defaultProfile(len(req.Devices))
	}
	for _, p := range []string{req.RaidData, req.RaidMeta} {
		min, ok := poolProfiles[p]
		if !ok {
			return fmt.Errorf("profile %q not supported (single, raid0, raid1, raid10)", p)
		}
		if len(req.Devices) < min {
			return fmt.Errorf("%s needs at least %d devices", p, min)
		}
	}
	if !validPoolMountpoint(req.Mountpoint) {
		return errors.New("mountpoint: a clean path below /srv or /mnt")
	}
	if req.MountOptions == "" {
		req.MountOptions = defaultPoolMountOptions
	}
	if !mountOptionsRe.MatchString(req.MountOptions) {
		return errors.New("mount options: letters, digits and = : , _ . + -")
	}
	if req.Keyfile != "" && (filepath.Dir(req.Keyfile) != poolKeyDir || filepath.Clean(req.Keyfile) != req.Keyfile ||
		!poolNameRe.MatchString(strings.TrimSuffix(filepath.Base(req.Keyfile), ".key")) || !strings.HasSuffix(req.Keyfile, ".key")) {
		return fmt.Errorf("keyfile: %s/<name>.key", poolKeyDir)
	}
	return nil
}

func defaultProfile(devices int) string {
	if devices >= 2 {
		return "raid1"
	}
	return "single"
}

// checkCreateTargets refuses devices in use and a mountpoint in use.
func checkCreateTargets(req poolCreateRequest) error {
	mounts, err := readMounts()
	if err != nil {
		return fmt.Errorf("read mounts: %w", err)
	}
	for _, m := range mounts {
		if m.target == req.Mountpoint {
			return fmt.Errorf("%s is already mounted", req.Mountpoint)
		}
		for _, d := range req.Devices {
			// the device or one of its partitions (sda1, nvme0n1p1)
			if m.source == d || (strings.HasPrefix(m.source, d) && strings.Trim(strings.TrimPrefix(m.source, d), "p0123456789") == "") {
				return fmt.Errorf("%s is in use (mounted at %s)", d, m.target)
			}
		}
	}
	if ents, err := os.ReadDir(req.Mountpoint); err == nil && len(ents) > 0 {
		return fmt.Errorf("%s is not empty", req.Mountpoint)
	}
	return nil
}

// stepRunner runs and records the steps of one operation.
type stepRunner struct {
	ctx    context.Context
	dryRun bool
	steps  []poolStep
}

func (s *stepRunner) run(id string, destructive bool, name string, args ...string) (string, error) {
	st := poolStep{ID: id, Command: strings.Join(quoteAll(append([]string{name}, args...)), " "), Destructive: destructive, Status: "planned"}
	if s.dryRun {
		s.steps = append(s.steps, st)
		return "", nil
	}
	out, err := poolCommand(s.ctx, name, args...)
	st.Output = truncate(out, 4096)
	st.Status = "ok"
	if err != nil {
		st.Status = "error"
		err = fmt.Errorf("%s: %v: %s", id, err, truncate(out, 512))
	}
	s.steps = append(s.steps, st)
	return out, err
}

// note records a step done in Go (files), for the plan and the log.
func (s *stepRunner) note(id, what string, fn func() error) error {
	st := poolStep{ID: id, Command: what, Status: "planned"}
	if !s.dryRun {
		st.Status = "ok"
		if err := fn(); err != nil {
			st.Status = "error"
			st.Output = err.Error()
			s.steps = append(s.steps, st)
			return fmt.Errorf("%s: %w", id, err)
		}
	}
	s.steps = append(s.steps, st)
	return nil
}

func handlePoolCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req poolCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := req.validate(); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := checkCreateTargets(req); err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), poolOpTimeout)
	defer cancel()
	resp := createPool(ctx, req)
	writeJSON(w, http.StatusOK, resp)
}

func createPool(ctx context.Context, req poolCreateRequest) poolOpResponse {
	s := &stepRunner{ctx: ctx, dryRun: req.DryRun}
	resp := poolOpResponse{Mountpoint: req.Mountpoint}

	// Devices that already hold something are only used with force.
	for i, d := range req.Devices {
		out, err := poolCommand(ctx, "wipefs", "-n", d)
		if err != nil {
			resp.Error = fmt.Sprintf("wipefs -n %s: %v: %s", d, err, out)
			return resp
		}
		if out == "" {
			continue
		}
		resp.Warnings = append(resp.Warnings, fmt.Sprintf("%s has a filesystem or partition table", d))
		if req.Force {
			if _, err := s.run(fmt.Sprintf("wipe-%d", i+1), true, "wipefs", "-a", d); err != nil {
				return fail(resp, s, err)
			}
		}
	}
	if len(resp.Warnings) > 0 && !req.Force {
		resp.Error = "devices have existing signatures; set force to erase them"
		resp.Steps = s.steps
		return resp
	}

	targets := req.Devices
	var opened []string
	cleanup := func() {}
	if req.Keyfile != "" {
		if err := s.note("key", "make the key "+req.Keyfile+" (0400) if missing", func() error { return makePoolKey(req.Keyfile) }); err != nil {
			return fail(resp, s, err)
		}
		targets = nil
		for i, d := range req.Devices {
			name := fmt.Sprintf("luks-%s-%d", req.Name, i)
			if _, err := s.run(fmt.Sprintf("luks-format-%d", i+1), true, "cryptsetup", "luksFormat", "--type", "luks2", "--batch-mode", "--key-file", req.Keyfile, d); err != nil {
				closeLUKS(ctx, opened)
				return fail(resp, s, err)
			}
			if _, err := s.run(fmt.Sprintf("luks-open-%d", i+1), false, "cryptsetup", "open", "--key-file", req.Keyfile, d, name); err != nil {
				closeLUKS(ctx, opened)
				return fail(resp, s, err)
			}
			opened = append(opened, name)
			targets = append(targets, "/dev/mapper/"+name)
		}
		cleanup = func() { closeLUKS(ctx, opened) }
	}

	mkfs := []string{"-L", req.Name, "-d", req.RaidData, "-m", req.RaidMeta}
	if req.Force {
		mkfs = append([]string{"-f"}, mkfs...)
	}
	if _, err := s.run("mkfs", true, "mkfs.btrfs", append(mkfs, targets...)...); err != nil {
		cleanup()
		return fail(resp, s, err)
	}
	// Multi-device pools mount once the kernel knows all members.
	if _, err := s.run("scan", false, "btrfs", "device", "scan"); err != nil {
		cleanup()
		return fail(resp, s, err)
	}
	if err := s.note("mkdir", "mkdir -p "+req.Mountpoint, func() error { return makeMountpoint(req.Mountpoint) }); err != nil {
		cleanup()
		return fail(resp, s, err)
	}
	if _, err := s.run("mount", false, "mount", "-t", "btrfs", "-o", req.MountOptions, targets[0], req.Mountpoint); err != nil {
		cleanup()
		return fail(resp, s, err)
	}
	unmount := func() {
		_, _ = poolCommand(ctx, "umount", req.Mountpoint)
		cleanup()
	}
	for _, sv := range []string{"data", "snaps", "apps"} {
		if _, err := s.run("subvol-"+sv, false, "btrfs", "subvolume", "create", filepath.Join(req.Mountpoint, sv)); err != nil {
			unmount()
			return fail(resp, s, err)
		}
	}
	uuid, err := s.run("uuid", false, "blkid", "-s", "UUID", "-o", "value", targets[0])
	if err != nil {
		unmount()
		return fail(resp, s, err)
	}
	if req.DryRun {
		uuid = "<uuid>"
	}
	resp.UUID = strings.TrimSpace(uuid)
	// nofail: a missing disk must not stop the system from booting.
	resp.Fstab = fmt.Sprintf("UUID=%s %s btrfs %s,nofail 0 0", resp.UUID, req.Mountpoint, req.MountOptions)
	var crypttab []string
	for i, name := range opened {
		if req.DryRun {
			break
		}
		luksUUID, err := poolCommand(ctx, "blkid", "-s", "UUID", "-o", "value", req.Devices[i])
		if err != nil {
			unmount()
			return fail(resp, s, fmt.Errorf("luks uuid of %s: %w", req.Devices[i], err))
		}
		crypttab = append(crypttab, fmt.Sprintf("%s UUID=%s %s luks,nofail", name, strings.TrimSpace(luksUUID), req.Keyfile))
	}
	if err := s.note("fstab", "add to /etc/fstab: "+resp.Fstab, func() error {
		return editTab("fstab", func(lines []string) []string { return append(dropField(lines, 1, req.Mountpoint), resp.Fstab) })
	}); err != nil {
		unmount()
		return fail(resp, s, err)
	}
	if req.Keyfile != "" {
		if err := s.note("crypttab", "add the LUKS devices to /etc/crypttab", func() error {
			return editTab("crypttab", func(lines []string) []string {
				for _, name := range opened {
					lines = dropField(lines, 0, name)
				}
				return append(lines, crypttab...)
			})
		}); err != nil {
			_ = editTab("fstab", func(lines []string) []string { return dropField(lines, 1, req.Mountpoint) })
			unmount()
			return fail(resp, s, err)
		}
	}
	resp.OK = true
	resp.Devices = targets
	resp.Steps = s.steps
	return resp
}

func fail(resp poolOpResponse, s *stepRunner, err error) poolOpResponse {
	resp.OK = false
	resp.Error = err.Error()
	resp.Steps = s.steps
	return resp
}

func closeLUKS(ctx context.Context, names []string) {
	for _, n := range names {
		_, _ = poolCommand(ctx, "cryptsetup", "close", n)
	}
}

// makeMountpoint makes the pool's folder; tests replace it.
var makeMountpoint = func(p string) error { return os.MkdirAll(p, 0o755) }

// makePoolKey is ensurePoolKey; tests replace it.
var makePoolKey = ensurePoolKey

func ensurePoolKey(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	key := make([]byte, 64)
	if _, err := rand.Read(key); err != nil {
		return err
	}
	return os.WriteFile(path, key, 0o400)
}

func handlePoolDestroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req poolDestroyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !validPoolMountpoint(req.Mountpoint) {
		writeErr(w, http.StatusBadRequest, "mountpoint: a clean path below /srv or /mnt")
		return
	}
	mounts, err := readMounts()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	mounted := false
	for _, m := range mounts {
		if m.target == req.Mountpoint {
			if m.fstype != "btrfs" {
				writeErr(w, http.StatusBadRequest, req.Mountpoint+" is not a btrfs pool")
				return
			}
			mounted = true
		}
	}
	if !mounted {
		writeErr(w, http.StatusNotFound, req.Mountpoint+" is not mounted")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), poolOpTimeout)
	defer cancel()
	writeJSON(w, http.StatusOK, destroyPool(ctx, req))
}

func destroyPool(ctx context.Context, req poolDestroyRequest) poolOpResponse {
	s := &stepRunner{ctx: ctx, dryRun: req.DryRun}
	resp := poolOpResponse{Mountpoint: req.Mountpoint}
	show, err := poolCommand(ctx, "btrfs", "filesystem", "show", req.Mountpoint)
	if err != nil {
		resp.Error = fmt.Sprintf("btrfs filesystem show: %v: %s", err, show)
		return resp
	}
	members := parseShowDevices(show)
	// The disks under LUKS mappings, for wiping and crypttab.
	type luksMember struct{ name, backing string }
	var luks []luksMember
	var disks []string
	for _, m := range members {
		name, ok := strings.CutPrefix(m, "/dev/mapper/")
		if !ok || !strings.HasPrefix(name, "luks-") {
			disks = append(disks, m)
			continue
		}
		st, _ := poolCommand(ctx, "cryptsetup", "status", name)
		backing := ""
		for _, ln := range strings.Split(st, "\n") {
			if v, ok := strings.CutPrefix(strings.TrimSpace(ln), "device:"); ok {
				backing = strings.TrimSpace(v)
			}
		}
		luks = append(luks, luksMember{name, backing})
		if backing != "" {
			disks = append(disks, backing)
		}
	}
	resp.Devices = disks

	if _, err := s.run("umount", false, "umount", req.Mountpoint); err != nil {
		return fail(resp, s, err)
	}
	if err := s.note("fstab", "remove "+req.Mountpoint+" from /etc/fstab", func() error {
		return editTab("fstab", func(lines []string) []string { return dropField(lines, 1, req.Mountpoint) })
	}); err != nil {
		return fail(resp, s, err)
	}
	for i, l := range luks {
		if _, err := s.run(fmt.Sprintf("luks-close-%d", i+1), false, "cryptsetup", "close", l.name); err != nil {
			return fail(resp, s, err)
		}
		if err := s.note(fmt.Sprintf("crypttab-%d", i+1), "remove "+l.name+" from /etc/crypttab", func() error {
			return editTab("crypttab", func(lines []string) []string { return dropField(lines, 0, l.name) })
		}); err != nil {
			return fail(resp, s, err)
		}
	}
	if req.Wipe {
		for i, d := range disks {
			if _, err := s.run(fmt.Sprintf("wipe-%d", i+1), true, "wipefs", "-a", d); err != nil {
				return fail(resp, s, err)
			}
		}
	}
	// The mountpoint folder goes if it is empty (os.Remove leaves it otherwise).
	_ = s.note("rmdir", "remove the empty "+req.Mountpoint, func() error {
		_ = os.Remove(req.Mountpoint)
		return nil
	})
	resp.OK = true
	resp.Steps = s.steps
	return resp
}

// parseShowDevices reads the member paths from `btrfs filesystem show`.
func parseShowDevices(out string) []string {
	var devs []string
	for _, ln := range strings.Split(out, "\n") {
		f := strings.Fields(ln)
		if len(f) >= 2 && f[0] == "devid" && strings.HasPrefix(f[len(f)-1], "/dev/") {
			devs = append(devs, f[len(f)-1])
		}
	}
	return devs
}

// dropField removes the lines whose n-th field is value (comments kept).
func dropField(lines []string, n int, value string) []string {
	out := lines[:0:0]
	for _, ln := range lines {
		f := strings.Fields(ln)
		if len(f) > n && !strings.HasPrefix(f[0], "#") && f[n] == value {
			continue
		}
		out = append(out, ln)
	}
	return out
}

// editTab rewrites /etc/<name> (fstab, crypttab) atomically.
func editTab(name string, edit func([]string) []string) error {
	path := filepath.Join(etcDir, name)
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	lines = edit(lines)
	out := strings.Join(lines, "\n")
	if out != "" {
		out += "\n"
	}
	mode := os.FileMode(0o644)
	if name == "crypttab" {
		mode = 0o600
	}
	tmp := path + ".nos-tmp"
	if err := os.WriteFile(tmp, []byte(out), mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return fsyncDir(filepath.Dir(path))
}
