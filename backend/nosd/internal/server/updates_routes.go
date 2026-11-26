package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"nos/backend/nosd/internal/updates"
	"nos/backend/nosd/pkg/httpx"
)

// registerUpdates serves /api/v1/updates/* (the Updates page) and the older
// /api/v1/snapshots/* list of update snapshots, from one updates.Service.
// (There used to be an inline implementation, an UpdatesHandler that asked a
// GitHub repository that doesn't exist and ran apt-get as the nos user, and
// pkg/updates, which nothing used.)
func (d *routeDeps) registerUpdates(pr chi.Router) {
	svc := d.updates
	if svc == nil {
		pr.Mount("/api/v1/updates", unavailable("updates.unavailable", "Updates are unavailable: the update state could not be loaded (see the nosd log)"))
		return
	}
	admin := d.adminRequired

	pr.Get("/api/v1/updates/version", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, svc.Version()) })
	pr.Get("/api/v1/updates/channel", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"channel": svc.Channel()})
	})
	pr.With(admin).Post("/api/v1/updates/channel", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Channel updates.Channel `json:"channel"`
		}
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body)
		if err := svc.SetChannel(body.Channel); err != nil {
			httpx.WriteTypedError(w, http.StatusBadRequest, "updates.invalid_channel", err.Error(), 0)
			return
		}
		writeJSON(w, map[string]any{"status": "ok", "channel": body.Channel})
	})

	check := func(w http.ResponseWriter, r *http.Request) {
		res, err := svc.Check(r.Context())
		if err != nil {
			httpx.WriteTypedError(w, http.StatusBadGateway, "updates.check_failed", "Could not check for updates: "+err.Error(), 0)
			return
		}
		writeJSON(w, res)
	}
	pr.With(admin).Get("/api/v1/updates/check", check)
	pr.With(admin).Post("/api/v1/updates/check", check)

	pr.With(admin).Post("/api/v1/updates/apply", func(w http.ResponseWriter, r *http.Request) {
		var req updates.ApplyRequest
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req) // empty body: upgrade everything
		if err := svc.Apply(req); errors.Is(err, updates.ErrBusy) {
			httpx.WriteTypedError(w, http.StatusConflict, "updates.busy", err.Error(), 0)
			return
		} else if err != nil {
			httpx.WriteTypedError(w, http.StatusInternalServerError, "updates.apply_failed", err.Error(), 0)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "started", "message": "Update started; follow /api/v1/updates/progress"})
	})

	pr.Get("/api/v1/updates/progress", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, svc.Progress()) })
	pr.Get("/api/v1/updates/progress/stream", func(w http.ResponseWriter, r *http.Request) { streamUpdateProgress(w, r, svc) })

	pr.With(admin).Post("/api/v1/updates/rollback", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			SnapshotID string `json:"snapshot_id"`
			TxID       string `json:"tx_id"` // older name
		}
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body)
		id := body.SnapshotID
		if id == "" {
			id = body.TxID
		}
		switch err := svc.Rollback(r.Context(), id); {
		case errors.Is(err, updates.ErrNotFound):
			httpx.WriteTypedError(w, http.StatusNotFound, "updates.snapshot_not_found", "No such update snapshot", 0)
		case errors.Is(err, updates.ErrBusy):
			httpx.WriteTypedError(w, http.StatusConflict, "updates.busy", err.Error(), 0)
		case err != nil:
			httpx.WriteTypedError(w, http.StatusBadGateway, "updates.rollback_failed", err.Error(), 0)
		default:
			writeJSON(w, map[string]any{"status": "rolled_back", "message": "Configuration restored; nosd restarts to load it (packages were not downgraded)"})
		}
	})

	pr.Get("/api/v1/updates/snapshots", func(w http.ResponseWriter, r *http.Request) {
		list, err := svc.Snapshots()
		if err != nil {
			httpx.WriteTypedError(w, http.StatusInternalServerError, "updates.records_unreadable", err.Error(), 0)
			return
		}
		writeJSON(w, map[string]any{"snapshots": list, "total": len(list)})
	})
	pr.With(admin).Delete("/api/v1/updates/snapshots/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := svc.DeleteSnapshot(r.Context(), id); errors.Is(err, updates.ErrNotFound) {
			httpx.WriteTypedError(w, http.StatusNotFound, "updates.snapshot_not_found", "No such update snapshot", 0)
			return
		} else if err != nil {
			httpx.WriteTypedError(w, http.StatusBadGateway, "updates.delete_failed", err.Error(), 0)
			return
		}
		writeJSON(w, map[string]any{"status": "deleted", "snapshot_id": id})
	})

	// The update records (snapshots and rollbacks), as nos-client lists them.
	pr.Get("/api/v1/snapshots/recent", func(w http.ResponseWriter, r *http.Request) {
		list, err := svc.Records()
		if err != nil {
			httpx.WriteTypedError(w, http.StatusInternalServerError, "updates.records_unreadable", err.Error(), 0)
			return
		}
		if len(list) > 20 {
			list = list[:20]
		}
		writeJSON(w, list)
	})
	pr.Get("/api/v1/snapshots/{tx_id}", func(w http.ResponseWriter, r *http.Request) {
		rec, err := svc.Record(chi.URLParam(r, "tx_id"))
		if err != nil {
			httpx.WriteTypedError(w, http.StatusNotFound, "updates.snapshot_not_found", "No such update snapshot", 0)
			return
		}
		writeJSON(w, rec)
	})
	pr.With(admin).Post("/api/v1/snapshots/prune", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			KeepPerTarget int `json:"keep_per_target"`
		}
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body)
		if body.KeepPerTarget <= 0 {
			body.KeepPerTarget = 5
		}
		deleted, err := svc.Prune(r.Context(), body.KeepPerTarget)
		if err != nil {
			httpx.WriteTypedError(w, http.StatusBadGateway, "updates.prune_failed", err.Error(), 0)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "deleted": deleted})
	})
}

// streamUpdateProgress sends the progress as server-sent events: the current
// state at once, then every change, with a comment every 15 s to keep proxies
// from closing the stream.
func streamUpdateProgress(w http.ResponseWriter, r *http.Request, svc *updates.Service) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "stream.unsupported", "Streaming is not supported", 0)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	send := func(p updates.Progress) bool {
		b, _ := json.Marshal(p)
		if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	ch, stop := svc.Subscribe()
	defer stop()
	if !send(svc.Progress()) {
		return
	}
	keepAlive := time.NewTicker(15 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case p := <-ch:
			if !send(p) {
				return
			}
		case <-keepAlive.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
