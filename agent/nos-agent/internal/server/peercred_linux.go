//go:build linux

package server

import (
	"net"
	"syscall"
)

func connPeerCred(c net.Conn) peerCred {
	uc, ok := c.(*net.UnixConn)
	if !ok {
		return peerCred{}
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return peerCred{}
	}
	var cred *syscall.Ucred
	var credErr error
	if err := raw.Control(func(fd uintptr) {
		cred, credErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil || credErr != nil || cred == nil {
		return peerCred{}
	}
	return peerCred{uid: cred.Uid, gid: cred.Gid, known: true}
}
