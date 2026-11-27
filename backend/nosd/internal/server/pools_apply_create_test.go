package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"nos/backend/nosd/internal/pools"
)

// apply-create sends the pool, not command lines, and remembers the pool
// with the UUID the agent read.
func TestApplyCreateRecordsThePool(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NOS_ETC_DIR", dir)
	t.Setenv("NOS_STATE_DIR", dir)
	t.Setenv("NOS_DISABLE_APP_EVENTS", "1")
	agent := useFakePoolAgent(t, func(_ string, body map[string]any) (agentPoolResult, error) {
		res, _ := planned("", body)
		res.UUID = "1234-abcd"
		res.Steps[0].Status = "ok"
		return res, nil
	})
	res := postJSON(t, "/api/v1/pools/apply-create", map[string]any{
		"name": "p1", "mountpoint": "/srv/pool/p1", "devices": []string{"/dev/sdb"},
		"mountOptions": "compress=zstd:3,noatime", "confirm": "CREATE",
	})
	if res.Code != http.StatusOK {
		t.Fatalf("%d %s", res.Code, res.Body.String())
	}
	var out struct {
		TxID string `json:"tx_id"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &out)
	txWorkers.Wait()

	if len(agent.calls) != 1 || agent.calls[0]["dry_run"] != false || agent.calls[0]["mountpoint"] != "/srv/pool/p1" {
		t.Fatalf("agent request: %v", agent.calls)
	}
	var tx pools.Tx
	if ok, _ := loadTx(out.TxID, &tx); !ok || !tx.OK || len(tx.Steps) != 1 || tx.Steps[0].Status != "ok" {
		t.Fatalf("tx: %+v", tx)
	}
	b, err := os.ReadFile(filepath.Join(dir, "nos", "pools.json"))
	if err != nil || !bytes.Contains(b, []byte(`"uuid": "1234-abcd"`)) && !bytes.Contains(b, []byte(`"uuid":"1234-abcd"`)) {
		t.Fatalf("pools.json: %s %v", b, err)
	}
	if !bytes.Contains(b, []byte("compress=zstd:3,noatime")) {
		t.Fatalf("pools.json lacks the mount options: %s", b)
	}
}

func TestApplyCreateFailureIsRecorded(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NOS_ETC_DIR", dir)
	t.Setenv("NOS_STATE_DIR", dir)
	useFakePoolAgent(t, func(_ string, body map[string]any) (agentPoolResult, error) {
		res, _ := planned("", body)
		res.OK = false
		res.Error = "mkfs: exit status 1: device busy"
		res.Steps[0].Status = "error"
		res.Steps[0].Output = "device busy"
		return res, nil
	})
	res := postJSON(t, "/api/v1/pools/apply-create", map[string]any{"name": "p1", "devices": []string{"/dev/sdb"}, "confirm": "CREATE"})
	var out struct {
		TxID string `json:"tx_id"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &out)
	txWorkers.Wait()
	var tx pools.Tx
	if ok, _ := loadTx(out.TxID, &tx); !ok || tx.OK || tx.Error == "" || tx.Steps[0].Err != "device busy" {
		t.Fatalf("tx: %+v", tx)
	}
	if _, err := os.Stat(filepath.Join(dir, "nos", "pools.json")); err == nil {
		t.Fatal("a failed pool was recorded")
	}
}

func TestApplyCreateNeedsConfirm(t *testing.T) {
	agent := useFakePoolAgent(t, planned)
	if res := postJSON(t, "/api/v1/pools/apply-create", map[string]any{"name": "p1", "devices": []string{"/dev/sdb"}}); res.Code != http.StatusPreconditionRequired {
		t.Fatalf("%d", res.Code)
	}
	if len(agent.calls) != 0 {
		t.Fatal("the agent was asked")
	}
}

func loadTx(id string, v any) (bool, error) {
	b, err := os.ReadFile(txPath(id))
	if err != nil {
		return false, err
	}
	return json.Unmarshal(b, v) == nil, nil
}
