package server

import (
	"testing"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Regression (S10): every generated WireGuard key pair was the literal
// "private-key-placeholder"/"public-key-placeholder".
func TestGenerateWGKeys(t *testing.T) {
	h := &NetworkConfigHandler{}
	priv, pub, err := h.generateWGKeys()
	if err != nil {
		t.Fatal(err)
	}
	k, err := wgtypes.ParseKey(priv)
	if err != nil {
		t.Fatalf("private key: %v", err)
	}
	if k.PublicKey().String() != pub {
		t.Fatal("public key does not belong to the private key")
	}
	priv2, _, _ := h.generateWGKeys()
	if priv2 == priv {
		t.Fatal("keys must differ between calls")
	}
}
