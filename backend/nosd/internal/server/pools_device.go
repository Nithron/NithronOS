package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"nithronos/backend/nosd/internal/config"
	"nithronos/backend/nosd/internal/disks"
	"nithronos/backend/nosd/internal/fsatomic"
	"nithronos/backend/nosd/internal/pools"
	btrfsplan "nithronos/backend/nosd/internal/storage/btrfs"
	"nithronos/backend/nosd/pkg/agentclient"
	"nithronos/backend/nosd/pkg/httpx"
)

var agentSocketPath = "/run/nos-agent.sock"

// seam for tests
var devicePollInterval = 3 * time.Second

// txWorkers tracks running pool device transactions (tests wait on it so a
// transaction never outlives the test that started it).
var txWorkers sync.WaitGroup

type agentAPI interface {
	PostJSON(ctx context.Context, path string, body any, v any) error
	BalanceStatus(ctx context.Context, mount string) (*agentclient.BalanceStatus, error)
	ReplaceStatus(ctx context.Context, mount string) (*agentclient.ReplaceStatus, error)
}

var makeAgentClient = func() agentAPI { return agentclient.New(agentSocketPath) }

// in-process gauges for last observed progress
var (
	progressMu          = &sync.Mutex{}
	gaugeBalancePercent = -1.0
	gaugeReplacePercent = -1.0
)

func currentBalancePercent() float64 {
	progressMu.Lock()
	defer progressMu.Unlock()
	return gaugeBalancePercent
}
func currentReplacePercent() float64 {
	progressMu.Lock()
	defer progressMu.Unlock()
	return gaugeReplacePercent
}
func setBalancePercent(v float64) { progressMu.Lock(); gaugeBalancePercent = v; progressMu.Unlock() }
func setReplacePercent(v float64) { progressMu.Lock(); gaugeReplacePercent = v; progressMu.Unlock() }

// POST /api/v1/pools/{id}/plan-device
func handlePlanDevice(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			httpx.WriteError(w, http.StatusBadRequest, "id required")
			return
		}
		var req btrfsplan.DevicePlanRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		// Discover current pool facts
		list, _ := pools.ListPools(r.Context())
		var mount string
		for _, p := range list {
			if p.ID == id || p.UUID == id || p.Mount == id {
				mount = p.Mount
				break
			}
		}
		if mount == "" {
			httpx.WriteError(w, http.StatusNotFound, "pool not found")
			return
		}
		// Build device sizes and existing devices from lsblk
		devList, _ := disks.Collect(r.Context())
		devSizes := map[string]int64{}
		existing := []string{}
		for _, d := range devList {
			devSizes[d.Path] = d.SizeBytes
			if d.Mountpoint != nil && *d.Mountpoint == mount {
				existing = append(existing, d.Path)
			}
		}
		// Get current profiles via agent: btrfs filesystem usage <mount>
		dataProf, metaProf := "", ""
		{
			client := makeAgentClient()
			var resp struct{ Results []struct{ Stdout string } }
			_ = client.PostJSON(r.Context(), "/v1/run", map[string]any{"steps": []map[string]any{{"cmd": "btrfs", "args": []string{"filesystem", "usage", mount}}}}, &resp)
			if len(resp.Results) > 0 {
				dataProf, metaProf = parseProfiles(resp.Results[0].Stdout)
			}
		}
		if dataProf == "" {
			dataProf = "raid1"
		}
		if metaProf == "" {
			metaProf = dataProf
		}
		planner := btrfsplan.Planner{PoolMount: mount, ExistingDevices: existing, CurrentProfileData: dataProf, CurrentProfileMeta: metaProf, DeviceSizes: devSizes}
		plan, err := planner.Plan(req)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, map[string]any{"planId": plan.PlanID, "steps": plan.Steps, "warnings": plan.Warnings, "requiresBalance": plan.RequiresBalance})
	}
}

