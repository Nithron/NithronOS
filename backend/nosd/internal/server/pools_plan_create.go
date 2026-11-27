package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"nos/backend/nosd/internal/pools"
	"nos/backend/nosd/pkg/agentclient"
	"nos/backend/nosd/pkg/httpx"
)

// Creating a pool: plan-create shows what will run (the agent checks the
// request and lists its steps without running them); apply-create sends
// the same request for real. The agent builds and runs the commands itself.

type planCreateRequest struct {
	pools.PoolSpec
	Force        bool   `json:"force"`
	MountOptions string `json:"mountOptions"`
}

// agentPoolOp calls one of the agent's pool operations; tests replace it.
var agentPoolOp = func(ctx context.Context, path string, body, out any) error {
	return agentclient.New("/run/nos-agent.sock").PostJSON(ctx, path, body, out)
}

const poolCreateTimeout = 15 * time.Minute

// agentPoolResult is the agent's answer to /v1/pools/create and destroy.
type agentPoolResult struct {
	OK         bool     `json:"ok"`
	Error      string   `json:"error"`
	Warnings   []string `json:"warnings"`
	UUID       string   `json:"uuid"`
	Mountpoint string   `json:"mountpoint"`
	Fstab      string   `json:"fstab"`
	Devices    []string `json:"devices"`
	Steps      []struct {
		ID          string `json:"id"`
		Command     string `json:"command"`
		Destructive bool   `json:"destructive"`
		Status      string `json:"status"`
		Output      string `json:"output"`
	} `json:"steps"`
}

var poolStepDescriptions = map[string]string{
	"wipe": "erase existing filesystem signatures", "key": "make the pool key",
	"luks-format": "encrypt the device (LUKS2)", "luks-open": "open the encrypted device",
	"mkfs": "create the btrfs filesystem", "scan": "find the pool's devices",
	"mkdir": "create the mountpoint", "mount": "mount the pool",
	"subvol": "create a default subvolume", "uuid": "read the filesystem UUID",
	"fstab": "mount the pool at boot (/etc/fstab)", "crypttab": "open the devices at boot (/etc/crypttab)",
	"umount": "unmount the pool", "luks-close": "close the encrypted device", "rmdir": "remove the mountpoint",
}

func describePoolStep(id string) string {
	for key := len(id); key > 0; key = strings.LastIndex(id[:key], "-") {
		if d, ok := poolStepDescriptions[id[:key]]; ok {
			return d
		}
	}
	return id
}

// agentCreateBody checks the request and turns it into the agent's.
func (req planCreateRequest) agentCreateBody(ctx context.Context, dryRun bool) (map[string]any, pools.PoolSpec, string, error) {
	spec, err := pools.ValidateSpec(req.PoolSpec)
	if err != nil {
		return nil, spec, "", err
	}
	opts := strings.TrimSpace(req.MountOptions)
	if opts == "" {
		opts = getDefaultMountOpts(ctx, spec.Devices)
	}
	body := map[string]any{
		"name": spec.Name, "devices": spec.Devices, "raid_data": spec.RaidData, "raid_meta": spec.RaidMeta,
		"mountpoint": spec.Mountpoint, "mount_options": opts, "force": req.Force, "dry_run": dryRun,
	}
	if spec.Encrypt.Enabled {
		body["keyfile"] = spec.Encrypt.Keyfile
	}
	return body, spec, opts, nil
}

// writeAgentPoolError answers with the agent's refusal (its 400 and 409 are
// about the request) or 502.
func writeAgentPoolError(w http.ResponseWriter, err error) {
	var he *agentclient.HTTPError
	if errors.As(err, &he) && (he.Status == http.StatusBadRequest || he.Status == http.StatusConflict || he.Status == http.StatusNotFound) {
		var body struct {
			Error string `json:"error"`
		}
		msg := he.Body
		if json.Unmarshal([]byte(he.Body), &body) == nil && body.Error != "" {
			msg = body.Error
		}
		httpx.WriteError(w, he.Status, msg)
		return
	}
	httpx.WriteError(w, http.StatusBadGateway, "nos-agent: "+err.Error())
}

func handlePlanCreateV1(w http.ResponseWriter, r *http.Request) {
	var req planCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}
	body, _, opts, err := req.agentCreateBody(r.Context(), true)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), poolQuickCallTimeout)
	defer cancel()
	var res agentPoolResult
	if err := agentPoolOp(ctx, "/v1/pools/create", body, &res); err != nil {
		writeAgentPoolError(w, err)
		return
	}
	if res.Error != "" {
		writeJSONStatus(w, http.StatusPreconditionFailed, map[string]any{
			"error": map[string]any{"code": "pools.signatures", "message": res.Error}, "warnings": res.Warnings,
		})
		return
	}
	steps := make([]pools.PlanStep, 0, len(res.Steps))
	for _, st := range res.Steps {
		steps = append(steps, pools.PlanStep{ID: st.ID, Description: describePoolStep(st.ID), Command: st.Command, Destructive: st.Destructive})
	}
	warnings := res.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	writeJSON(w, map[string]any{"plan": pools.CreatePlan{Steps: steps}, "fstab": []string{res.Fstab}, "warnings": warnings, "mountOptions": opts})
}

func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
