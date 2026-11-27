package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nos/backend/nosd/internal/config"
	"nos/backend/nosd/pkg/agentclient"
)

// fakePoolAgent stands in for the agent's pool operations: it records the
// requests and answers with what answer returns.
type fakePoolAgent struct {
	calls  []map[string]any
	paths  []string
	answer func(path string, body map[string]any) (agentPoolResult, error)
}

func useFakePoolAgent(t *testing.T, answer func(path string, body map[string]any) (agentPoolResult, error)) *fakePoolAgent {
	t.Helper()
	f := &fakePoolAgent{answer: answer}
	old := agentPoolOp
	agentPoolOp = func(_ context.Context, path string, body, out any) error {
		b, _ := json.Marshal(body)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		f.calls = append(f.calls, m)
		f.paths = append(f.paths, path)
		res, err := f.answer(path, m)
		if err != nil {
			return err
		}
		b, _ = json.Marshal(res)
		return json.Unmarshal(b, out)
	}
	t.Cleanup(func() { agentPoolOp = old })
	return f
}

// planned answers a dry run with an mkfs step and an fstab line.
func planned(_ string, body map[string]any) (agentPoolResult, error) {
	var res agentPoolResult
	res.OK = true
	res.Fstab = "UUID=<uuid> " + body["mountpoint"].(string) + " btrfs " + body["mount_options"].(string) + ",nofail 0 0"
	res.Steps = append(res.Steps, struct {
		ID          string `json:"id"`
		Command     string `json:"command"`
		Destructive bool   `json:"destructive"`
		Status      string `json:"status"`
		Output      string `json:"output"`
	}{ID: "mkfs", Command: "'mkfs.btrfs' ...", Destructive: true, Status: "planned"})
	return res, nil
}

func postJSON(t *testing.T, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	res := httptest.NewRecorder()
	NewRouter(config.FromEnv()).ServeHTTP(res, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b)))
	return res
}

func TestPlanCreateAsksTheAgentForADryRun(t *testing.T) {
	agent := useFakePoolAgent(t, planned)
	res := postJSON(t, "/api/v1/pools/plan-create", map[string]any{
		"name": "pool2", "mountpoint": "/srv/pool/pool2", "devices": []string{"/dev/sda", "/dev/sdb", "/dev/sda"},
	})
	if res.Code != http.StatusOK {
		t.Fatalf("%d %s", res.Code, res.Body.String())
	}
	if len(agent.calls) != 1 || agent.paths[0] != "/v1/pools/create" {
		t.Fatalf("agent calls: %v %v", agent.paths, agent.calls)
	}
	got := agent.calls[0]
	if got["dry_run"] != true || got["raid_data"] != "raid1" || got["raid_meta"] != "raid1" || len(got["devices"].([]any)) != 2 {
		t.Fatalf("agent request: %v", got)
	}
	var out struct {
		Plan struct {
			Steps []struct{ ID, Description, Command string }
		}
		Fstab []string
	}
	_ = json.Unmarshal(res.Body.Bytes(), &out)
	if len(out.Plan.Steps) != 1 || out.Plan.Steps[0].Description != "create the btrfs filesystem" || len(out.Fstab) != 1 {
		t.Fatalf("plan: %s", res.Body.String())
	}
}

func TestPlanCreateRefusesBadSpecs(t *testing.T) {
	agent := useFakePoolAgent(t, planned)
	for name, body := range map[string]map[string]any{
		"raid5":         {"name": "p", "devices": []string{"/dev/sda", "/dev/sdb", "/dev/sdc"}, "raidData": "raid5"},
		"no devices":    {"name": "p"},
		"bad name":      {"name": "../etc", "devices": []string{"/dev/sda"}},
		"system folder": {"name": "p", "mountpoint": "/etc", "devices": []string{"/dev/sda"}},
		"unclean path":  {"name": "p", "mountpoint": "/srv/../etc", "devices": []string{"/dev/sda"}},
	} {
		if res := postJSON(t, "/api/v1/pools/plan-create", body); res.Code != http.StatusBadRequest {
			t.Errorf("%s: %d %s", name, res.Code, res.Body.String())
		}
	}
	if len(agent.calls) != 0 {
		t.Fatalf("the agent was asked: %v", agent.calls)
	}
}

func TestPlanCreateDefaultMountOptionsByDeviceMix(t *testing.T) {
	useFakePoolAgent(t, planned)
	old := getDefaultMountOpts
	getDefaultMountOpts = func(_ context.Context, devs []string) string {
		for _, d := range devs {
			if strings.Contains(d, "nvme") {
				return "compress=zstd:3,ssd,discard=async,noatime"
			}
		}
		return "compress=zstd:3,noatime"
	}
	defer func() { getDefaultMountOpts = old }()

	ssd := postJSON(t, "/api/v1/pools/plan-create", map[string]any{"name": "p1", "devices": []string{"/dev/nvme0n1"}})
	if ssd.Code != http.StatusOK || !strings.Contains(ssd.Body.String(), "ssd,discard=async") {
		t.Fatalf("ssd: %d %s", ssd.Code, ssd.Body.String())
	}
	hdd := postJSON(t, "/api/v1/pools/plan-create", map[string]any{"name": "p2", "devices": []string{"/dev/sda"}})
	if hdd.Code != http.StatusOK || strings.Contains(hdd.Body.String(), "ssd,discard=async") {
		t.Fatalf("hdd: %d %s", hdd.Code, hdd.Body.String())
	}
}

// A device with a filesystem: the agent's dry run says so, nosd answers 412.
func TestPlanCreateWarnsWithoutForce(t *testing.T) {
	useFakePoolAgent(t, func(_ string, body map[string]any) (agentPoolResult, error) {
		if body["force"] == true {
			return planned("", body)
		}
		return agentPoolResult{Error: "devices have existing signatures; set force to erase them", Warnings: []string{"/dev/sda has a filesystem"}}, nil
	})
	if res := postJSON(t, "/api/v1/pools/plan-create", map[string]any{"name": "p", "devices": []string{"/dev/sda"}}); res.Code != http.StatusPreconditionFailed {
		t.Fatalf("without force: %d %s", res.Code, res.Body.String())
	}
	if res := postJSON(t, "/api/v1/pools/plan-create", map[string]any{"name": "p", "devices": []string{"/dev/sda"}, "force": true}); res.Code != http.StatusOK {
		t.Fatalf("with force: %d %s", res.Code, res.Body.String())
	}
}

// The agent's refusals (a device in use, not a block device) reach the UI.
func TestPlanCreatePassesAgentRefusals(t *testing.T) {
	useFakePoolAgent(t, func(string, map[string]any) (agentPoolResult, error) {
		return agentPoolResult{}, &agentclient.HTTPError{Status: http.StatusConflict, Body: `{"error":"/dev/sda is in use (mounted at /)"}`}
	})
	res := postJSON(t, "/api/v1/pools/plan-create", map[string]any{"name": "p", "devices": []string{"/dev/sda"}})
	if res.Code != http.StatusConflict || !strings.Contains(res.Body.String(), "in use") {
		t.Fatalf("%d %s", res.Code, res.Body.String())
	}
}
