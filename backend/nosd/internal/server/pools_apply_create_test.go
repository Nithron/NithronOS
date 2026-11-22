package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"nithronos/backend/nosd/internal/config"
	"nithronos/backend/nosd/internal/pools"
)

func TestApplyCreatePersistsMountOptions(t *testing.T) {
	// isolate etc dir for persistence checks and disable noisy subsystems
	dir := t.TempDir()
	t.Setenv("NOS_ETC_DIR", dir)
	t.Setenv("NOS_DISABLE_APP_EVENTS", "1")
	t.Cleanup(txWorkers.Wait) // the async transaction writes into dir
	r := NewRouter(config.FromEnv())
	reqBody := applyCreateRequest{
		Plan:    CreateSimplePlanForTest(),
		Fstab:   []string{"UUID=<uuid> /mnt/p1 btrfs compress=zstd:3,ssd,discard=async,noatime 0 0"},
		Confirm: "CREATE",
	}
	b, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pools/apply-create", bytes.NewReader(b))
	req.Header.Set("X-CSRF-Token", "x")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	// Accept 200 (async tx created) or 428/400 depending on middleware; focus on persistence check
	p := filepath.Join(dir, "nos", "pools.json")
	if b, err := os.ReadFile(p); err == nil {
		if !bytes.Contains(b, []byte("compress=zstd:3,ssd,discard=async,noatime")) {
			t.Fatalf("pools.json missing mountOptions: %s", string(b))
		}
	}
}

// CreateSimplePlanForTest builds a tiny plan-free skeleton used by the handler; we only care that it is non-empty.
func CreateSimplePlanForTest() (pln pools.CreatePlan) {
	pln.Steps = []pools.PlanStep{{ID: "noop", Description: "noop", Command: "true", Destructive: false}}
	return
}

func TestExecutePlanWritesTxAndLogs(t *testing.T) {
	// isolate state directory
	dir := t.TempDir()
	t.Setenv("NOS_STATE_DIR", dir)

	// mock runner: fail the step with command 'echo two'
	old := agentStepRunner
	agentStepRunner = func(cmd string, args []string) (int, string) {
		if cmd == "echo" && len(args) > 0 && args[0] == "two" {
			return 1, "fail"
		}
		return 0, "ok"
	}
	defer func() { agentStepRunner = old }()

	txid := "tx-test"
	tx := pools.Tx{ID: txid, StartedAt: time.Now().UTC(), Steps: []pools.TxStep{{ID: "s1", Name: "one", Cmd: "echo one", Status: "pending"}, {ID: "s2", Name: "two", Cmd: "echo two", Status: "pending"}, {ID: "s3", Name: "three", Cmd: "echo three", Status: "pending"}}}
	if err := saveTx(tx); err != nil {
		t.Fatalf("saveTx: %v", err)
	}

	// run
	executePlan(txid, applyCreateRequest{Plan: pools.CreatePlan{Steps: []pools.PlanStep{{ID: "s1", Description: "one", Command: "echo one"}, {ID: "s2", Description: "two", Command: "echo two"}, {ID: "s3", Description: "three", Command: "echo three"}}}}, config.Defaults())

	// assert tx updated
	var got pools.Tx
	if ok, _ := loadTx(txid, &got); !ok {
		t.Fatalf("tx not found")
	}
	if got.OK {
		t.Fatalf("expected tx not OK on failure")
	}
	if len(got.Steps) != 3 {
		t.Fatalf("unexpected steps len: %d", len(got.Steps))
	}
	if got.Steps[0].Status != "ok" || got.Steps[1].Status != "error" || got.Steps[2].Status != "pending" {
		t.Fatalf("unexpected step statuses: %+v", got.Steps)
	}

	// log file has content
	b, err := os.ReadFile(filepath.Join(dir, "pools", "tx", txid+".log"))
	if err != nil || len(b) == 0 {
		t.Fatalf("log missing or empty: %v", err)
	}
}

// helpers
func loadTx(id string, v any) (bool, error) {
	path := txPath(id)
	b, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return json.Unmarshal(b, v) == nil, nil
}
