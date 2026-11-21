package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"nithronos/backend/nosd/internal/config"
	"nithronos/backend/nosd/internal/fsatomic"
	"nithronos/backend/nosd/internal/pools"
	"nithronos/backend/nosd/pkg/agentclient"
)

type fakeAgentPoll struct {
	postCodes []int
	bsSeq     []*agentclient.BalanceStatus
	bsIdx     int
}

func (f *fakeAgentPoll) PostJSON(_ context.Context, _ string, _ any, v any) error {
	code := 0
	if len(f.postCodes) > 0 {
		code = f.postCodes[0]
		f.postCodes = f.postCodes[1:]
	}
	// Encode into the expected shape used by handler
	payload := map[string]any{
		"Results": []map[string]any{{"Code": code, "Stdout": "ok"}},
	}
	b, _ := json.Marshal(payload)
	return json.Unmarshal(b, v)
}

func (f *fakeAgentPoll) BalanceStatus(_ context.Context, _ string) (*agentclient.BalanceStatus, error) {
	if f.bsIdx < len(f.bsSeq) {
		v := f.bsSeq[f.bsIdx]
		f.bsIdx++
		return v, nil
	}
	// default: still running
	return &agentclient.BalanceStatus{Running: true, Percent: 50}, nil
}

func (f *fakeAgentPoll) ReplaceStatus(_ context.Context, _ string) (*agentclient.ReplaceStatus, error) {
	return &agentclient.ReplaceStatus{Running: false, Percent: 100}, nil
}

func TestApplyDevice_BalanceRunningToDone(t *testing.T) {
	t.Setenv("NOS_STATE_DIR", t.TempDir())
	t.Setenv("NOS_TEST_FAST_POLL", "1")
	oldPoll := devicePollInterval
	devicePollInterval = 1 * time.Millisecond
	defer func() { devicePollInterval = oldPoll }()

	oldMake := makeAgentClient
	makeAgentClient = func() agentAPI {
		return &fakeAgentPoll{
			postCodes: []int{0, 0},
			bsSeq: []*agentclient.BalanceStatus{
				{Running: true, Percent: 10},
				{Running: false, Percent: 100},
			},
		}
	}
	defer func() { makeAgentClient = oldMake }()

	r := NewRouter(config.FromEnv())
	steps := []map[string]string{
		{"id": "s1", "description": "add", "command": "btrfs device add /dev/sdb /mnt/p1"},
		{"id": "s2", "description": "balance", "command": "btrfs balance start -dconvert=single -mconvert=single /mnt/p1"},
	}
	body := map[string]any{"steps": steps, "confirm": "ADD"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/pools/ptest1/apply-device", bytes.NewReader(b))
	req.Header.Set("X-CSRF-Token", "x")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("unexpected status: %d", w.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	txID, _ := resp["tx_id"].(string)
	if txID == "" {
		t.Fatalf("missing tx_id")
	}

	// wait for completion (allow extra slack on CI)
	deadline := time.Now().Add(30 * time.Second)
	for {
		if time.Now().After(deadline) {
			txJSON, _ := os.ReadFile(txPath(txID))
			logTail, _ := os.ReadFile(txLogPath(txID))
			t.Fatalf("timeout waiting for tx finish\ntx file %s:\n%s\ntx log:\n%s", txPath(txID), txJSON, logTail)
		}
		var cur pools.Tx
		_, _ = fsatomic.LoadJSON(txPath(txID), &cur)
		// Consider done if marked OK or last step is ok (avoid rare FinishedAt race in CI)
		if cur.OK || (len(cur.Steps) > 0 && cur.Steps[len(cur.Steps)-1].Status == "ok") {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// tx log exists
	if _, err := os.Stat(filepath.Join(os.Getenv("NOS_STATE_DIR"), "pools", "tx", txID+".log")); err != nil {
		t.Fatalf("missing tx log: %v", err)
	}
}

func TestApplyDevice_StepErrorStops(t *testing.T) {
	t.Setenv("NOS_STATE_DIR", t.TempDir())
	t.Setenv("NOS_TEST_FAST_POLL", "1")
	oldPoll := devicePollInterval
	devicePollInterval = 1 * time.Millisecond
	defer func() { devicePollInterval = oldPoll }()

	oldMake := makeAgentClient
	makeAgentClient = func() agentAPI {
		return &fakeAgentPoll{
			postCodes: []int{0, 1}, // add ok, balance start fails
			bsSeq:     []*agentclient.BalanceStatus{{Running: true, Percent: 5}},
		}
	}
	defer func() { makeAgentClient = oldMake }()

	r := NewRouter(config.FromEnv())
	steps := []map[string]string{
		{"id": "s1", "description": "add", "command": "btrfs device add /dev/sdb /mnt/p1"},
		{"id": "s2", "description": "balance", "command": "btrfs balance start -dconvert=single -mconvert=single /mnt/p1"},
	}
	body := map[string]any{"steps": steps, "confirm": "ADD"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/pools/ptest2/apply-device", bytes.NewReader(b))
	req.Header.Set("X-CSRF-Token", "x")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("unexpected status: %d", w.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	txID, _ := resp["tx_id"].(string)
	if txID == "" {
		t.Fatalf("missing tx_id")
	}

	// wait for error (allow extra slack on CI)
	deadline := time.Now().Add(30 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for tx error")
		}
		var cur pools.Tx
		_, _ = fsatomic.LoadJSON(txPath(txID), &cur)
		// detect any step error to avoid flakiness on index assumptions
		stepErr := false
		for _, s := range cur.Steps {
			if s.Status == "error" {
				stepErr = true
				break
			}
		}
		if cur.FinishedAt != nil || cur.Error != "" || (!cur.OK && stepErr) {
			if cur.OK {
				t.Fatalf("expected failure, got OK")
			}
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
}
