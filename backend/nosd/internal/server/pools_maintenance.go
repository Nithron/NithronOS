package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"nos/backend/nosd/internal/pools"
	"nos/backend/nosd/pkg/agentclient"
	"nos/backend/nosd/pkg/httpx"
)

// Scrub and balance for the Btrfs pools. One implementation behind
// /api/v1/{scrub,balance}/{status,start,cancel} (every pool, or the one named
// by mount or poolId) and /api/v1/pools/scrub/{start,status} (one mount).
// (Before, /api/v1/btrfs/* returned made-up status and pretended to start
// and cancel, /balance/start only created a job, /scrub/cancel did nothing,
// and the agent ran scrubs with -B, holding the request for hours.)

// ScrubStatus is one pool's scrub state.
type ScrubStatus struct {
	PoolID       string  `json:"poolId"`
	Mount        string  `json:"mount"`
	Status       string  `json:"status"` // idle, running, finished, aborted, interrupted, unknown
	Progress     *int    `json:"progress,omitempty"`
	BytesScanned *uint64 `json:"bytesScanned,omitempty"`
	BytesTotal   *uint64 `json:"bytesTotal,omitempty"`
	Errors       *int    `json:"errors,omitempty"`
	StartedAt    *string `json:"startedAt,omitempty"`
	FinishedAt   *string `json:"finishedAt,omitempty"`
	Error        string  `json:"error,omitempty"`
}

// BalanceStatus is one pool's balance state.
type BalanceStatus struct {
	PoolID   string `json:"poolId"`
	Mount    string `json:"mount"`
	Status   string `json:"status"` // idle, running, unknown
	Progress *int   `json:"progress,omitempty"`
	Left     string `json:"left,omitempty"`
	Total    string `json:"total,omitempty"`
	Error    string `json:"error,omitempty"`
}

// listPoolsFn finds the pools; tests replace it.
var listPoolsFn = pools.ListPools

type poolMaintenance struct {
	agent AgentClient
}

func (m poolMaintenance) routes(pr chi.Router, adminRequired func(http.Handler) http.Handler) {
	pr.Get("/api/v1/scrub/status", m.scrubStatus)
	pr.Get("/api/v1/balance/status", m.balanceStatus)
	pr.With(adminRequired).Post("/api/v1/scrub/start", m.op("/v1/btrfs/scrub/start", true))
	pr.With(adminRequired).Post("/api/v1/scrub/cancel", m.op("/v1/btrfs/scrub/cancel", false))
	pr.With(adminRequired).Post("/api/v1/balance/start", m.op("/v1/btrfs/balance/start", true))
	pr.With(adminRequired).Post("/api/v1/balance/cancel", m.op("/v1/btrfs/balance/cancel", false))
	// One mount (api/client.ts).
	pr.With(adminRequired).Post("/api/v1/pools/scrub/start", m.op("/v1/btrfs/scrub/start", true))
	pr.With(adminRequired).Get("/api/v1/pools/scrub/status", func(w http.ResponseWriter, r *http.Request) {
		mount := r.URL.Query().Get("mount")
		if mount == "" {
			httpx.WriteTypedError(w, http.StatusBadRequest, "pool.mount_required", "mount is required", 0)
			return
		}
		if out, ok := m.scrubStatuses(w, r, target{mount: mount}); ok {
			writeJSON(w, out[0])
		}
	})
}

// target is the pool named in the request (mount or poolId), or every pool.
type target struct{ mount, poolID string }

// selectPools returns the mounted pools the request is about. A named pool
// that isn't found is an error (the operation must not fall back to all).
func selectPools(ctx context.Context, t target) ([]pools.Pool, error) {
	list, err := listPoolsFn(ctx)
	if err != nil {
		return nil, err
	}
	var out []pools.Pool
	for _, p := range list {
		if p.Mount == "" {
			continue
		}
		if (t.mount == "" && t.poolID == "") || p.Mount == t.mount || (t.poolID != "" && (p.ID == t.poolID || p.UUID == t.poolID)) {
			out = append(out, p)
		}
	}
	if len(out) == 0 && (t.mount != "" || t.poolID != "") {
		return nil, errPoolNotFound
	}
	return out, nil
}

var errPoolNotFound = errors.New("no mounted pool matches")

func requestTarget(r *http.Request) target {
	q := r.URL.Query()
	return target{mount: q.Get("mount"), poolID: q.Get("poolId")}
}

func (m poolMaintenance) pools(w http.ResponseWriter, r *http.Request, t target) ([]pools.Pool, bool) {
	list, err := selectPools(r.Context(), t)
	switch {
	case errors.Is(err, errPoolNotFound):
		httpx.WriteTypedError(w, http.StatusNotFound, "pool.not_found", "No mounted pool matches", 0)
		return nil, false
	case err != nil:
		httpx.WriteTypedError(w, http.StatusServiceUnavailable, "pools.unavailable", "Could not list pools: "+err.Error(), 0)
		return nil, false
	}
	return list, true
}

func (m poolMaintenance) scrubStatus(w http.ResponseWriter, r *http.Request) {
	if out, ok := m.scrubStatuses(w, r, requestTarget(r)); ok {
		writeJSON(w, out)
	}
}

