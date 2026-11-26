package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListeningPorts(t *testing.T) {
	f := filepath.Join(t.TempDir(), "tcp")
	content := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:01BB 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 1 1
   1: 0100007F:2328 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 2 1
   2: 0A00000F:9C40 0A000001:0050 01 00000000:00000000 00:00000000 00000000     0        0 3 1
`
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got := listeningPorts(f, filepath.Join(t.TempDir(), "missing"))
	if len(got) != 2 || got[0] != 443 || got[1] != 9000 {
		t.Fatalf("ports: %v (an established connection is not listening)", got)
	}
}
