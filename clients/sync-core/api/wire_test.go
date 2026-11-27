package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"nos/clients/sync-core/config"
)

// The client decodes what nosd sends (JSON as backend/nosd/pkg/sync/types.go
// and httpx.WriteTypedError write it).
func TestDecodesNosdResponses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/sync/shares":
			_, _ = w.Write([]byte(`{"shares":[{"share_id":"s1","share_name":"docs","share_path":"/srv/shares/docs","sync_enabled":true,"total_size":5,"file_count":1}],"count":1}`))
		case "/api/v1/sync/changes":
			_, _ = w.Write([]byte(`{"changes":[{"path":"a/b.txt","type":"rename","size":3,"mtime":"2026-11-27T10:00:00Z","hash":"ab","old_path":"b.txt","is_dir":false}],"cursor":"c2","has_more":false}`))
		case "/api/v1/sync/files/s1/hash":
			_, _ = w.Write([]byte(`{"path":"x","size":10,"hash":"ff","block_size":4194304,"blocks":[{"offset":0,"size":10,"hash":"aa","weak":7}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"share.not_found","message":"Share not found"}}`))
		}
	}))
	defer srv.Close()
	cfg, err := config.LoadFrom(filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatal(err)
	}
	_ = cfg.Update(func(c *config.Config) { c.ServerURL, c.DeviceToken = srv.URL, "nos_dt_x" })
	c, ctx := NewClient(cfg), context.Background()

	shares, err := c.ListShares(ctx)
	if err != nil || len(shares) != 1 || shares[0].ID != "s1" || shares[0].Name != "docs" {
		t.Fatalf("shares %+v %v", shares, err)
	}
	ch, err := c.GetChanges(ctx, "s1", "", 10)
	if err != nil || len(ch.Changes) != 1 || ch.Cursor != "c2" {
		t.Fatalf("changes %+v %v", ch, err)
	}
	if got := ch.Changes[0]; got.Type != ChangeRename || got.OldPath != "b.txt" || got.Hash != "ab" || got.MTime.IsZero() {
		t.Fatalf("change %+v", got)
	}
	bh, err := c.GetBlockHashes(ctx, "s1", "x", 0)
	if err != nil || bh.BlockSize != 4194304 || len(bh.Blocks) != 1 || bh.Blocks[0].StrongHash != "aa" || bh.Blocks[0].WeakHash != 7 {
		t.Fatalf("blocks %+v %v", bh, err)
	}
	var apiErr *APIError
	if err := c.UploadDelta(ctx, "nope", "x", "00", nil); !errors.As(err, &apiErr) || apiErr.Code != "share.not_found" || apiErr.Status != 404 {
		t.Fatalf("error %v", err)
	}
}
