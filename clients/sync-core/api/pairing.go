package api

import (
	"errors"
	"net/url"
	"strings"

	"nos/clients/sync-core/config"
)

// Pairing is what a client needs to sync with a nOS machine; the web
// interface's pairing dialog gives it as one link,
//
//	nos://sync?server=<URL>&token=<device token>&refresh=<refresh token>&key=<server key>&name=<device name>
//
// (also the mobile app's QR code). refresh and key are optional: without a
// refresh token the device must be paired again when its token expires;
// without a key the server needs a CA-signed certificate (public domain).
type Pairing struct {
	ServerURL    string
	DeviceToken  string
	RefreshToken string
	ServerKey    string // normalized (NormalizeKeyPin)
	DeviceName   string
}

// ParsePairingLink reads a nos://sync link.
func ParsePairingLink(link string) (Pairing, error) {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil || u.Scheme != "nos" || u.Host != "sync" {
		return Pairing{}, errors.New("not a pairing link: it starts with nos://sync?")
	}
	q := u.Query()
	p := Pairing{
		ServerURL:    strings.TrimRight(q.Get("server"), "/"),
		DeviceToken:  q.Get("token"),
		RefreshToken: q.Get("refresh"),
		DeviceName:   q.Get("name"),
	}
	su, err := url.Parse(p.ServerURL)
	if err != nil || (su.Scheme != "https" && su.Scheme != "http") || su.Host == "" {
		return Pairing{}, errors.New("the pairing link has no server address")
	}
	if !strings.HasPrefix(p.DeviceToken, "nos_dt_") {
		return Pairing{}, errors.New("the pairing link has no device token")
	}
	if k := q.Get("key"); k != "" {
		if p.ServerKey, err = NormalizeKeyPin(k); err != nil {
			return Pairing{}, err
		}
	}
	return p, nil
}

// Apply saves the pairing in cfg, syncing into folder.
func (p Pairing) Apply(cfg *config.Config, folder string) error {
	return cfg.Update(func(c *config.Config) {
		c.ServerURL = p.ServerURL
		c.DeviceToken = p.DeviceToken
		c.RefreshToken = p.RefreshToken
		c.ServerKeySHA256 = p.ServerKey
		c.SyncFolder = folder
		c.SyncEnabled = true
	})
}
