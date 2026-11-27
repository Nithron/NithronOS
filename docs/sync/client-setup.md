# NithronSync Client Setup

How to connect a computer to nOS with NithronSync, or reach a share with a
file manager over WebDAV.

What exists today:

| Client | State |
|--------|-------|
| Sync daemon (`nithron-sync-daemon`, Linux, Windows, macOS) | Works; tested end to end in CI against a real nOS |
| Desktop app (Wails), Linux and Windows | Builds in CI (download the binary from a CI run's artifacts); no installer yet |
| Desktop app, macOS | Not built yet |
| Mobile apps (iOS, Android) | Not available yet |
| File managers over WebDAV | Work with a device token as the password |

## Before you start

1. The share must allow sync: it does unless **Sync** is turned off in the
   share's settings (**Shares → edit → Sync**). You can sync the shares you
   may read; you can upload to the shares you may change.
2. Pair the device: in the web interface open **Settings → Sync → Devices →
   Add Device**, give it a name, and copy the **pairing link**. It looks like
   `nos://sync?server=…&token=…&refresh=…&key=…` and is shown once.

The pairing link holds the device's tokens (keep it private) and, unless your
nOS uses a public domain with a Let's Encrypt certificate, the server's key:
nOS's own certificate is self-signed, so the client trusts exactly that key
instead of a certificate authority. The device token lasts 90 days and the
client renews it with the refresh token in the link. A device can be revoked
on the same page.

## Sync daemon

The daemon syncs every share you may read into `<folder>/<share name>`.

```bash
# Pair once (saves the configuration), then keep running
nithron-sync-daemon --pair 'nos://sync?server=…' --folder ~/NithronSync
```

Later starts need no arguments. Options: `--config <file>` (another
configuration file), `--debug`. The poll interval (`poll_interval_secs`,
default 30, minimum 5) is in the configuration file.

To run it at login on Linux, create `~/.config/systemd/user/nithron-sync.service`:

```ini
[Unit]
Description=NithronSync

[Service]
ExecStart=%h/.local/bin/nithron-sync-daemon
Restart=on-failure

[Install]
WantedBy=default.target
```

and enable it with `systemctl --user enable --now nithron-sync`
(`journalctl --user -u nithron-sync -f` for its log).

Build it from source: `cd clients/sync-core && go build -o nithron-sync-daemon ./cmd/daemon`
(needs a C compiler, for SQLite).

## Desktop app

Start NithronSync, paste the pairing link, and choose a folder (default
`~/NithronSync`). On Windows it keeps running in the notification area when
the window is closed; on Linux closing the window quits it (there is no tray
there yet; use the daemon for background sync).

## How syncing behaves

- Changes are picked up from the folder at once and from the server on every
  poll.
- Large files go up as deltas: only the blocks (4 MiB) that changed are sent.
- If a file changed on both sides since the last sync, the server's version
  is downloaded and yours is kept next to it as a conflict copy (the default
  `conflict_policy`, `keep_both`).
- Temporary and system files (`*.tmp`, `~$*`, `.DS_Store`, `Thumbs.db`,
  `desktop.ini`, `.git`, `node_modules`, …) are not synced.

## WebDAV (file managers)

```
https://<your nOS>/dav/<share id>/
```

The web interface doesn't show share ids yet; list them with the device
token: `curl -H 'Authorization: Bearer nos_dt_…' https://<your nOS>/api/v1/sync/shares`
(`share_id`; add `-k` or `--cacert` for a self-signed certificate). Sign in
with any user name and the **device token** (the `token=` part of the
pairing link, URL-decoded, `nos_dt_…`) as the password.

- **Windows Explorer:** This PC → Map network drive → the URL → Connect using
  different credentials. Windows refuses self-signed certificates for WebDAV:
  import nOS's certificate into the trusted root store first, or use a public
  domain.
- **macOS Finder:** Go → Connect to Server (⌘K) → the URL.
- **Linux (Files, Dolphin):** `davs://<your nOS>/dav/<share id>/`.

## Troubleshooting

**The client can't connect**
- Open the server address in a browser from the same machine.
- "the server's TLS key doesn't match": the server's key changed (a new
  installation) or you are connecting to another machine. Pair the device
  again.
- Without a key in the pairing link, the server needs a certificate from a
  public authority (a public domain).

**Nothing syncs**
- Is the share's sync turned on, and may you read it?
- Was the device revoked? Pair it again.
- Look at the log (below) for "Local change not synced" or "Failed to fetch
  changes".

**Logs and configuration**

| | Configuration | Logs |
|---|---|---|
| Linux | `~/.config/NithronSync/config.json` | `~/.local/share/nithron-sync/logs/` |
| Windows | `%APPDATA%\NithronSync\config.json` | `%LOCALAPPDATA%\NithronSync\logs\` |
| macOS | `~/Library/Application Support/NithronSync/config.json` | `~/Library/Logs/NithronSync/` |
