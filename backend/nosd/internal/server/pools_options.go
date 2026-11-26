package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"nos/backend/nosd/internal/config"
	"nos/backend/nosd/internal/fsatomic"
	"nos/backend/nosd/internal/pools"
	"nos/backend/nosd/pkg/agentclient"
	"nos/backend/nosd/pkg/httpx"
)

type poolOptionsRecord struct {
	Mount        string   `json:"mount"`
	MountOptions string   `json:"mountOptions"`
	Devices      []string `json:"devices,omitempty"`
}

type poolOptionsStore struct {
	Records []poolOptionsRecord `json:"records"`
}

func poolsStorePath(cfg config.Config) string { return filepath.Join(cfg.EtcDir, "nos", "pools.json") }

func loadPoolOptions(cfg config.Config) (poolOptionsStore, error) {
	var st poolOptionsStore
	_, _ = fsatomic.LoadJSON(poolsStorePath(cfg), &st)
	return st, nil
}

// updatePoolOptions loads the pool options store, applies mutate and saves it,
// all under one exclusive lock. Never call it while already holding the
// pools.json lock: flock locks are per open file, so a nested lock from the
// same process blocks forever.
func updatePoolOptions(cfg config.Config, mutate func(st *poolOptionsStore)) error {
	return fsatomic.WithLock(poolsStorePath(cfg), func() error {
		var st poolOptionsStore
		if _, err := fsatomic.LoadJSON(poolsStorePath(cfg), &st); err != nil {
			return err
		}
		mutate(&st)
		return fsatomic.SaveJSON(poolsStorePath(cfg), st, 0o600)
	})
}

func findPoolMountByID(r *http.Request, id string) (string, error) {
	// Allow passing mount path directly
	if strings.HasPrefix(id, "/") {
		return id, nil
	}
	list, err := pools.ListPools(r.Context())
	if err != nil {
		return "", err
	}
	id = strings.TrimSpace(id)
	for _, p := range list {
		if p.Mount == id || p.UUID == id || p.ID == id {
			if p.Mount != "" {
				return p.Mount, nil
			}
		}
	}
	return "", fmt.Errorf("not found")
}

func handlePoolOptionsGet(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			httpx.WriteError(w, http.StatusBadRequest, "id required")
			return
		}
		mount, err := findPoolMountByID(r, id)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				httpx.WriteError(w, http.StatusNotFound, "not found")
			} else {
				httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		st, _ := loadPoolOptions(cfg)
		opts := ""
		for _, rec := range st.Records {
			if rec.Mount == mount {
				opts = rec.MountOptions
				break
			}
		}
		if opts == "" {
			// default conservative
			opts = "compress=zstd:3,noatime"
		}
		writeJSON(w, map[string]any{"mountOptions": opts})
	}
}

type invalidTokenError struct{ token string }

func (e invalidTokenError) Error() string { return "invalid token: " + e.token }

func validateMountOptions(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("mountOptions required")
	}
	// tokens
	for _, raw := range strings.Split(s, ",") {
		tok := strings.TrimSpace(raw)
		if tok == "" {
			continue
		}
		lower := strings.ToLower(tok)
		if lower == "nodatacow" {
			return invalidTokenError{token: "nodatacow"}
		}
		if strings.HasPrefix(lower, "compress=") {
			val := strings.TrimPrefix(lower, "compress=")
			if !strings.HasPrefix(val, "zstd") {
				return invalidTokenError{token: "compress"}
			}
			if strings.Contains(val, ":") {
				parts := strings.SplitN(val, ":", 2)
				if len(parts) == 2 {
					lvl, err := strconv.Atoi(parts[1])
					if err != nil || lvl < 1 || lvl > 15 {
						return invalidTokenError{token: "compress"}
					}
				}
			}
			continue
		}
		if lower == "ssd" || lower == "noatime" || lower == "nodiratime" || lower == "autodefrag" || lower == "discard" || lower == "discard=async" {
			continue
		}
		return invalidTokenError{token: tok}
	}
	return nil
}

// test seam for remount
var remountFunc = func(r *http.Request, mount string, opts string) error {
	client := agentclient.New("/run/nos-agent.sock")
	// run: mount -o remount,<opts> <mount>
	var resp map[string]any
	err := client.PostJSON(r.Context(), "/v1/run", map[string]any{
		"steps": []map[string]any{{"cmd": "mount", "args": []string{"-o", "remount," + opts, mount}}},
	}, &resp)
	return err
}

func handlePoolOptionsPost(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			httpx.WriteError(w, http.StatusBadRequest, "id required")
			return
		}
		var body struct {
			MountOptions string `json:"mountOptions"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if err := validateMountOptions(body.MountOptions); err != nil {
			switch e := err.(type) {
			case invalidTokenError:
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnprocessableEntity)
				if err := json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "mount.options.invalid", "message": "invalid mount option", "details": map[string]any{"token": e.token}}}); err != nil {
					fmt.Printf("Failed to write error response: %v\n", err)
				}
				return
			default:
				httpx.WriteError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		mount, err := findPoolMountByID(r, id)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				httpx.WriteError(w, http.StatusNotFound, "not found")
			} else {
				httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		old := ""
		_ = updatePoolOptions(cfg, func(st *poolOptionsStore) {
			for i := range st.Records {
				if st.Records[i].Mount == mount {
					old = st.Records[i].MountOptions
					st.Records[i].MountOptions = body.MountOptions
					return
				}
			}
			st.Records = append(st.Records, poolOptionsRecord{Mount: mount, MountOptions: body.MountOptions})
		})

		// Try remount; if fails, update fstab and require reboot
		rebootRequired := false
		if err := remountFunc(r, mount, body.MountOptions); err != nil {
			rebootRequired = true
			client := agentclient.New("/run/nos-agent.sock")
			_ = client.PostJSON(r.Context(), "/v1/fstab/remove", map[string]any{"contains": mount}, nil)
			line := "UUID=<uuid> " + mount + " btrfs " + body.MountOptions + " 0 0"
			_ = client.PostJSON(r.Context(), "/v1/fstab/ensure", map[string]any{"line": line}, nil)
		}

		// Log structured event
		Logger(cfg).Info().
			Str("event", "pool.options.updated").
			Str("mount", mount).
			Str("old", old).
			Str("new", body.MountOptions).
			Bool("requiresReboot", rebootRequired).
			Msg("")

		writeJSON(w, map[string]any{"ok": true, "mountOptions": body.MountOptions, "rebootRequired": rebootRequired, "updatedAt": time.Now().UTC().Format(time.RFC3339)})
	}
}
