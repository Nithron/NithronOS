# nOS Documentation

nOS (NithronOS) is a self-hosted storage and app platform for a home or
small-office server, built on Debian: Btrfs storage pools, SMB and NFS
shares, Docker apps, backups and NithronSync, managed from a web interface.

This site is new; until its guides are written, the documentation lives in
the repository and the links below go there.

## Getting started

- [Installing from the ISO](https://github.com/Nithron/nOS/blob/main/docs/installer.md)
- [First boot and the setup code](https://github.com/Nithron/nOS/blob/main/docs/first-boot.md)
- [Signing in and sessions](https://github.com/Nithron/nOS/blob/main/docs/admin/login-and-sessions.md)

## Using nOS

| Area | Guide |
|------|-------|
| Storage pools | [Pools](https://github.com/Nithron/nOS/blob/main/docs/storage/pools.md), [disk health](https://github.com/Nithron/nOS/blob/main/docs/storage/health.md) |
| Shares | [Shares](https://github.com/Nithron/nOS/blob/main/docs/admin/shares.md), [permissions](https://github.com/Nithron/nOS/blob/main/docs/user/shares-permissions.md) |
| Apps | [App catalog](https://github.com/Nithron/nOS/blob/main/docs/apps/catalog.md) |
| Backups | [Backups](https://github.com/Nithron/nOS/blob/main/docs/backup.md) |
| NithronSync | [Setting up clients](https://github.com/Nithron/nOS/blob/main/docs/sync/client-setup.md) |
| Network | [Networking and remote access](https://github.com/Nithron/nOS/blob/main/docs/networking.md), [HTTPS](https://github.com/Nithron/nOS/blob/main/docs/admin/https.md) |
| Updates | [Updates and rollback](https://github.com/Nithron/nOS/blob/main/docs/updates.md) |
| Monitoring | [Monitoring and alerts](https://github.com/Nithron/nOS/blob/main/docs/monitoring.md) |
| Recovery | [Recovery](https://github.com/Nithron/nOS/blob/main/docs/admin/recovery.md) |

## What it has

- **Storage:** Btrfs pools (single, RAID0, RAID1, RAID10), snapshots,
  scrubs and SMART monitoring.
- **Shares:** SMB and NFS, with owners and readers from users and groups.
- **Apps:** Docker apps from a catalog, installed from the web interface.
- **Security:** sign-in with optional two-factor authentication (TOTP), a
  firewall that allows the local network only by default, WireGuard and
  HTTPS for remote access.
- **Backups:** scheduled snapshots and copies to other destinations.
- **NithronSync:** a sync daemon and desktop app (Linux, Windows, macOS),
  an Android app, and WebDAV for file managers.

## Requirements

- A 64-bit x86 computer with 2 GB of memory (4 GB or more recommended)
- A disk of 20 GB or more for the system, and data disks for pools
- A wired network connection

## Developers

- [The REST API](https://github.com/Nithron/nOS/blob/main/docs/dev/api.md)
  (the full route list is in `docs/api/openapi.json`)
- [Building and packaging](https://github.com/Nithron/nOS/blob/main/docs/dev/build-and-packaging.md)
- [Continuous integration](https://github.com/Nithron/nOS/blob/main/docs/dev/ci.md)

## Community

- [GitHub](https://github.com/Nithron/nOS)
- [Discord](https://discord.gg/qzB37WS5AT)
