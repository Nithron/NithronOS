package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeHost replaces the commands, the block-device check, /proc mounts and
// /etc for one test.
type fakeHost struct {
	ran     []string
	outputs map[string]string // command prefix -> output
	fail    string            // command prefix that fails
	etc     string
}

func newFakeHost(t *testing.T, mounts string) *fakeHost {
	t.Helper()
	h := &fakeHost{outputs: map[string]string{}, etc: t.TempDir()}
	oldCmd, oldDev, oldMounts, oldEtc, oldMkdir := poolCommand, poolDeviceCheck, mountsPath, etcDir, makeMountpoint
	makeMountpoint = func(string) error { return nil }
	poolCommand = func(_ context.Context, name string, args ...string) (string, error) {
		line := strings.Join(append([]string{name}, args...), " ")
		h.ran = append(h.ran, line)
		if h.fail != "" && strings.HasPrefix(line, h.fail) {
			return "boom", errors.New("exit status 1")
		}
		for p, out := range h.outputs {
			if strings.HasPrefix(line, p) {
				return out, nil
			}
		}
		return "", nil
	}
	poolDeviceCheck = func(p string) bool { return strings.HasPrefix(p, "/dev/sd") || strings.HasPrefix(p, "/dev/loop") }
	mountsPath = filepath.Join(h.etc, "mountinfo")
	if err := os.WriteFile(mountsPath, []byte(mounts), 0o644); err != nil {
		t.Fatal(err)
	}
	etcDir = h.etc
	t.Cleanup(func() {
		poolCommand, poolDeviceCheck, mountsPath, etcDir, makeMountpoint = oldCmd, oldDev, oldMounts, oldEtc, oldMkdir
	})
	return h
}

const rootMount = "22 1 8:1 / / rw,relatime shared:1 - ext4 /dev/sda1 rw\n"

func (h *fakeHost) ranPrefix(p string) bool {
	for _, l := range h.ran {
		if strings.HasPrefix(l, p) {
			return true
		}
	}
	return false
}

