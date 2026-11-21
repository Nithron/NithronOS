package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"nithronos/backend/nosd/internal/config"
	"nithronos/backend/nosd/pkg/agentclient"
	"nithronos/backend/nosd/pkg/httpx"
)

// handleBalanceStatus returns the status of a BTRFS balance operation
func handleBalanceStatus(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		poolID := r.URL.Query().Get("pool_id")
		mountPath := r.URL.Query().Get("mount_path")

		status := map[string]any{
			"running":    false,
			"pool_id":    poolID,
			"mount_path": mountPath,
		}

		if mountPath != "" {
			// Try to get status from agent
			agentSocket := "/run/nos-agent.sock"
			agent := agentclient.New(agentSocket)
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()

			req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://unix/v1/btrfs/balance/status?mount=%s", mountPath), nil)
			if resp, err := agent.HTTP.Do(req); err == nil && resp.StatusCode == 200 {
				defer resp.Body.Close()
				var agentStatus map[string]any
				if json.NewDecoder(resp.Body).Decode(&agentStatus) == nil {
					// Merge agent response
					for k, v := range agentStatus {
						status[k] = v
					}
				}
			}
		}

		writeJSON(w, status)
	}
}

// handleBalanceStart initiates a BTRFS balance operation
func handleBalanceStart(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			PoolID    string `json:"pool_id"`
			MountPath string `json:"mount_path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httpx.WriteTypedError(w, http.StatusBadRequest, "invalid.json", "Invalid request body", 0)
			return
		}

		mountPath := body.MountPath
		if mountPath == "" {
			httpx.WriteTypedError(w, http.StatusBadRequest, "mount.required", "Mount path is required", 0)
			return
		}

		// Create a job for this operation
		job := CreateJob("balance", fmt.Sprintf("Starting balance on %s", mountPath), map[string]any{
			"pool_id":    body.PoolID,
			"mount_path": mountPath,
		})

		// TODO: Start balance via agent
		StartJob(job.ID)

		writeJSON(w, map[string]any{
			"status":  "started",
			"message": fmt.Sprintf("Balance started on %s", mountPath),
			"job_id":  job.ID,
		})
	}
}

// handleBalanceCancel cancels a running BTRFS balance operation
func handleBalanceCancel(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			PoolID    string `json:"pool_id"`
			MountPath string `json:"mount_path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httpx.WriteTypedError(w, http.StatusBadRequest, "invalid.json", "Invalid request body", 0)
			return
		}

		mountPath := body.MountPath
		if mountPath == "" {
			httpx.WriteTypedError(w, http.StatusBadRequest, "mount.required", "Mount path is required", 0)
			return
		}

		// TODO: Cancel balance via agent
		writeJSON(w, map[string]any{
			"status":  "cancelled",
			"message": fmt.Sprintf("Balance cancelled on %s", mountPath),
		})
	}
}
