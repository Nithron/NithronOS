package pools

import "testing"

// findmnt -J output as Debian prints it (spaces after colons, nulls where
// udev knows nothing), with a container's file bind mounts.
const findmntSample = `{
   "filesystems": [
      {"target": "/", "source": "/dev/sda2", "uuid": "aaaa", "label": null},
      {"target": "/etc/hostname", "source": "/dev/sdz1", "uuid": null, "label": null},
      {"target": "/srv/pool/media/snapshots", "source": "/dev/sdb", "uuid": "bbbb", "label": "media"},
      {"target": "/srv/pool/media", "source": "/dev/sdb", "uuid": "bbbb", "label": "media"},
      {"target": "/mnt/pool1", "source": "/dev/loop3", "uuid": null, "label": null}
   ]
}`

func TestPoolsFromFindmnt(t *testing.T) {
	dirs := map[string]bool{"/": true, "/srv/pool/media": true, "/srv/pool/media/snapshots": true, "/mnt/pool1": true}
	usage := func(string) (uint64, uint64, uint64) { return 100, 40, 60 }
	got := poolsFromFindmnt([]byte(findmntSample), usage, func(p string) bool { return dirs[p] })
	if len(got) != 3 {
		t.Fatalf("pools: %+v", got)
	}
	want := []struct{ id, mount, label, uuid, device string }{
		{"aaaa", "/", "system", "aaaa", "/dev/sda2"},
		{"bbbb", "/srv/pool/media", "media", "bbbb", "/dev/sdb"},
		{"pool1", "/mnt/pool1", "pool1", "", "/dev/loop3"},
	}
	for i, w := range want {
		p := got[i]
		if p.ID != w.id || p.Mount != w.mount || p.Label != w.label || p.UUID != w.uuid || len(p.Devices) != 1 || p.Devices[0] != w.device {
			t.Errorf("pool %d: %+v, want %+v", i, p, w)
		}
		if p.Size != 100 || p.Used != 40 || p.Free != 60 {
			t.Errorf("pool %d usage: %+v", i, p)
		}
	}
	if len(poolsFromFindmnt([]byte(`not json`), usage, func(string) bool { return true })) != 0 {
		t.Error("bad output must give no pools")
	}
}

// Pool ids go into URLs: never a path, and unique.
func TestPoolIDsAreURLSafeAndUnique(t *testing.T) {
	out := []byte(`{"filesystems":[{"target":"/a/data","source":"/dev/sdb"},{"target":"/b/data","source":"/dev/sdc"}]}`)
	got := poolsFromFindmnt(out, func(string) (uint64, uint64, uint64) { return 0, 0, 0 }, func(string) bool { return true })
	if len(got) != 2 || got[0].ID != "data" || got[1].ID != "data-2" {
		t.Fatalf("ids: %+v", got)
	}
}