func TestPoolCreateValidates(t *testing.T) {
	newFakeHost(t, rootMount)
	ok := poolCreateRequest{Name: "p1", Devices: []string{"/dev/sdb", "/dev/sdc"}, Mountpoint: "/srv/pool/p1"}
	if err := ok.validate(); err != nil || ok.RaidData != "raid1" || ok.MountOptions != defaultPoolMountOptions {
		t.Fatalf("valid request: %v %+v", err, ok)
	}
	for name, req := range map[string]poolCreateRequest{
		"name":           {Name: "../x", Devices: []string{"/dev/sdb"}, Mountpoint: "/srv/pool/x"},
		"not a device":   {Name: "p", Devices: []string{"/etc/passwd"}, Mountpoint: "/srv/pool/p"},
		"twice":          {Name: "p", Devices: []string{"/dev/sdb", "/dev/sdb"}, Mountpoint: "/srv/pool/p"},
		"raid1 one disk": {Name: "p", Devices: []string{"/dev/sdb"}, RaidData: "raid1", Mountpoint: "/srv/pool/p"},
		"raid5":          {Name: "p", Devices: []string{"/dev/sdb", "/dev/sdc", "/dev/sdd"}, RaidData: "raid5", Mountpoint: "/srv/pool/p"},
		"system folder":  {Name: "p", Devices: []string{"/dev/sdb"}, Mountpoint: "/etc"},
		"unclean":        {Name: "p", Devices: []string{"/dev/sdb"}, Mountpoint: "/srv/../etc"},
		"srv itself":     {Name: "p", Devices: []string{"/dev/sdb"}, Mountpoint: "/srv"},
		"options":        {Name: "p", Devices: []string{"/dev/sdb"}, Mountpoint: "/srv/pool/p", MountOptions: "noatime /etc"},
		"keyfile":        {Name: "p", Devices: []string{"/dev/sdb"}, Mountpoint: "/srv/pool/p", Keyfile: "/root/k.key"},
	} {
		if err := req.validate(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestPoolCreateRefusesDevicesInUse(t *testing.T) {
	newFakeHost(t, rootMount)
	req := poolCreateRequest{Name: "p", Devices: []string{"/dev/sda"}, Mountpoint: "/srv/pool/p"}
	if err := checkCreateTargets(req); err == nil || !strings.Contains(err.Error(), "in use") {
		t.Fatalf("the root disk: %v", err)
	}
	req.Devices = []string{"/dev/sdb"}
	if err := checkCreateTargets(req); err != nil {
		t.Fatalf("a free disk: %v", err)
	}
}

func TestPoolCreateRunsTheStepsAndWritesARealFstabLine(t *testing.T) {
	h := newFakeHost(t, rootMount)
	h.outputs["blkid -s UUID -o value /dev/sdb"] = "1111-2222"
	mp := "/srv/pool/p1"
	req := poolCreateRequest{Name: "p1", Devices: []string{"/dev/sdb", "/dev/sdc"}, Mountpoint: mp}
	if err := req.validate(); err != nil {
		t.Fatal(err)
	}
	res := createPool(context.Background(), req)
	if !res.OK {
		t.Fatalf("%s %+v", res.Error, res.Steps)
	}
	for _, want := range []string{
		"wipefs -n /dev/sdb", "mkfs.btrfs -L p1 -d raid1 -m raid1 /dev/sdb /dev/sdc", "btrfs device scan",
		"mount -t btrfs -o noatime,compress=zstd:3 /dev/sdb " + mp, "btrfs subvolume create " + mp + "/data",
	} {
		if !h.ranPrefix(want) {
			t.Errorf("did not run %q; ran %q", want, h.ran)
		}
	}
	fstab, _ := os.ReadFile(filepath.Join(h.etc, "fstab"))
	if want := "UUID=1111-2222 " + mp + " btrfs noatime,compress=zstd:3,nofail 0 0\n"; string(fstab) != want {
		t.Fatalf("fstab %q, want %q", fstab, want)
	}
}

func TestPoolCreateNeedsForceForUsedDevices(t *testing.T) {
	h := newFakeHost(t, rootMount)
	h.outputs["wipefs -n /dev/sdb"] = "DEVICE OFFSET TYPE\nsdb 0x438 ext4"
	req := poolCreateRequest{Name: "p1", Devices: []string{"/dev/sdb"}, Mountpoint: "/srv/pool/p1"}
	if err := req.validate(); err != nil {
		t.Fatal(err)
	}
	if res := createPool(context.Background(), req); res.OK || len(res.Warnings) != 1 || h.ranPrefix("mkfs") {
		t.Fatalf("without force: %+v, ran %q", res, h.ran)
	}
	req.Force = true
	if res := createPool(context.Background(), req); !res.OK || !h.ranPrefix("wipefs -a /dev/sdb") || !h.ranPrefix("mkfs.btrfs -f ") {
		t.Fatalf("with force: %+v, ran %q", res, h.ran)
	}
}

func TestPoolCreateDryRunRunsNothing(t *testing.T) {
	h := newFakeHost(t, rootMount)
	req := poolCreateRequest{Name: "p1", Devices: []string{"/dev/sdb"}, Mountpoint: "/srv/pool/p1", DryRun: true}
	if err := req.validate(); err != nil {
		t.Fatal(err)
	}
	res := createPool(context.Background(), req)
	if !res.OK || len(res.Steps) < 5 {
		t.Fatalf("%+v", res)
	}
	for _, l := range h.ran {
		if !strings.HasPrefix(l, "wipefs -n") {
			t.Fatalf("a dry run ran %q", l)
		}
	}
	if _, err := os.Stat(filepath.Join(h.etc, "fstab")); err == nil {
		t.Fatal("a dry run wrote fstab")
	}
}

func TestPoolCreateUndoesTheMountWhenAStepFails(t *testing.T) {
	h := newFakeHost(t, rootMount)
	h.fail = "btrfs subvolume create"
	mp := "/srv/pool/p1"
	req := poolCreateRequest{Name: "p1", Devices: []string{"/dev/sdb"}, Mountpoint: mp}
	if err := req.validate(); err != nil {
		t.Fatal(err)
	}
	res := createPool(context.Background(), req)
	if res.OK || !strings.Contains(res.Error, "subvol-data") || !h.ranPrefix("umount "+mp) {
		t.Fatalf("%+v, ran %q", res, h.ran)
	}
	if _, err := os.Stat(filepath.Join(h.etc, "fstab")); err == nil {
		t.Fatal("fstab written for a failed pool")
	}
}

func TestPoolCreateEncrypted(t *testing.T) {
	h := newFakeHost(t, rootMount)
	h.outputs["blkid -s UUID -o value /dev/sdb"] = "luks-uuid"
	h.outputs["blkid -s UUID -o value /dev/mapper/luks-p1-0"] = "fs-uuid"
	mp := "/srv/pool/p1"
	req := poolCreateRequest{Name: "p1", Devices: []string{"/dev/sdb"}, Mountpoint: mp, Keyfile: "/etc/nos/keys/p1.key", DryRun: false}
	if err := req.validate(); err != nil {
		t.Fatal(err)
	}
	old := makePoolKey
	makePoolKey = func(string) error { return nil }
	defer func() { makePoolKey = old }()
	res := createPool(context.Background(), req)
	if !res.OK {
		t.Fatalf("%s", res.Error)
	}
	if !h.ranPrefix("cryptsetup luksFormat --type luks2 --batch-mode --key-file /etc/nos/keys/p1.key /dev/sdb") ||
		!h.ranPrefix("mkfs.btrfs -L p1 -d single -m single /dev/mapper/luks-p1-0") {
		t.Fatalf("ran %q", h.ran)
	}
	crypttab, _ := os.ReadFile(filepath.Join(h.etc, "crypttab"))
	if string(crypttab) != "luks-p1-0 UUID=luks-uuid /etc/nos/keys/p1.key luks,nofail\n" {
		t.Fatalf("crypttab %q", crypttab)
	}
}

func TestPoolDestroy(t *testing.T) {
	h := newFakeHost(t, rootMount)
	mp := "/srv/pool/p1"
	h.outputs["btrfs filesystem show "+mp] = "Label: 'p1'  uuid: 1111\n\tTotal devices 2 FS bytes used 1MiB\n\tdevid    1 size 1GiB used 0 path /dev/sdb\n\tdevid    2 size 1GiB used 0 path /dev/sdc\n"
	if err := os.WriteFile(filepath.Join(h.etc, "fstab"), []byte("# keep\nUUID=root / ext4 defaults 0 1\nUUID=1111 /srv/pool/p1 btrfs noatime,nofail 0 0\nUUID=2222 /srv/pool/p10 btrfs noatime 0 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := destroyPool(context.Background(), poolDestroyRequest{Mountpoint: mp, Wipe: true})
	if !res.OK || len(res.Devices) != 2 {
		t.Fatalf("%+v", res)
	}
	for _, want := range []string{"umount " + mp, "wipefs -a /dev/sdb", "wipefs -a /dev/sdc"} {
		if !h.ranPrefix(want) {
			t.Errorf("did not run %q", want)
		}
	}
	fstab, _ := os.ReadFile(filepath.Join(h.etc, "fstab"))
	if string(fstab) != "# keep\nUUID=root / ext4 defaults 0 1\nUUID=2222 /srv/pool/p10 btrfs noatime 0 0\n" {
		t.Fatalf("fstab %q", fstab)
	}
}
