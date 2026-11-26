package net

import (
	"testing"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Regression (S10): generated WireGuard keys were placeholders or empty.
func TestGenerateWireGuardKeys(t *testing.T) {
	priv, pub, err := GenerateWireGuardKeys()
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
	if priv2, _, _ := GenerateWireGuardKeys(); priv2 == priv {
		t.Fatal("keys must differ between calls")
	}
}