// scrubStatuses answers the error itself when it returns false.
func (m poolMaintenance) scrubStatuses(w http.ResponseWriter, r *http.Request, t target) ([]ScrubStatus, bool) {
	list, ok := m.pools(w, r, t)
	if !ok {
		return nil, false
	}
	out := make([]ScrubStatus, 0, len(list))
	for _, p := range list {
		st := ScrubStatus{PoolID: p.ID, Mount: p.Mount}
		var resp struct {
			Status string `json:"status"`
		}
		if err := m.agent.GetJSON(r.Context(), "/v1/btrfs/scrub/status?mount="+url.QueryEscape(p.Mount), &resp); err != nil {
			st.Status, st.Error = "unknown", err.Error()
			out = append(out, st)
			continue
		}
		info := pools.ParseScrubStatus(resp.Status)
		st.Status = info.Status
		if info.Status != "idle" {
			st.Progress, st.Errors = &info.Percent, &info.Errors
			if info.BytesTotal > 0 {
				st.BytesTotal, st.BytesScanned = &info.BytesTotal, &info.BytesScanned
			}
		}
		if info.StartedAt != nil {
			s := info.StartedAt.Format(time.RFC3339)
			st.StartedAt = &s
			if info.Status == "finished" {
				f := info.StartedAt.Add(info.Duration).Format(time.RFC3339)
				st.FinishedAt = &f
			}
		}
		out = append(out, st)
	}
	return out, true
}

func (m poolMaintenance) balanceStatus(w http.ResponseWriter, r *http.Request) {
	list, ok := m.pools(w, r, requestTarget(r))
	if !ok {
		return
	}
	out := make([]BalanceStatus, 0, len(list))
	for _, p := range list {
		st := BalanceStatus{PoolID: p.ID, Mount: p.Mount, Status: "idle"}
		var resp struct {
			Running bool    `json:"running"`
			Percent float64 `json:"percent"`
			Left    string  `json:"left"`
			Total   string  `json:"total"`
		}
		if err := m.agent.GetJSON(r.Context(), "/v1/btrfs/balance/status?mount="+url.QueryEscape(p.Mount), &resp); err != nil {
			st.Status, st.Error = "unknown", err.Error()
		} else if resp.Running {
			pct := int(resp.Percent)
			st.Status, st.Progress, st.Left, st.Total = "running", &pct, resp.Left, resp.Total
		}
		out = append(out, st)
	}
	writeJSON(w, out)
}

// opResult is one pool's outcome of a start or cancel.
type opResult struct {
	PoolID string `json:"poolId"`
	Mount  string `json:"mount"`
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
}

// op starts or cancels scrub/balance on the selected pools. The body may
// name one ({mount} or {poolId}); empty means every pool. A pool busy with a
// device operation is not started.
func (m poolMaintenance) op(agentPath string, start bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Mount  string `json:"mount"`
			PoolID string `json:"poolId"`
		}
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body) // empty body: every pool
		list, ok := m.pools(w, r, target{mount: body.Mount, poolID: body.PoolID})
		if !ok {
			return
		}
		results := make([]opResult, 0, len(list))
		anyOK, allConflict := false, len(list) > 0
		for _, p := range list {
			res := opResult{PoolID: p.ID, Mount: p.Mount}
			if start {
				if cur := currentPoolTx(p.Mount); cur != "" {
					res.Error = "pool is busy with operation " + cur
					results = append(results, res)
					continue
				}
			}
			err := m.agent.PostJSON(r.Context(), agentPath, map[string]string{"mount": p.Mount}, nil)
			var he *agentclient.HTTPError
			switch {
			case err == nil:
				res.OK, anyOK, allConflict = true, true, false
			case errors.As(err, &he):
				res.Error = strings.TrimSpace(he.Body)
				if he.Status != http.StatusConflict {
					allConflict = false
				}
			default:
				res.Error, allConflict = err.Error(), false
			}
			results = append(results, res)
		}
		status := http.StatusOK
		switch {
		case len(list) == 0:
			httpx.WriteTypedError(w, http.StatusNotFound, "pool.not_found", "No mounted pools", 0)
			return
		case anyOK:
		case allConflict:
			status = http.StatusConflict
		default:
			status = http.StatusBadGateway
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"results": results})
	}
}

// withRAIDProfiles fills in each pool's data profile (single, raid1, ...)
// from nos-agent: `btrfs filesystem usage` needs root. A pool the agent
// can't describe keeps an empty profile.
func withRAIDProfiles(ctx context.Context, agent AgentClient, list []pools.Pool) []pools.Pool {
	if agent == nil {
		return list
	}
	out := make([]pools.Pool, len(list))
	for i, p := range list {
		out[i] = p
		if p.Mount == "" || p.RAID != "" {
			continue
		}
		var usage struct {
			Classes map[string]struct {
				Profile string `json:"profile"`
			} `json:"classes"`
		}
		c, cancel := context.WithTimeout(ctx, 3*time.Second)
		err := agent.GetJSON(c, "/v1/btrfs/usage?mount="+url.QueryEscape(p.Mount), &usage)
		cancel()
		if err == nil {
			out[i].RAID = strings.ToLower(usage.Classes["data"].Profile)
		}
	}
	return out
}
