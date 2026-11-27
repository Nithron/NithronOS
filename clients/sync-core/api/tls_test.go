package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKeyPin(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()
	pin := KeySHA256(srv.Certificate())
	get := func(p string) error {
		c := &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig(p)}}
		res, err := c.Get(srv.URL)
		if err == nil {
			res.Body.Close()
		}
		return err
	}
	if err := get(pin); err != nil {
		t.Fatalf("pinned key: %v", err)
	}
	other := strings.Repeat("ab", 32)
	if err := get(other); !errors.Is(err, ErrServerKeyMismatch) {
		t.Fatalf("other key: %v, want ErrServerKeyMismatch", err)
	}
	if err := get(""); err == nil {
		t.Fatal("no pin: a self-signed certificate was accepted")
	}
	colons := strings.ToUpper(pin[:2]) + ":" + pin[2:]
	if got, err := NormalizeKeyPin(colons); err != nil || got != pin {
		t.Fatalf("normalize: %q %v", got, err)
	}
	if _, err := NormalizeKeyPin("abc"); err == nil {
		t.Fatal("a short pin was accepted")
	}
}
