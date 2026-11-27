package server

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"time"
)

// webKeyPin is the SHA-256 of the public key the web interface presents
// on this machine (Caddy, the default :443 site with nos-certgen's
// certificate), which a sync client pins. nosd can't read the certificate
// file (root and caddy only) but can ask Caddy for it like any client.
// "" if Caddy isn't answering.
func (h *SyncHandler) webKeyPin(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	d := tls.Dialer{Config: &tls.Config{
		// Only reading the certificate the machine presents; nothing is
		// sent over this connection.
		InsecureSkipVerify: true, //nolint:gosec // reads our own certificate
		MinVersion:         tls.VersionTLS12,
	}}
	conn, err := d.DialContext(ctx, "tcp", h.webTLSAddr)
	if err != nil {
		h.logger.Warn().Err(err).Msg("sync pairing: can't read the web interface's certificate; the client must be given its key by hand")
		return ""
	}
	defer conn.Close()
	certs := conn.(*tls.Conn).ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return ""
	}
	sum := sha256.Sum256(certs[0].RawSubjectPublicKeyInfo)
	return hex.EncodeToString(sum[:])
}
