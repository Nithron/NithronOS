package server

import "testing"

func TestSmartTestState(t *testing.T) {
	ata := "Self-test execution status:      ( 249)	Self-test routine in progress...\n\t\t\t\t\t90% of test remaining.\n"
	if run, pct := smartTestState(ata); !run || pct != 10 {
		t.Fatalf("ata: %v %d", run, pct)
	}
	nvme := "Self-test status: Short self-test in progress (35% completed)\n"
	if run, pct := smartTestState(nvme); !run || pct != 35 {
		t.Fatalf("nvme: %v %d", run, pct)
	}
	idle := "Self-test execution status:      (   0)	The previous self-test routine completed\n\t\t\t\t\twithout error or no self-test has ever \n\t\t\t\t\tbeen run.\n"
	if run, _ := smartTestState(idle); run {
		t.Fatal("idle disk reported running")
	}
	for _, bad := range []string{"", "/etc/passwd", "/dev/../etc/passwd", "sda", "/dev/null"} {
		if isBlockDevice(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}
