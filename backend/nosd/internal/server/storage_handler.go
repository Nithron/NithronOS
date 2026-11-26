package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"nos/backend/nosd/internal/pools"
	"nos/backend/nosd/pkg/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// Pool represents a storage pool
type Pool struct {
	ID         string       `json:"id"`
	UUID       string       `json:"uuid"`
	Label      string       `json:"label,omitempty"`
	Mountpoint string       `json:"mountpoint"`
	Size       uint64       `json:"size"`
	Used       uint64       `json:"used"`
	Free       uint64       `json:"free"`
	Raid       string       `json:"raid"`
	Status     string       `json:"status"` // online, degraded, offline
	Devices    []PoolDevice `json:"devices"`
	Subvolumes []Subvolume  `json:"subvolumes,omitempty"`
}

// PoolDevice represents a device in a pool
type PoolDevice struct {
	Path   string `json:"path"`
	Size   uint64 `json:"size"`
	Used   uint64 `json:"used,omitempty"`
	Status string `json:"status,omitempty"`
}

// Subvolume represents a Btrfs subvolume
type Subvolume struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Gen      string `json:"gen,omitempty"`
	TopLevel string `json:"top_level,omitempty"`
}

// PoolSummary represents a summary of all pools
type PoolSummary struct {
	TotalPools    int    `json:"totalPools"`
	TotalSize     uint64 `json:"totalSize"`
	TotalUsed     uint64 `json:"totalUsed"`
	PoolsOnline   int    `json:"poolsOnline"`
	PoolsDegraded int    `json:"poolsDegraded"`
}

// Device represents a storage device
type Device struct {
	Path   string `json:"path"`
	Model  string `json:"model,omitempty"`
	Serial string `json:"serial,omitempty"`
	Size   uint64 `json:"size"`
	Type   string `json:"type,omitempty"` // ssd, hdd
	InUse  bool   `json:"inUse"`
	Pool   string `json:"pool,omitempty"`
}

// StorageHandler handles storage-related endpoints
type StorageHandler struct {
	agentClient AgentClient
}

// NewStorageHandler creates a new storage handler
func NewStorageHandler(agentClient AgentClient) *StorageHandler {
	return &StorageHandler{
		agentClient: agentClient,
	}
}

// Routes registers the storage routes
func (h *StorageHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Pool endpoints
	r.Get("/pools", h.GetPools)
	r.Get("/pools/{uuid}", h.GetPool)
	r.Get("/pools/{uuid}/subvols", h.GetPoolSubvolumes)

	// Device endpoints

	return r
}

// GetPools returns all storage pools or a summary
// GET /api/v1/storage/pools?summary=1
func (h *StorageHandler) GetPools(w http.ResponseWriter, r *http.Request) {
	summary := r.URL.Query().Get("summary") == "1"

	// Get pool data (in real implementation, this would use btrfs fi show, etc.)
	pools := h.getPools(r.Context())

	if summary {
		// Return summary
		poolSummary := PoolSummary{
			TotalPools:    len(pools),
			TotalSize:     0,
			TotalUsed:     0,
			PoolsOnline:   0,
			PoolsDegraded: 0,
		}

		for _, pool := range pools {
			poolSummary.TotalSize += pool.Size
			poolSummary.TotalUsed += pool.Used

			switch pool.Status {
			case "online":
				poolSummary.PoolsOnline++
			case "degraded":
				poolSummary.PoolsDegraded++
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(poolSummary); err != nil {
			fmt.Printf("Failed to write response: %v\n", err)
		}
	} else {
		// Return full pool list
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(pools); err != nil {
			fmt.Printf("Failed to write response: %v\n", err)
		}
	}
}

// GetPool returns a specific pool by UUID
// GET /api/v1/storage/pools/{uuid}
func (h *StorageHandler) GetPool(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")

	pools := h.getPools(r.Context())
	for _, pool := range pools {
		if pool.UUID == uuid || pool.ID == uuid {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(pool); err != nil {
				fmt.Printf("Failed to write response: %v\n", err)
			}
			return
		}
	}

	http.Error(w, "Pool not found", http.StatusNotFound)
}

// GetPoolSubvolumes lists a pool's subvolumes (the pool by id, UUID or
// mount point). GET /api/v1/storage/pools/{uuid}/subvols and
// /api/v1/pools/{id}/subvolumes.
func (h *StorageHandler) GetPoolSubvolumes(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "uuid")
	if key == "" {
		key = chi.URLParam(r, "id")
	}
	list, err := selectPools(r.Context(), target{poolID: key, mount: key})
	if errors.Is(err, errPoolNotFound) {
		httpx.WriteTypedError(w, http.StatusNotFound, "pool.not_found", "No mounted pool matches", 0)
		return
	}
	if err != nil {
		httpx.WriteTypedError(w, http.StatusServiceUnavailable, "pools.unavailable", "Could not list pools: "+err.Error(), 0)
		return
	}
	var out struct {
		Subvolumes []Subvolume `json:"subvolumes"`
	}
	if err := h.agentClient.GetJSON(r.Context(), "/v1/btrfs/subvolumes?mount="+url.QueryEscape(list[0].Mount), &out); err != nil {
		httpx.WriteTypedError(w, http.StatusBadGateway, "agent.failed", err.Error(), 0)
		return
	}
	if out.Subvolumes == nil {
		out.Subvolumes = []Subvolume{}
	}
	writeJSON(w, out.Subvolumes)
}

// getPools lists the Btrfs pools found on this machine. (It used to return a
// made-up 2 TB RAID1 pool.)
func (h *StorageHandler) getPools(ctx context.Context) []Pool {
	list, err := pools.ListPools(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("list pools")
		return []Pool{}
	}
	out := make([]Pool, 0, len(list))
	for _, p := range list {
		pool := Pool{ID: p.ID, UUID: p.UUID, Label: p.Label, Mountpoint: p.Mount, Size: p.Size, Used: p.Used, Free: p.Free, Raid: p.RAID, Status: "online"}
		if p.Mount == "" {
			pool.Status = "offline"
		}
		for _, d := range p.Devices {
			pool.Devices = append(pool.Devices, PoolDevice{Path: d})
		}
		out = append(out, pool)
	}
	return out
}
