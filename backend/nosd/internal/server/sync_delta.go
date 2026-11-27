package server

// POST /api/v1/sync/files/{share_id}/delta: a sync client uploads only what
// changed in a file. The body lists operations that rebuild the new file
// from the current one: copy a range of the current file, or new bytes.
// nosd writes the result next to the file, checks its SHA-256 against the
// client's, and only then replaces the file (atomically). (Clients always
// uploaded whole files: the server had no way to take a delta.)

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"

	"nos/backend/nosd/pkg/httpx"
	nosync "nos/backend/nosd/pkg/sync"
)

// maxDeltaBody caps a delta request; a client with more new data uploads
// the whole file instead.
const maxDeltaBody = 64 << 20

// DeltaOp is one step: copy Size bytes at Offset of the current file, or
// append Data.
type DeltaOp struct {
	Offset int64  `json:"offset,omitempty"`
	Size   int64  `json:"size,omitempty"`
	Data   []byte `json:"data,omitempty"` // base64 in JSON
}

// DeltaUploadRequest rebuilds Path; SHA256 is what the result must hash to.
type DeltaUploadRequest struct {
	Path   string    `json:"path"`
	SHA256 string    `json:"sha256"`
	Ops    []DeltaOp `json:"ops"`
}

// writableShare is a share the caller may change (an owner).
func (h *SyncHandler) writableShare(r *http.Request, shareID string) (bool, string) {
	sh, ok := h.shareStore.Get(shareID)
	if !ok {
		return false, ""
	}
	if _, write := h.access.ShareAccess(sh, r.Header.Get("X-Device-User-ID")); !write {
		return false, ""
	}
	return true, sh.Path
}

// UploadDelta handles POST /sync/files/{share_id}/delta.
func (h *SyncHandler) UploadDelta(w http.ResponseWriter, r *http.Request) {
	ok, sharePath := h.writableShare(r, chi.URLParam(r, "share_id"))
	if !ok {
		httpx.WriteTypedError(w, http.StatusNotFound, "share.not_found", "Share not found", 0)
		return
	}
	var req DeltaUploadRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxDeltaBody)).Decode(&req); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "Invalid request body (at most 64 MiB; upload the whole file instead)", 0)
		return
	}
	want, err := hex.DecodeString(req.SHA256)
	if err != nil || len(want) != sha256.Size {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "sha256 must be the result's SHA-256 in hex", 0)
		return
	}
	target, err := nosync.ResolveInShare(sharePath, req.Path)
	if err != nil || target == sharePath {
		httpx.WriteTypedError(w, http.StatusBadRequest, "input.invalid", "path is outside the share", 0)
		return
	}
	size, err := applyDelta(target, req.Ops, want)
	switch {
	case errors.Is(err, errDeltaMismatch):
		httpx.WriteTypedError(w, http.StatusConflict, "sync.delta_mismatch", "The rebuilt file doesn't match: the file changed on the server; upload the whole file", 0)
		return
	case errors.Is(err, os.ErrNotExist):
		httpx.WriteTypedError(w, http.StatusNotFound, "sync.no_base", "There is no current file to apply a delta to", 0)
		return
	case err != nil:
		h.logger.Error().Err(err).Str("path", req.Path).Msg("delta upload")
		httpx.WriteTypedError(w, http.StatusBadRequest, "sync.delta_invalid", err.Error(), 0)
		return
	}
	writeJSON(w, map[string]any{"path": req.Path, "size": size, "sha256": req.SHA256})
}

var errDeltaMismatch = errors.New("the rebuilt file's hash doesn't match")

// applyDelta rebuilds target from its current content and ops into a
// temporary file in the same folder, checks the hash, and renames it over
// target. On any error target is unchanged.
func applyDelta(target string, ops []DeltaOp, want []byte) (int64, error) {
	base, err := os.Open(target)
	if err != nil {
		return 0, err
	}
	defer base.Close()
	info, err := base.Stat()
	if err != nil {
		return 0, err
	}
	if !info.Mode().IsRegular() {
		return 0, fmt.Errorf("not a regular file")
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".nos-delta-*")
	if err != nil {
		return 0, err
	}
	defer os.Remove(tmp.Name()) // no-op after the rename
	h := sha256.New()
	out := io.MultiWriter(tmp, h)
	var written int64
	for i, op := range ops {
		switch {
		case op.Data != nil && op.Size == 0 && op.Offset == 0:
			n, err := out.Write(op.Data)
			written += int64(n)
			if err != nil {
				tmp.Close()
				return 0, err
			}
		case op.Data == nil && op.Size > 0 && op.Offset >= 0 && op.Offset+op.Size <= info.Size():
			n, err := io.Copy(out, io.NewSectionReader(base, op.Offset, op.Size))
			written += n
			if err != nil {
				tmp.Close()
				return 0, err
			}
		default:
			tmp.Close()
			return 0, fmt.Errorf("operation %d: a copy within the current file (%d bytes) or data", i, info.Size())
		}
	}
	if !bytesEqual(h.Sum(nil), want) {
		tmp.Close()
		return 0, errDeltaMismatch
	}
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		tmp.Close()
		return 0, err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return 0, err
	}
	if err := tmp.Close(); err != nil {
		return 0, err
	}
	return written, os.Rename(tmp.Name(), target)
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var d byte
	for i := range a {
		d |= a[i] ^ b[i]
	}
	return d == 0
}
