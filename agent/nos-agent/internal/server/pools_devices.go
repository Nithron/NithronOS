package server

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Whether a disk may become part of a new pool: the kernel's view, not the
// path's spelling. /dev/disk/by-id/... and /dev/sdb are the same disk; a
// disk is in use when it or one of its partitions is mounted, is swap, or
// is held by something (LVM, md, LUKS, another btrfs device mapping).

// sysBlock and swapsPath are /sys/class/block and /proc/swaps; tests point
// them elsewhere.
var (
	sysBlock  = "/sys/class/block"
	swapsPath = "/proc/swaps"
)

// resolveDevice returns the kernel device a path names (/dev/<name>).
var resolveDevice = func(p string) (string, error) {
	return filepath.EvalSymlinks(p)
}

// kernelNames returns the device's kernel name and its partitions' names.
func kernelNames(dev string) []string {
	name := filepath.Base(dev)
	names := []string{name}
	ents, _ := os.ReadDir(filepath.Join(sysBlock, name))
	for _, e := range ents {
		if _, err := os.Stat(filepath.Join(sysBlock, name, e.Name(), "partition")); err == nil {
			names = append(names, e.Name())
		}
	}
	return names
}

// deviceInUse says why dev (a resolved /dev/<name>) can't be used, or "".
func deviceInUse(dev string, mounts []mountEntry) string {
	names := kernelNames(dev)
	inSet := map[string]bool{}
	for _, n := range names {
		inSet[n] = true
		if holders, _ := os.ReadDir(filepath.Join(sysBlock, n, "holders")); len(holders) > 0 {
			return fmt.Sprintf("/dev/%s is held by %s (LVM, RAID or encryption)", n, holders[0].Name())
		}
	}
	for _, m := range mounts {
		src := m.source
		if !strings.HasPrefix(src, "/dev/") {
			continue
		}
		if r, err := resolveDevice(src); err == nil {
			src = r
		}
		if inSet[filepath.Base(src)] {
			return fmt.Sprintf("%s is mounted at %s", src, m.target)
		}
	}
	if f, err := os.Open(swapsPath); err == nil {
		defer func() { _ = f.Close() }()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			fields := strings.Fields(sc.Text())
			if len(fields) > 0 && strings.HasPrefix(fields[0], "/dev/") && inSet[filepath.Base(fields[0])] {
				return fields[0] + " is swap"
			}
		}
	}
	return ""
}
