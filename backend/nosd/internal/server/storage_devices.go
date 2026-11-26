package server

import (
	"context"
	"net/http"
	"time"

	"nos/backend/nosd/internal/storage/blk"
)

type deviceDTO struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Size        uint64   `json:"size"`
	Model       string   `json:"model,omitempty"`
	Serial      string   `json:"serial,omitempty"`
	Rota        *bool    `json:"rota,omitempty"`
	FsType      string   `json:"fsType,omitempty"`
	BtrfsMember bool     `json:"btrfsMember"`
	LUKS        bool     `json:"luks"`
	Warnings    []string `json:"warnings"`
}

func handleListDevices(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	ds, err := blk.ListCandidates(ctx)
	if err != nil {
		writeJSON(w, map[string]any{"devices": []any{}, "error": err.Error()})
		return
	}
	out := make([]deviceDTO, 0, len(ds))
	for _, d := range ds {
		out = append(out, deviceDTO{
			Name:        d.Name,
			Path:        d.Path,
			Size:        d.SizeBytes,
			Model:       d.Model,
			Serial:      d.Serial,
			Rota:        d.Rota,
			FsType:      d.FSType,
			BtrfsMember: d.BtrfsMember,
			LUKS:        d.LUKS,
			Warnings:    d.Warnings,
		})
	}
	writeJSON(w, map[string]any{"devices": out})
}
