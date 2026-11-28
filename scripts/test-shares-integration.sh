#!/bin/sh
# Runs nos-agent's share apply as root against real Samba, ACLs and Unix
# accounts, in a throwaway Debian container (it creates users and groups).
#
#   scripts/test-shares-integration.sh
set -eu

root="$(cd "$(dirname "$0")/.." && pwd)"
bin="$(mktemp -d)"
trap 'rm -rf "$bin"' EXIT

# The test binary is built on the host; the container only needs Samba.
(cd "$root/agent/nos-agent" && CGO_ENABLED=0 go test -c -o "$bin/server.test" ./internal/server)

docker run --rm -v "$bin:/t:ro" debian:trixie sh -euc '
	apt-get update -qq >/dev/null
	DEBIAN_FRONTEND=noninteractive apt-get install -y -qq samba smbclient acl >/dev/null
	useradd --system nos
	mkdir -p /etc/samba/smb.conf.d
	# What the nos-shares package adds to smb.conf.
	printf "\n[global]\n    include = /etc/samba/smb.conf.d/nos-shares.conf\n" >> /etc/samba/smb.conf
	NOS_SHARES_INTEGRATION=1 /t/server.test -test.run TestSharesIntegration -test.v
'
