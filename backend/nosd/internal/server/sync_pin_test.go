package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
)

func TestWebKeyPin(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()
	h := &SyncHandler{webTLSAddr: srv.Listener.Addr().String(), logger: zerolog.Nop()}
	sum := sha256.Sum256(srv.Certificate().RawSubjectPublicKeyInfo)
	if got := h.webKeyPin(context.Background()); got != hex.EncodeToString(sum[:]) {
		t.Fatalf("pin %q", got)
	}
	srv.Close()
	if got := h.webKeyPin(context.Background()); got != "" {
		t.Fatalf("closed server: %q", got)
	}
}
