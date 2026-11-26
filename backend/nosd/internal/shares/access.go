package shares

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Principals in Owners and Readers.
const (
	UserPrefix  = "user:"
	GroupPrefix = "group:"
)

// reName is a user or group name: what useradd and groupadd accept without
// tricks, and what Samba and setfacl take as-is.
var reName = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)

// ValidName reports whether s can name a user or a group.
func ValidName(s string) bool { return reName.MatchString(s) }

// ParsePrincipal splits "user:alice" or "group:media".
func ParsePrincipal(p string) (kind, name string, err error) {
	switch {
	case strings.HasPrefix(p, UserPrefix):
		kind, name = "user", strings.TrimPrefix(p, UserPrefix)
	case strings.HasPrefix(p, GroupPrefix):
		kind, name = "group", strings.TrimPrefix(p, GroupPrefix)
	default:
		return "", "", fmt.Errorf("principal %q: use user:<name> or group:<name>", p)
	}
	if !ValidName(name) {
		return "", "", fmt.Errorf("principal %q: invalid name", p)
	}
	return kind, name, nil
}

// Everyone is the principal for every nOS user ("group:users"). No real
// group can have that name.
const Everyone = GroupPrefix + AllUsersGroup

// AllUsersGroup is the built-in group every nOS user is in.
const AllUsersGroup = "users"

// Access reports what the user (a member of groups) may do in the share.
// This is the rule Samba, the file ACLs, NithronSync and WebDAV all follow. A
// share without owners and readers is closed to everyone.
func (s Share) Access(username string, groups []string) (read, write bool) {
	groups = append(slices.Clone(groups), AllUsersGroup)
	matches := func(list []string) bool {
		for _, p := range list {
			kind, name, err := ParsePrincipal(p)
			if err != nil {
				continue
			}
			if kind == "user" && name == username {
				return true
			}
			if kind == "group" && slices.Contains(groups, name) {
				return true
			}
		}
		return false
	}
	if matches(s.Owners) {
		return true, true
	}
	return matches(s.Readers), false
}
