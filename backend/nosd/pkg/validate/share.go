package validate

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
)

// ShareRoots are the directories a share's path must be inside: /srv (where
// /srv/shares lives) and /mnt (where storage pools are mounted).
var ShareRoots = []string{"/srv", "/mnt"}

// MaxDescription is the longest share description accepted.
const MaxDescription = 256

var (
	ErrBadDescription = errors.New("description must be at most 256 characters, without control characters")
	ErrBadPrincipal   = errors.New("invalid user or group name")
	ErrBadNFSClient   = errors.New("invalid NFS client (use a host name, IP address, network or wildcard)")

	errExpected = errors.New("expected an error")

	// A user name, or a group as @name, as Samba's "valid users" takes them.
	rePrincipal = regexp.MustCompile(`^@?[A-Za-z_][A-Za-z0-9_.-]{0,31}\$?$`)
	// An exports(5) client: host name, wildcard, IPv4/IPv6 address or network.
	// No spaces or parentheses, so it can't carry its own export options.
	reNFSClient = regexp.MustCompile(`^[A-Za-z0-9.*?:_/\[\]-]{1,255}$`)
)

// Description checks a share description (it is written into smb.conf).
func Description(s string) error {
	if len(s) > MaxDescription || strings.IndexFunc(s, unicode.IsControl) >= 0 {
		return ErrBadDescription
	}
	return nil
}

// Principal checks a user or @group name for a share's access list.
func Principal(s string) error {
	if !rePrincipal.MatchString(s) {
		return ErrBadPrincipal
	}
	return nil
}

// NFSClient checks one client of an NFS export.
func NFSClient(s string) error {
	if !reNFSClient.MatchString(s) {
		return ErrBadNFSClient
	}
	return nil
}
