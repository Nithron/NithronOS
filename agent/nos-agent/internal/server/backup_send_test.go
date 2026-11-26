package server

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Runs the generated pipeline with stub btrfs and ssh that record their
// arguments: each value must arrive as one argument, and nothing in it is
// run as shell code.
func TestBackupCommandPipeline(t *testing.T) {
	bin := t.TempDir()
	logs := t.TempDir()
	for _, name := range []string{"btrfs", "ssh"} {
		// Each stub logs to its own file (the two run at the same time).
		logf := filepath.Join(logs, name+"-$$")
		stub := "#!/bin/sh\nf=" + logf + "\nprintf '%s' " + name + " > $f\nfor a in \"$@\"; do printf ' [%s]' \"$a\" >> $f; done\necho >> $f\ncat >/dev/null\n"
		if err := os.WriteFile(filepath.Join(bin, name), []byte(stub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	run := func(req BackupSendRequest) string {
		t.Helper()
		if msg := req.validate(); msg != "" {
			t.Fatalf("validate: %s", msg)
		}
		cmd := backupCommand(req)
		i := slicesIndex(cmd, "-c")
		c := exec.Command("/bin/bash", append([]string{"-o", "pipefail", "-c"}, cmd[i+1:]...)...)
		c.Env = []string{"PATH=" + bin + ":/usr/bin:/bin"}
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("pipeline: %v %s", err, out)
		}
		var all strings.Builder
		entries, _ := os.ReadDir(logs)
		for _, e := range entries {
			b, _ := os.ReadFile(filepath.Join(logs, e.Name()))
			all.Write(b)
			_ = os.Remove(filepath.Join(logs, e.Name()))
		}
		return all.String()
	}

	got := run(BackupSendRequest{Job: "job-12345678", Snapshot: "/srv/shares/media/.snapshots/20261126-120000-daily", Parent: "/srv/shares/media/.snapshots/20261125-120000-daily",
		Target: "ssh", Host: "backup.lan", Port: 2222, User: "nos", Path: "/backups/media", Key: "id_ed25519"})
	for _, want := range []string{
		"btrfs [send] [-p] [/srv/shares/media/.snapshots/20261125-120000-daily] [/srv/shares/media/.snapshots/20261126-120000-daily]",
		"[-p] [2222] [-i] [/etc/nos/backup/keys/id_ed25519] [--] [nos@backup.lan] [btrfs receive /backups/media]",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}

	got = run(BackupSendRequest{Job: "job-12345678", Snapshot: "/mnt/pool/.snapshots/20261126-120000-x", Target: "local", Path: "/mnt/usb/backups"})
	if !strings.Contains(got, "btrfs [send] [/mnt/pool/.snapshots/20261126-120000-x]") || !strings.Contains(got, "btrfs [receive] [/mnt/usb/backups]") {
		t.Errorf("local:\n%s", got)
	}

	for _, bad := range []BackupSendRequest{
		{Job: "job-12345678", Snapshot: "/etc", Target: "local", Path: "/x"},
		{Job: "job-12345678", Snapshot: "/srv/a/.snapshots/20261126-120000-x", Target: "ssh", Host: "h", Port: 22, User: "u", Path: "/x; rm -rf /"},
		{Job: "job-12345678", Snapshot: "/srv/a/.snapshots/20261126-120000-x", Target: "ssh", Host: "-oProxyCommand=x", Port: 22, User: "u", Path: "/x"},
		{Job: "job-12345678", Snapshot: "/srv/a/.snapshots/20261126-120000-x", Target: "ssh", Host: "h", Port: 22, User: "u", Path: "/x", Key: "../../etc/shadow"},
		{Job: "job-12345678", Snapshot: "/srv/a/.snapshots/20261126-120000-x", Parent: "/srv/b/.snapshots/20261125-120000-x", Target: "local", Path: "/x"},
	} {
		if bad.validate() == "" {
			t.Errorf("accepted %+v", bad)
		}
	}
}

func slicesIndex(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}
