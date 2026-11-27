package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"strings"
)

// ErrServerKeyMismatch: the server's key isn't the pinned one (another
// machine, or someone in between).
var ErrServerKeyMismatch = errors.New("the server's TLS key doesn't match the one this device was paired with")

// NormalizeKeyPin accepts a pin as hex, with or without colons, any case.
func NormalizeKeyPin(pin string) (string, error) {
	p := strings.ToLower(strings.NewReplacer(":", "", " ", "").Replace(strings.TrimSpace(pin)))
	if b, err := hex.DecodeString(p); err != nil || len(b) != sha256.Size {
		return "", errors.New("the server key must be a SHA-256 in hex (64 characters)")
	}
	return p, nil
}

// KeySHA256 is the pin of a certificate: SHA-256 of its public key.
func KeySHA256(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return hex.EncodeToString(sum[:])
}

// tlsConfig checks the server against pin when one is set, instead of
// against the system's CAs. (The client used only the system's CAs, so it
// couldn't connect to a nOS machine's self-signed certificate at all.)
func tlsConfig(pin string) *tls.Config {
	if pin == "" {
		return &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		// The chain isn't checked against CAs; VerifyConnection checks the
		// key instead, which is stricter for a single known machine.
		InsecureSkipVerify: true, //nolint:gosec // replaced by the key pin below
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return ErrServerKeyMismatch
			}
			got := KeySHA256(cs.PeerCertificates[0])
			if subtle.ConstantTimeCompare([]byte(got), []byte(pin)) != 1 {
				return ErrServerKeyMismatch
			}
			return nil
		},
	}
}
