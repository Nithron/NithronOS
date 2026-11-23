package server

import (
	"context"
	"net"
	"net/http"
	"os/user"
	"strconv"
)

// The agent runs as root and its API can wipe disks, write files and run
// commands, so only root and the nos service account (nosd) may use it. The
// socket is root:nos 0660, and every connection's peer credentials are checked
// as well (Linux SO_PEERCRED), so a wrong socket mode can't open it up again.

type peerCredKey struct{}

// peerCred is the uid/gid of the process on the other end of the socket.
type peerCred struct {
	uid, gid uint32
	known    bool
}

// serviceAccount is the account allowed besides root.
type serviceAccount struct {
	uid, gid int // -1 when the account doesn't exist
}

func lookupServiceAccount(name string) serviceAccount {
	acc := serviceAccount{uid: -1, gid: -1}
	if u, err := user.Lookup(name); err == nil {
		if id, err := strconv.Atoi(u.Uid); err == nil {
			acc.uid = id
		}
	}
	if g, err := user.LookupGroup(name); err == nil {
		if id, err := strconv.Atoi(g.Gid); err == nil {
			acc.gid = id
		}
	}
	return acc
}

// peerAllowed reports whether a peer may use the agent.
func peerAllowed(c peerCred, acc serviceAccount) bool {
	if !c.known {
		return false
	}
	if c.uid == 0 {
		return true
	}
	return (acc.uid >= 0 && int(c.uid) == acc.uid) || (acc.gid >= 0 && int(c.gid) == acc.gid)
}

// withPeerCred stores the connection's peer credentials in the request
// context (http.Server.ConnContext).
func withPeerCred(ctx context.Context, c net.Conn) context.Context {
	return context.WithValue(ctx, peerCredKey{}, connPeerCred(c))
}

// requirePeer rejects requests from processes other than root and acc.
func requirePeer(acc serviceAccount, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := r.Context().Value(peerCredKey{}).(peerCred)
		if !peerAllowed(c, acc) {
			writeErr(w, http.StatusForbidden, "forbidden")
			return
		}
		next.ServeHTTP(w, r)
	})
}
