# nOS updates and rollback

An update upgrades the system's packages with APT. Before it does, nOS saves
its own configuration and keeps the installed version of every package it is
about to upgrade. If the upgrade fails, or nOS does not work afterwards, the
update is rolled back automatically: the configuration is restored and the
previous package versions are reinstalled. Any update can also be rolled back
by hand later, from the snapshot it left.

## What an update does

The Updates page (**Settings → Updates & releases**) and `POST /api/v1/updates/apply`
start the same sequence. nosd runs it and asks nos-agent for everything that
needs root.

1. **Checks.** nos-agent must answer and no other update may be running. At
   least 2 GB must be free on `/` and on `/var/cache/apt`. The package lists
   are refreshed (`apt-get update`) and the packages to upgrade are listed with
   their installed and new versions.
2. **Snapshot.** nOS's configuration, `/etc/nos` (users, groups, shares,
   settings and secrets), is saved as an archive under
   `/var/lib/nos/snapshots/`. Then nos-agent keeps a `.deb` of the installed
   version of each package being upgraded, in `/var/lib/nos/update-debs/<id>/`.
   It takes the file from APT's cache when it is there, downloads it from the
   package repository when it is still offered, and otherwise rebuilds it from
   the installed files with `dpkg-repack`.
   If a version cannot be kept, the update stops before anything is installed,
   because it could not be undone.
3. **Install.** `apt-get upgrade` (or `apt-get install --only-upgrade` for the
   packages chosen) runs as its own systemd unit, `nos-update`, so it is not
   interrupted when the upgrade restarts nosd or nos-agent. New dependencies
   are installed; nothing is removed. Changed configuration files keep the
   local version.
4. **Verify.** After the upgrade, nosd, nos-agent and Caddy must be running,
   the web interface's files must be in place, and the API must answer through
   Caddy. nOS gives them up to three minutes to come back.
5. **Done.** The update is marked successful and old snapshots are pruned (see
   [Snapshot retention](#snapshot-retention)). If a package asks for a restart
   (a new kernel, for example), the page says so; nOS does not restart the
   system by itself.

Data pools are never touched by an update.

## Automatic rollback

The update is rolled back without asking when:

- APT fails;
- the checks after the upgrade still fail when the three minutes are over;
- the upgrade did not run to the end, for example because the system lost
  power or was restarted during it. nosd notices when it starts again.

A rollback restores `/etc/nos` from the snapshot and then runs the systemd unit
`nos-update-revert`. That unit finishes any interrupted `dpkg` run
(`dpkg --configure -a`), reinstalls the kept `.deb`s with downgrades allowed,
and restarts nosd. The restarted nosd reads the restored configuration and
marks the rollback done. The update is recorded as failed, with the reason and
"rolled back automatically".

If the rollback itself fails, the page says so and shows the log. The system
may then be between versions; the kept `.deb`s are still in
`/var/lib/nos/update-debs/<id>/` and can be installed by hand with
`apt-get install --allow-downgrades /var/lib/nos/update-debs/<id>/*.deb`.

## Rolling back by hand

Every snapshot listed under **System snapshots** can be rolled back. This puts
back the configuration from before that update and reinstalls the package
versions that update replaced. nosd restarts at the end, so the page reconnects.

```bash
curl -X POST https://<nos>/api/v1/updates/rollback \
  -H "Content-Type: application/json" \
  -d '{"snapshot_id": "<id from /api/v1/updates/snapshots>"}'
```

A rollback answers `202 Accepted` once it has started. Follow
`/api/v1/updates/progress` until the state is `rolled_back` or `failed`.

Packages that an update newly installed (new dependencies) stay installed after
a rollback; they are not removed.

Rolling back an older update only reinstalls the packages that update changed;
packages upgraded by later updates stay at their newer versions. The
configuration goes back to how it was before that update.

## Snapshot retention

After each successful update, nOS keeps the newest **3** update snapshots and
deletes the older ones together with their kept `.deb`s. To change the number
(1 to 20), use the setting under **System snapshots**, or the API:

```bash
curl -X PUT https://<nos>/api/v1/updates/config \
  -H "Content-Type: application/json" \
  -d '{"snapshot_retention": 5}'
```

The setting is stored in `/etc/nos/update/config.json` and is read when it is
needed, so no restart is necessary after changing the file. A snapshot can also
be deleted by hand from the list.

## Following an update

```bash
# The state, the phase and the log
curl https://<nos>/api/v1/updates/progress

# The same as Server-Sent Events
curl -N https://<nos>/api/v1/updates/progress/stream
```

The states are `idle`, `applying`, `verifying`, `success`, `failed`,
`rolling_back` and `rolled_back`. The phases are `preflight`, `snapshot`,
`install`, `postflight` and `rollback`.

The progress is kept in `/var/lib/nos/updates/state.json` and the list of
updates and rollbacks in `/var/lib/nos/updates/records.json`. Both survive a
restart of nosd. Each run of APT is also written to
`/var/log/nos-updates.log`, and the systemd journal has the full output:

```bash
journalctl -u nos-update
journalctl -u nos-update-revert
```

Only one update or rollback runs at a time; starting a second one answers
`409 Conflict`.

## Channels and the package repository

nOS records a release channel, **stable** (the default) or **beta**, in
`/etc/nos/update/channel`. nOS does not have its own package repository yet,
so the channel does not change where packages come from: updates come from the
APT sources the system has. When the signed nOS repository at
`https://nos.nithron.com/apt` is published, the channel will choose between
its stable and beta suites.

## Troubleshooting

**The update does not start.** The message says why: nos-agent not answering,
another update running, or too little free space (`df -h / /var/cache/apt`).

**The update stopped at "Keeping the installed versions".** A package's
installed version could not be kept. Check that `dpkg-repack` is installed
(`apt-get install dpkg-repack`). An update can still be forced through the API
with `{"force": true}`, but then it cannot be rolled back completely.

**The update was rolled back.** The reason is shown on the Updates page and in
the record (`GET /api/v1/snapshots/<id>`). For the checks after an upgrade:

```bash
systemctl status nosd nos-agent caddy
curl -s http://127.0.0.1/api/v1/health
```

## Telemetry

nOS sends nothing about updates anywhere.
