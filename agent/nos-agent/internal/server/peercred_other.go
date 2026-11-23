//go:build !linux

package server

import "net"

// Peer credentials are only checked on Linux (the platform the agent ships
// on); elsewhere the socket permissions are the only guard.
func connPeerCred(net.Conn) peerCred { return peerCred{uid: 0, known: true} }