// POST /api/v1/pools/{id}/apply-device
func handleApplyDevice(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			httpx.WriteError(w, http.StatusBadRequest, "id required")
			return
		}
		// Pool busy check
		if cur := currentPoolTx(id); cur != "" {
			httpx.WriteError(w, http.StatusConflict, `{"error":{"code":"pool.busy","txId":"`+cur+`"}}`)
			return
		}
		var body struct {
			Steps   []struct{ ID, Description, Command string }
			Confirm string `json:"confirm"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid json")
			return
		}
		if len(body.Steps) == 0 {
			httpx.WriteError(w, http.StatusBadRequest, "no steps")
			return
		}
		// Determine action and validate confirm
		expected := ""
		for _, st := range body.Steps {
			c := strings.ToLower(st.Command)
			if strings.Contains(c, "btrfs device add ") {
				expected = "ADD"
				break
			}
			if strings.Contains(c, "btrfs device remove ") {
				expected = "REMOVE"
				break
			}
			if strings.Contains(c, "btrfs replace start ") {
				expected = "REPLACE"
				break
			}
		}
		if expected == "" {
			httpx.WriteError(w, http.StatusBadRequest, "unable to infer action from steps")
			return
		}
		if strings.ToUpper(strings.TrimSpace(body.Confirm)) != expected {
			httpx.WriteError(w, http.StatusPreconditionRequired, "confirm="+expected+" required")
			return
		}
		mount := ""
		for _, st := range body.Steps {
			parts := strings.Fields(st.Command)
			if len(parts) >= 2 && strings.ToLower(parts[0]) == "btrfs" {
				mount = parts[len(parts)-1]
				break
			}
		}

		// Create tx and save
		tx := pools.Tx{ID: generateUUID(), StartedAt: time.Now().UTC()}
		for _, st := range body.Steps {
			tx.Steps = append(tx.Steps, pools.TxStep{ID: st.ID, Name: st.Description, Cmd: st.Command, Destructive: strings.Contains(st.Command, " device "), Status: "pending"})
		}
		_ = saveTx(tx)
		// Acquire per-pool lock; if fails, report busy
		if !tryAcquirePoolLock(id, tx.ID) {
			httpx.WriteError(w, http.StatusConflict, `{"error":{"code":"pool.busy","txId":"`+currentPoolTx(id)+`"}}`)
			return
		}
		// Metrics + log started
		action := strings.ToLower(expected)
		Logger(cfg).Info().Str("event", "pool.device."+action+".started").Str("txId", tx.ID).Strs("devices", extractDevices(body.Steps)).Msg("")
		incBtrfsTx(action)
		start := time.Now()
		// Execute asynchronously. Everything the transaction needs is taken
		// now, not when the goroutine runs, and the pool lock is released when
		// the transaction ends.
		client := makeAgentClient()
		pollEvery := devicePollInterval
		txWorkers.Add(1)
		go func(txID, poolID string) {
			defer txWorkers.Done()
			defer releasePoolLock(poolID)
			fn := func() error {
				var cur pools.Tx
				_, _ = fsatomic.LoadJSON(txPath(txID), &cur)
				for i, st := range cur.Steps {
					now := time.Now().UTC()
					cur.Steps[i].Status = "running"
					cur.Steps[i].StartedAt = &now
					_ = saveTx(cur)
					parts := strings.Fields(st.Cmd)
					var resp struct {
						Results []struct {
							Code   int
							Stdout string
						}
					}
					_ = client.PostJSON(context.TODO(), "/v1/run", map[string]any{"steps": []map[string]any{{"cmd": parts[0], "args": parts[1:]}}}, &resp)
					code := 0
					out := ""
					if len(resp.Results) > 0 {
						code = resp.Results[0].Code
						out = resp.Results[0].Stdout
					} else {
						// Treat a missing result as error to ensure the tx terminates in tests
						code = 1
					}
					if code != 0 {
						cur.OK = false
						cur.Error = "step failed"
						done := time.Now().UTC()
						cur.Steps[i].Status = "error"
						cur.Steps[i].FinishedAt = &done
						cur.FinishedAt = &done
						_ = saveTx(cur)
						appendTxLog(cur.ID, "error", st.ID, out)
						return nil
					}
					done := time.Now().UTC()
					cur.Steps[i].Status = "ok"
					cur.Steps[i].FinishedAt = &done
					_ = saveTx(cur)
					appendTxLog(cur.ID, "info", st.ID, out)
					// Poll structured status endpoints instead of parsing /v1/run output
					if mount == "" {
						mount = strings.TrimSpace(strings.Split(st.Cmd, " ")[len(strings.Split(st.Cmd, " "))-1])
					}
					if strings.Contains(st.Cmd, "balance start") {
						if os.Getenv("NOS_TEST_FAST_POLL") == "1" {
							entry := map[string]any{"event": "balance", "percent": 100}
							b, _ := json.Marshal(entry)
							appendTxLog(cur.ID, "info", st.ID, string(b))
							setBalancePercent(-1)
							clearBtrfsBalanceProgress()
						} else {
							for j := 0; j < 10; j++ {
								bs, _ := client.BalanceStatus(context.TODO(), mount)
								if bs != nil {
									entry := map[string]any{"event": "balance", "percent": bs.Percent}
									if bs.Left != nil {
										entry["left"] = *bs.Left
									}
									if bs.Total != nil {
										entry["total"] = *bs.Total
									}
									b, _ := json.Marshal(entry)
									appendTxLog(cur.ID, "info", st.ID, string(b))
									setBalancePercent(bs.Percent)
									setBtrfsBalanceProgress(bs.Percent)
									if !bs.Running || bs.Percent >= 100 {
										setBalancePercent(-1)
										clearBtrfsBalanceProgress()
										break
									}
								}
								time.Sleep(pollEvery)
							}
						}
					}
					if strings.Contains(st.Cmd, "replace start") {
						if os.Getenv("NOS_TEST_FAST_POLL") == "1" {
							entry := map[string]any{"event": "replace", "percent": 100}
							b, _ := json.Marshal(entry)
							appendTxLog(cur.ID, "info", st.ID, string(b))
							setReplacePercent(-1)
						} else {
							for j := 0; j < 10; j++ {
								rs, _ := client.ReplaceStatus(context.TODO(), mount)
								if rs != nil {
									entry := map[string]any{"event": "replace", "percent": rs.Percent}
									if rs.Completed != nil {
										entry["completed"] = *rs.Completed
									}
									if rs.Total != nil {
										entry["total"] = *rs.Total
									}
									b, _ := json.Marshal(entry)
									appendTxLog(cur.ID, "info", st.ID, string(b))
									setReplacePercent(rs.Percent)
									if !rs.Running || rs.Percent >= 100 {
										setReplacePercent(-1)
										break
									}
								}
								time.Sleep(pollEvery)
							}
						}
					}
				}
				cur.OK = true
				now := time.Now().UTC()
				cur.FinishedAt = &now
				_ = saveTx(cur)
				// Post-success: best-effort refresh device list for this pool
				if mount != "" {
					devList, _ := disks.Collect(context.TODO())
					devices := []string{}
					for _, d := range devList {
						if d.Mountpoint != nil && *d.Mountpoint == mount {
							devices = append(devices, d.Path)
						}
					}
					_ = updatePoolOptions(cfg, func(st *poolOptionsStore) {
						for i := range st.Records {
							if st.Records[i].Mount == mount {
								st.Records[i].Devices = devices
								return
							}
						}
						st.Records = append(st.Records, poolOptionsRecord{Mount: mount, Devices: devices})
					})
				}
				return nil
			}
			// fn runs the whole transaction (possibly minutes of balance/replace
			// polling), so it must not hold the pools.json lock; the store update at
			// the end takes the lock itself via updatePoolOptions.
			_ = fn()
			// Metrics + log finished
			observeBtrfsTxDuration(start)
			Logger(cfg).Info().Str("event", "pool.device."+action+".finished").Str("txId", tx.ID).Strs("devices", extractDevices(body.Steps)).Msg("")
		}(tx.ID, id)
		writeJSON(w, map[string]any{"ok": true, "tx_id": tx.ID})
	}
}

func parseProfiles(out string) (data string, meta string) {
	s := strings.ToLower(out)
	for _, line := range strings.Split(s, "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "data,") {
			if strings.Contains(l, "raid10") {
				data = "raid10"
			} else if strings.Contains(l, "raid1") {
				data = "raid1"
			} else if strings.Contains(l, "single") {
				data = "single"
			}
		}
		if strings.HasPrefix(l, "metadata,") {
			if strings.Contains(l, "raid10") {
				meta = "raid10"
			} else if strings.Contains(l, "raid1") {
				meta = "raid1"
			} else if strings.Contains(l, "single") {
				meta = "single"
			}
		}
	}
	return
}

func extractDevices(steps []struct{ ID, Description, Command string }) []string {
	out := []string{}
	for _, s := range steps {
		c := strings.ToLower(s.Command)
		if strings.Contains(c, "btrfs device add") || strings.Contains(c, "btrfs device remove") || strings.Contains(c, "btrfs replace start") {
			parts := strings.Fields(s.Command)
			for _, p := range parts {
				if strings.HasPrefix(p, "/dev/") {
					out = append(out, p)
				}
			}
		}
	}
	return out
}
