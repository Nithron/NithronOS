# nOS Revision Plan

NithronOS was renamed nOS (NithronOS stays the long form) between phases 3 and 4; see "Rename to nOS" below. Older sections keep the names they were written with.

Status: draft, 2026-11-21. Based on a full audit of `main` @ `ce063da`.

This plan covers what is broken, half-built or badly structured, in priority order, and lines the work up against the README roadmap. Each phase should land as its own branch/PR with CI green.

---

## 0. Audit summary

| Area | State |
|---|---|
| Backend (`nosd`, ~53k LOC Go) | Builds, unit tests pass. ~10 dead packages (~5.4k LOC), 3 parallel user stores, 3 session stores, 4 share implementations, mock data in core endpoints, critical auth bugs. |
| Agent (`nos-agent`) | Builds, tests pass. systemd sandbox blocks its own job (deb unit) or exposes it to every local user (ISO unit). |
| Web UI | Unit tests pass, `tsc` fails (3 errors). Calls **~79 API paths that don't exist** in the backend. 8 orphan pages, 4–5 overlapping API client layers, mock data in Apps / Install wizard / Monitoring. |
| First boot / OTP | Broken end to end (see Phase 1). |
| Installer / ISO | Installed systems get **no NithronOS packages** (live-build marks them live-only and the installer removes them); the menus boot a stock Debian installer; live-build's own installer couldn't start. |
| Sync clients | Desktop is a skeleton. Mobile has no native projects (cannot build). QR scanner is a mock. README marks all of this "Complete". |
| CI | Last run on `main` failed in backend lint. Lockfile out of sync (`npm ci` fails). |

---

## Phase 0: Baseline and repo hygiene (small, do first)

Status: done on `revision/phase-0-baseline`. Found along the way and fixed here: a **deadlock** on `pools.json` (pool device ops and pool destroy took the same flock twice, hanging every later pool write); ESLint never ran (v9 with a legacy config); `clients/sync-core` and `cmd/nosctl` didn't compile and weren't in CI.

Goal: a clean, reproducible starting point so every later PR is judged on green CI.

1. **Lockfile**: regenerate `web/package-lock.json` so `npm ci` works (axios, sonner, follow-redirects, form-data drift).
2. **TypeScript**: fix the 3 Recharts `Formatter` type errors in `web/src/pages/MonitoringDashboard.tsx`.
3. **CI lint**: run `golangci-lint`, `staticcheck` and `govulncheck` locally, fix findings, bump vulnerable deps. Make the lint job fail loudly instead of `set +e` aggregation hiding which tool failed.
4. **Committed binaries**: remove `agent/nos-agent/nos-agent` and `packaging/deb/nos-agent/deb-build/nos-agent` from git and add them to `.gitignore`. `.gitattributes` has `* text eol=lf`, so git rewrites these binaries on checkout and they show as modified (and corrupted) on every fresh clone. Add `*.png *.ico *.iso *.deb binary` rules.
5. **Tests that mutate tracked files**: backend tests rewrite `devdata/users.json` and create `devdata/apps.json`. Point them at `t.TempDir()`. Untrack `web/tsconfig.tsbuildinfo`.
6. **Version strings**: one source of truth (`VERSION` file, injected via `-ldflags` and Vite env). Today there are `0.9.5-pre-alpha`, `1.0`, `NithronOS 1.0` (installer/issue) and a hard-coded string in `/api/v1/openapi.json`.
7. **Org links**: README, docs, UI and ISO files still point at the old `Nithronverse` org.

---

## Phase 1: First boot, OTP and installer (CRITICAL; the reported bug)

### Root causes found

1. **Installed systems get no NithronOS packages.** Confirmed with a QEMU install of the ISO built from `main` @ `518bbed`: after installing, `/usr/bin/nosd`, `nos-agent` and the web UI are missing, so nothing generates or shows a setup code. Three layered causes:
   - The NithronOS debs were installed by a chroot hook *after* live-build recorded its package list, so live-build listed them all in `live/filesystem.packages-remove`, and `live-installer` **removes** them from every installed system.
   - Every "Install NithronOS" boot entry (BIOS and UEFI) booted `/install.amd/`, a stock Debian installer initrd downloaded by `scripts/fetch-di.sh`, with `priority=low` and no preseed. live-build's own installer under `/install/` (with the embedded preseed and `live-installer`) was unreachable.
   - That installer couldn't start anyway: `debian-installer-startup.d/S10nithronos-branding` is *sourced* by the startup script and ends in `exit 0`, so the installer never launched. Its preseed then used the reserved username `admin`, an unusable password hash, no swap answer, `partman-auto/disk /dev/sda` (could erase a data disk on a multi-disk NAS) and a `late_command` that failed on its own quoting.

   Also: `auto/config` and `build.sh` both configure live-build, the package list ships `docker.io` with Compose v1 and no ZFS, and `build.sh` disables the Debian **security** mirror at build time (`LB_SECURITY=false`).
2. **`pkgsel/include` in the preseeds is broken**: `linux-headers-$(uname -r)` isn't shell-expanded in preseed, so the install step gets an invalid package name. `docker-ce*`/`containerd.io` only resolve because the live image already contains them. It also pulls `build-essential nodejs npm python3-pip`, which an appliance doesn't need.
3. **The OTP expires and is never regenerated.** nosd creates it only once, at process start (`backend/nosd/main.go`), with a 15-minute TTL. `firstboot.Store.Load()` deletes the file once it's expired. After that, `/setup/state` reports `otpRequired:false`, the UI says "Waiting for OTP generation…" forever, and `/otp/verify` says "Invalid". The only recovery is `systemctl restart nosd`, which a fresh-install user can't know about.
4. **Seven competing OTP mechanisms** with different formats and paths:
   | Mechanism | Format | Writes to |
   |---|---|---|
   | nosd `main.go` | 6 digits | `/var/lib/nos/state/firstboot.json`, `/run/nos/firstboot-otp` |
   | ISO `generate-otp` (`nithronos-otp-gen.service`) | 6 digits | firstboot.json (different schema), `/tmp/nos-otp` **0644**, `/etc/nos/otp` **0644**, `/run/nos/firstboot-otp` |
   | ISO `nithronos-firstboot.sh` | 8 alnum | `/etc/nos/first-run-otp`, `/etc/issue` |
   | `deploy/firstboot/print-url@.service` | 8 alnum | `/etc/nos/first-run` |
   | `deploy/scripts/nos-firstboot-otp.sh` | 8 alnum, calls non-existent `nosd generate-otp` | `/var/lib/nos/firstboot-otp`, says "expires in 30 minutes" |
   | `otp-notify` / `otp-announce.service` | reads firstboot.json | console; **disabled** unless `/etc/nos/enable-terminal-otp` exists |
   | `update-motd.d/50-nithronos-otp` | reads firstboot.json | MOTD (only after login) |

   The 8-character codes can never pass `/otp/verify`, which requires exactly 6 digits.
5. **The display scripts can't parse nosd's file.** `otp-notify` and the MOTD script grep for `"used": false`, but nosd's current schema (`{otp, issued_at, expires_at}`) has no `used` field, so they never print anything.
6. **The console print is wiped.** nosd prints the OTP to `journal+console` early in boot, then `getty@tty1` clears the screen (agetty default) and the code is gone.
7. **The setup UI gives the wrong instructions**: "OTP is valid for 10 minutes. If expired, restart nosd" (`Setup.tsx:336`); the real TTL is 15 minutes.
8. nosd generates and prints an OTP on **every** start, even after setup is complete (`main.go` calls `NewOrReuse` unconditionally).
9. Installed and live systems still claim `NithronOS 1.0` (`/etc/os-release`, `/etc/issue`, `finish-install.d/10nithronos-setup`, the preseeds). Generate these from `VERSION` at ISO build time.

### Fix design

- **Single owner: nosd.** Delete `generate-otp`, `nithronos-firstboot.sh`, `print-url@.service`, `nos-firstboot-otp.sh/.service`, `otp-announce.service`, `otp-notify`, `nithronos-otp-gen.service`, the MOTD script and the `/tmp`, `/etc/nos/otp` copies.
- `internal/setup/firstboot`: one schema `{otp, issued_at, expires_at, attempts}` with a migration path for the older formats. Generate the OTP only while `!HasAdmin()`.
- **Auto-rotate**: a background goroutine in nosd regenerates the OTP when it expires, for as long as no admin exists. `/setup/state` returns `otpRequired:true` plus `expiresAt`, so the UI can show a countdown and "a new code is on the server console".
- **Console display that survives getty**: nosd writes `/run/nos/issue` (OTP, IP URLs, expiry). A tmpfiles symlink exposes it as `/etc/issue.d/50-nithronos.issue`, and nosd runs `agetty --reload` (or `systemctl kill -s HUP getty@*`) after each rotation. agetty also refreshes the IP itself via `\4` escapes. The same text goes to the journal and `/dev/ttyS0` for serial/headless installs. Remove the issue file when setup completes.
- Mark the OTP consumed on a successful first-admin. Lock out after N wrong attempts (on top of the existing per-IP limit).
- **Installer** (done in phase 1): the .debs are staged as live-build local packages (`config/packages.chroot` + a generated package list) and the build fails if any of them lands in `filesystem.packages-remove`; boot menus (BIOS and UEFI) start live-build's installer from `/install/` with `auto=true priority=high`; one embedded preseed that asks only for the console password, the disk and the erase confirmation; `late-command.sh` checks nosd/nos-agent/web UI are on the disk (fails the install otherwise), writes APT sources incl. security and enables the services. Removed `fetch-di.sh`, the broken startup script, three unused preseeds, the placeholder and deb hooks and two dead build scripts. Moved to phase 6: one live-build config, Docker from Docker's repo (Compose v2), ZFS in the image, package-list trim, the GRUB memtest and "boot from disk" entries.
- Fix the Setup UI copy and add an expiry countdown.
- **QEMU acceptance test** (done in phase 1, `tests/smoke/iso-install-test.sh`, BIOS and UEFI in CI): install to a virtual disk, reboot, read the 6-digit code from the serial console, and check that `/api/v1/setup/otp/verify` rejects a wrong code and accepts it.

---

## Phase 2: Security fixes (CRITICAL)

Status: done on `revision/phase-2-security`. All of S1 to S15 are fixed with regression tests (S12 in phase 1; S13 by deleting the unused in-memory TOTP manager). Found along the way and fixed: the settings page's `/auth/2fa/*` ran on that in-memory manager (every request was user `user-123`, any password was accepted, nothing persisted or was enforced at login); 2FA now has one implementation on the user store with pending enrollment. `DELETE /users/{id}` never deleted anyone. The agent's interface name was used in a file path (`../` wrote root-owned files anywhere).

| # | Issue | Where |
|---|---|---|
| S1 | **2FA is never enforced at login.** `/auth/login` ignores `code`/TOTP entirely. | `router.go` `/api/v1/auth/login` |
| S2 | **`/auth/totp/setup` and `/auth/totp/confirm` are unauthenticated** and the password check is commented out. Anyone who knows an email gets a TOTP secret generated. | `router.go:899-942` |
| S3 | Login accepts `plain:` and `dev:` password hashes in production. | `router.go` login |
| S4 | `middleware.RealIP` trusts `X-Forwarded-For` unconditionally, so the "localhost only" checks on `/api/v1/recovery/*` and `/api/v1/setup/recover` (password reset, 2FA disable, **delete users**) can be bypassed remotely, and per-IP rate limits are bypassable. Honour XFF only when `TrustProxy` is set and the peer is the local Caddy; use the real socket peer for the localhost checks. | `router.go:91,311,615` |
| S5 | The client-supplied `X-UID`/`X-SID` headers are trusted when no session cookie decodes; `sync_handler`, `apps_handler` and `notifications_handler` read identity from them. Move identity into `context` only and strip inbound `X-UID`/`X-SID`. | `router.go:268`, handlers |
| S6 | `/api/v1/auth/refresh` is registered twice; the second (no rotation, no reuse detection) replaces the first. | `router.go:740,888` |
| S7 | Agent socket: the ISO unit does `chmod 666` (any local user gets root actions such as `wipefs -a`). The deb unit uses `chgrp nos` + 0660. There is no caller authentication; add `SO_PEERCRED` uid/gid checks. | `nos-agent.service` x2, `agent/.../server.go` |
| S8 | Agent `system.hostname.set` writes an unvalidated hostname into `/etc/hostname` and `/etc/hosts` (newline injection). Validate RFC 1123 in both nosd and the agent. | `agent/.../system.go` |
| S9 | The agent bootstrap token is raw binary written by nosd and compared as a trimmed string. Use hex. The `agents/register` endpoint is enabled by default (`AllowAgentRegistration: true`). | `main.go`, `config.go` |
| S10 | WireGuard returns literal `"private-key-placeholder"`/`"public-key-placeholder"` keys. | `network_config_handler.go:693` |
| S11 | Users: "prevent deleting any admin" instead of a proper last-admin check. | `users_handler.go:294` |
| S12 | Default installer password `nithronos` for the `admin` Unix user with sudo and docker. Prompt for it or force a change on first login. | preseeds |
| S13 | TOTP secret "encryption" in `pkg/net/totp.go` is a TODO. Reuse the XChaCha20 helper used by the user store. | `pkg/net/totp.go:320` |
| S14 | nosd `signal.Notify(ch)` without a signal list catches **every** signal, including the runtime's SIGURG, so config hot-reload fires constantly. Use `syscall.SIGHUP`. | `main.go` |

| S15 | `generateUUID()` is not random: it is built from `time.Now().UnixNano()` with the last 8 bytes always zero. It mints **per-agent auth tokens** (`/agents/register`), user IDs, session record IDs and transaction IDs, so tokens are guessable from the registration time. Use `crypto/rand` (`github.com/google/uuid` is already a dependency). | `router.go:1887` |

Add regression tests for each (auth flow tests exist in `router_flow_test.go` to extend).

---

## Phase 3: Backend restructuring

The backend grew by adding parallel implementations instead of finishing one. Consolidate before adding features, so features land once.

1. **Dead packages**: the duplicates `internal/{backup,firewall,https,wireguard,observability}` were deleted in phase 0. Phase 3: `pkg/appsdk` (an unused manifest type for README A9) deleted; `pkg/validate` now validates share fields. Left for later phases on purpose: `pkg/webhooks` (notification delivery, Phase 4) and `pkg/updates` + `pkg/snapcfg` (update flow with pre-update snapshot and rollback, Phase 4); delete whatever Phase 4 doesn't use.
2. **One user store**: today `auth.NewStore` (`pkg/auth`), `userstore` (`internal/auth/store`) and `pkg/auth.UserManager` all read and write `/etc/nos/users.json`. Keep `internal/auth/store` and port the TOTP and recovery logic into it.
3. **One session system**: `internal/sessions`, `internal/auth/session` and `pkg/auth/session_manager.go`, plus a legacy `SessionCodec` cookie. Pick opaque server-side sessions, and make `requireAuth` verify them (today the binding middleware explicitly "does not enforce").
4. **One shares implementation**: `internal/shares`, `pkg/shares`, `SharesHandlerV1` (mock fallback) and `SharesHandlerV2`.
5. **One network stack**: `pkg/net` and `pkg/firewall` vs the dead `internal/*`; merge `net_handler.go` (stub TODOs) and `network_config_handler.go` into one API.
6. **One apps layer**: `internal/apps` wraps `pkg/apps`, and there is a fallback `apps.Catalog`. Fail fast instead of silently registering stub routes.
7. **Split `router.go` (2.5k lines)** into `routes_auth.go`, `routes_setup.go`, `routes_storage.go`, and so on, each with a `Register(r chi.Router, deps)` function. A dependency container is built once in `main.go`. Remove duplicate registrations (`/storage/devices` twice, `/health/*` both public at the root and mounted under auth, the `/system/*/…` wildcard duplicates in the route dump).
8. **No silent mock fallbacks**: return `503 agent.unavailable` instead of fake disks (`router.go:1093`), fake SMART (`smart_handler.go:110`), fake jobs (`jobs_handler.go:213`) or sample health (`health_handler.go:75,99`).
9. Remove dead `ensureFirstBootOTP` in `main.go`; propagate `context` instead of `context.TODO()` (~25 sites).
10. **API contract as the source of truth**: done in phase 3 for paths. `docs/api/openapi.json` is generated from the router (`cmd/route-dump -format openapi`) and served at `/api/v1/openapi.json`; CI fails when it is stale. `scripts/check-fe-api-paths.mjs` checks every API path the UI calls; the 72 paths the UI calls today that nosd doesn't serve (backup, monitoring, firewall, groups/policy, remote, sync settings, update progress and others) are listed in `docs/api/fe-known-missing.json`. Phases 4 and 5 shrink that list to zero (implement the endpoint or fix the UI call). Still to do: request/response schemas in the generated document, then FE types generated from it (replacing `web/src/api/schema.ts`), and per-method checks.

---

## Rename to nOS (between phases 3 and 4)

Done on `revision/rename-nos`. Display name nOS; identifiers `nithronos` became `nos`:

- Go modules `nos/backend/nosd`, `nos/agent/nos-agent`, `nos/clients/...`, `nos/cmd/nosctl`, `nos/installer`.
- Packages: meta package `nos` (with a transitional `nithronos` package for upgrades, left out of the ISO), `nos-caddy`.
- Paths: `/usr/share/nos`, `/usr/lib/nos`, `/var/lib/nos`; `/etc/nithronos` merged into `/etc/nos`, which also fixes nosd (user `nos`) not being able to write `/etc/nithronos/firewall.nft` and `update/channel`. TLS moved to `/etc/ssl/nos` (Caddy reads it; `/etc/nos` is private to `nos`).
- Units `nos-*`, default hostname `nos` (`nos.local`), os-release `ID=nos`, metrics `nos_*`, the sync clients' URL scheme `nos://`, ISO `nOS - <arch> - <tag>.iso`.
- Repository: github.com/Nithron/nOS (GitHub redirects the old URLs).

Existing installs: nosd ships `/usr/lib/nos/migrate-from-nithronos.sh` and its postinst runs it on every configure (it does nothing once nothing old is left). It moves config, state and TLS to the new paths, renames files named after NithronOS (nftables, timesyncd, avahi, apt source and keyring, presets), renames the units the ISO installed and keeps their enablement, rewrites old paths in the Caddyfile, unit files and presets, and rebrands os-release, issue and motd. It never overwrites: if the old and the new file both exist, the new one stays. `deploy/scripts/test-migrate-from-nithronos.sh` (in CI) runs it on a fake NithronOS root. `nos-web` now depends on `nosd`, so the migration runs before nos-web restarts Caddy. The hostname of an installed system is left alone.

Left unchanged on purpose: the license texts (`LICENSE` stays byte-identical with nAI's; it and `TRADEMARK_POLICY.md` already name both), the nAI template (`NAI_NAS_PLATFORM=nithronos` is nAI's identifier), CHANGELOG history, and the placeholder `nithronos.com`/`.org` domains (the apt repository URL in `docs/updates.md`, `site_url` in `docs-site/mkdocs.yml`): which domain nOS uses is the maintainer's call.

---

## Phase 4: Implement missing backend functionality

Grouped by feature. Each item lists the evidence.

**Backup & replication**: done in phase 4. `/api/v1/backup/*` is mounted (admin only; the handler used to be nil). Everything privileged goes through nos-agent: scheduled and manual snapshots are read-only Btrfs snapshots in the subvolume's `.snapshots` folder (`/v1/snapshot/*`; the scheduler used relative placeholder paths and an agent interface nothing implemented), and replication runs `btrfs send | ssh ... btrfs receive` (or to a local folder) as the transient unit `nos-backup-<job>` (`/v1/backup/send`, `/status`), incrementally from a parent snapshot; the replicator used a hard-coded test path. Restore rolls a subvolume back to a local snapshot (the agent keeps the current state in `.rollback`). Fixed on the way: every destination change and schedule change deadlocked (saveState took the lock its callers held), jobs were one shared pointer written by runners while the API read it, and the SSH destination check ran the unquoted path in the remote shell. Not supported, with clear errors: rclone destinations, restoring from a remote destination, single-file restore (snapshots are visible in the share's `.snapshots` folder), and pre/post hooks (they would run any command as root).

**Monitoring & alerts**: done in phase 4. `pkg/monitor` uses the pure-Go `modernc.org/sqlite`, and its schema works (it declared MySQL-style `INDEX` clauses inside `CREATE TABLE`, which SQLite rejects, so the storage never opened with either driver). `/api/v1/monitor/*` is mounted (alert rules, channels and events for admins: channels hold SMTP passwords); the collector and alert engine run in the daemon only. Alert message templates use `text/template` (`.Name .Metric .Value .Threshold .State .Host`). `internal/notifications` delivers webhooks (JSON POST, 10 s timeout) and syslog. Still two notification systems (`/notifications` channels and `/monitor/alerts/channels`); merging them is Phase 5 together with the UI pages.

**Storage**: done in phase 4. `pools.json` has one format (`{records: [...]}`); the bare list import/create wrote is still read, and a file that can't be parsed is never overwritten. Scrub and balance have one implementation (`/api/v1/{scrub,balance}/{status,start,cancel}` for every pool or the one named, `/api/v1/pools/scrub/*` for one mount) backed by new agent endpoints (scrub without `-B`, filtered background balance, cancel, Btrfs mount check); the fake `/api/v1/btrfs/*` handlers and the job-only balance handler are gone. SMART: real self-tests (`POST /smart/test/{device}`, `/smart/scan` on every disk, `GET /smart/test/{device}` for progress); disks come from lsblk (NVMe and virtio disks were never listed). Subvolumes come from `btrfs subvolume list` (`/pools/{id}/subvolumes`); the fake `/storage/pools/{uuid}/options` handlers are gone (the real ones are `/pools/{id}/options` and `/mount-options`).

**Shares (found in phase 3)**: done in phase 4. The API now uses the UI's model: a share is `/srv/shares/<name>`, exported over SMB and/or NFS (`smb`, `nfs`), with `owners` (read/write) and `readers` as `user:<name>`/`group:<name>` principals (`group:users` is everyone; no owners and readers means closed). Old one-protocol records convert on load. After each change nosd sends the whole state to nos-agent's new `/v1/shares/apply`, which validates it again, mirrors nOS groups as Unix groups `nos-<name>`, creates the folder (a Btrfs subvolume when it can) and sets its ACL, writes one `smb.conf.d/nos-shares.conf` and one `exports.d/nos.exports` (testparm first), and reloads smbd and the exports; nosd also applies once at startup. nos-shares' postinst now includes that file (the old glob include was never read). `PATCH`/`PUT` change only the fields sent. NithronSync and WebDAV use the same access rule (WebDAV writes need owner access; sync endpoints checked no access at all). Found and fixed: the agent put SMB passwords into an `sh -c` command line (a password `$(...)` ran as root), and the WebDAV handler map had no lock. `scripts/test-shares-integration.sh` (in CI) runs the apply against real Samba in a container and checks owner/reader/guest access with smbclient. Still to do: the share end to end over SMB and NFS in the QEMU install test.

**Dashboard**: done in phase 4. The shares and apps widgets come from the shares store and the app manager (they showed made-up Documents/Media shares and Plex/Nextcloud apps); maintenance no longer shows a made-up next run. `/api/v1/metrics/stream` sends live vitals (SSE).

**Network / remote access**: partly done in phase 4. The firewall manager reads and loads rulesets through nos-agent (new `/v1/firewall/ruleset`; `/v1/firewall/apply` checks with `nft -c` and keeps a backup) instead of running `nft` as the nos user. The remote-access wizard is per signed-in user (every user shared the session "session-123"), stores its WireGuard and HTTPS steps and applies them (WireGuard, HTTPS, then the firewall plan with the 120 s confirm window) instead of reporting success without doing anything; `/net/status` shows the real interface addresses and listening ports instead of made-up ones, and no external IP (nOS doesn't ask an outside service). Moved to Phase 6: the WireGuard and HTTPS managers still run `wg`/`wg-quick` and write config themselves, so as the nos user they fail; they need agent operations like the firewall's. The HTTPS manager also regenerates the whole `/etc/caddy/Caddyfile` (it would drop the TLS paths, the API proxy and the app imports), its DNS-challenge modes need Caddy plugins Debian's caddy doesn't have, and the WireGuard manager asks api.ipify.org for the public address with curl. Both belong with the Caddy work in Phase 6. The `/network/*` paths the UI gap list shows come from `SettingsNetwork.tsx`, which isn't routed (the routed pages use `api/net.ts` → `/net/*`); Phase 5 removes it with the other duplicate pages, together with `SettingsConnected.tsx` (`/system/config`, `/settings/advanced`).

**Updates**: done in phase 4. One implementation, `internal/updates.Service`, behind `/api/v1/updates/{version,channel,check,apply,progress,progress/stream,rollback,snapshots}` (and `/api/v1/snapshots/{recent,prune,{id}}`). Removed: the inline routes, `UpdatesHandler` (asked a GitHub repository that doesn't exist and ran `apt-get install nithronos` as the nos user) and the unused `pkg/updates`, `pkg/snapcfg` and `pkg/snapdb`. The agent now runs the upgrade as the transient unit `nos-update` (`apt-get --with-new-pkgs upgrade`, keeping changed config files): before, apt-get was a child of nos-agent tied to the HTTP request, so upgrading nos-agent or nosd killed it halfway through dpkg. nosd keeps the progress in `/var/lib/nos/updates/state.json` and follows the unit again after the upgrade restarts it. The pre-update snapshot is `/etc/nos` (it used to be a tar of `/srv`, `/mnt` and every pool, i.e. all user data); rollback restores it and restarts nosd. Not done: downgrading packages and a whole-system rollback (needs a Btrfs root and booting from a snapshot); the channel is recorded but nOS has no package repository to switch yet (see the domain question below).

**Apps**: done in phase 4. Log lines (`/apps/{id}/logs`, `follow=1` as server-sent events) come from `docker compose logs`; `/apps/catalog/{id}/schema` serves the template's `schema.json` and the install wizard uses it instead of its hard-coded `mockSchemas`. Git catalog sources are not supported (a remote catalog only adds entries whose templates must already be installed); the error says so.

**Apps (found by the NithronAI session, see `NAI_INTEGRATION.md`)**: done in phase 4, except the last point. The lifecycle ran privileged steps as the nos user (mkdir in /srv/apps, chown, systemctl, the snapshot script, writing Caddy snippets), so installing an app could not work on an installed system. nos-agent now has `/v1/apps/op` (prepare, start/stop/restart/disable, pull, snapshot, rollback, remove, proxy/unproxy) on validated app IDs; the dangerous `/v1/systemd/install-app` (any unit text, started as root) and the unused compose endpoints are gone. The data folder belongs to the container user (PUID/PGID, 1000 by default), the config folder to nos. `nos-app@.service` uses `/srv/apps/<id>` as the project directory, so `./data` is the snapshotted data subvolume. The Caddy snippet proxies to `127.0.0.1:<published port>` (Caddy on the host can't resolve container names, so the jellyfin name mismatch no longer matters), the Caddyfile imports `Caddyfile.d/app-*.caddy`, and its admin API moved from `off` (which made `systemctl reload caddy` fail) to a socket in `/var/lib/caddy`; existing Caddyfiles are updated the first time an app is proxied. The renderer uses the catalog's `defaults.resources`; category icons match the catalog's lowercase keys. Still open: read access to shares for indexing apps (nAI) — a share can grant `group:<name>`, but the template needs that group's GID for `group_add`; to agree with the nAI session.

**Users / RBAC**: minimal groups and policy done in phase 4: `/api/v1/groups` (CRUD; a group a share uses can't be deleted; deleting a user takes them out of groups and shares) and `/api/v1/policy` (`shares.guest_access_forbidden`). A groups management page is Phase 5.

**System**: not done in phase 4. `/system/config` and `/settings/advanced` are only called by the unrouted `SettingsConnected.tsx` (Phase 5 decides whether that page survives); real per-service CPU and log data (`services_health_handler.go`, `system_handler.go`) moves to Phase 5 with the pages that show it.

**NithronSync server**: done in phase 4 except the last point. The sync API authenticates itself (device token, or the browser session with CSRF) and is mounted outside the session group, so sync clients reach it; device tokens validate and survive restarts (the cache key, the scan and the saved hashes were all broken); every share-scoped endpoint checks the same access rule as Samba. The API tokens of `pkg/auth/token.go` were removed in phase 3 (nothing used them; nAI's contract signs in with a session). Moved to Phase 7 with the clients: offline queue processing (`sync/offline/manager.go` "simulate processing") and delta upload (`sync-core/engine.go` uploads whole files).

---

## Phase 5: Web UI

1. **One API client.** Today there are `api/client.ts`, `api/http.ts`, `lib/nos-client.ts`, `lib/api.ts`, `hooks/use-api.ts` and per-feature `api/*.ts`. Keep `lib/nos-client.ts` (baseURL `/api`) plus generated OpenAPI types; port the callers and delete the rest. **Done:** `lib/nos-client.ts` is the only transport. `api/client.ts`, `api/schema.ts`, `api/http.ts`, `lib/api.ts`, `hooks/use-api.ts` and `hooks/use-shares.ts` are gone; each area has a typed module in `api/` (users, system, storage, shares, groups, backup, apps, monitor, updates, net, sync). The types are written by hand from the Go structs: the route dump's OpenAPI has paths but no schemas, and the old generated `schema.ts` came from a spec that no longer existed.
2. **FE/BE contract**: for each of the ~79 missing paths, implement it (Phase 4) or delete the caller. Make `scripts/verify-fe-be-contract.*` a required CI check against the live route dump (`cmd/route-dump`).
3. **Orphan and duplicate pages**: decide which variant wins and delete the other:
   - `SettingsNetwork` (1295 lines, orphan) vs `NetworkSettings` (routed)
   - `Settings2FA` vs `TwoFactorSettings`
   - `SettingsUsers` vs `Users`
   - `Shares` vs `SharesList`
   - `Apps` vs `AppCatalog`
   - `SettingsUpdates` vs `Updates`
   - `SettingsConnected`, `PoolSnapshots`
4. **Remove mock data**: `Apps.tsx:32` (marketplace), `AppInstallWizard.tsx:54` (schemas), `MonitoringDashboard.tsx:191` (service status); `AppShell.tsx:102` (health badge disabled); `SettingsNetwork.tsx:999` (WireGuard peer dialog TODO).
5. **Setup wizard**: OTP countdown and regeneration messaging (Phase 1); handle the token expiry (10-minute setup token) gracefully mid-wizard.
6. **Live UI pass**: run the app against a real nosd + agent in a VM and walk every nav entry (`config/nav.ts`), logging console errors, 404s, crashes and dead buttons. This is how the "broken UI elements" list gets made concrete; most aren't visible from static reading. Fix and add Playwright coverage (`tests/e2e/ui`) for each page.
7. `react-router-dom` 6 has a moderate advisory fixed only in 7 (major upgrade); do it with the routing cleanup.
8. Error and empty states: every page should show a typed error (backend `httpx.WriteTypedError` codes), never a blank card or a crash when an endpoint 404s or 503s.


**Phase 5 status (done):**
- (1) One client: see above.
- (2) Every path the UI calls is served (`fe-known-missing.json` is empty). The check also flags client calls outside `/v1`, after it missed `/api/pools` and `/api/pools/{id}/snapshots`.
- (3) The orphan pages are removed. Where the routed page was the stub (Users, Settings), the real one was written against the API.
- (4) No mock data is left in the routed UI. The services list, the alerts bell, app snapshots and dashboard disk health are real; the dashboard's made-up event and scan time are gone.
- (5) The setup wizard survives its 10-minute token.
- (6) The live UI pass (`scripts/live-ui/run.sh`) is the CI "UI E2E" job: real nosd and nos-agent in a container, and every page and the first pool's page opened by Playwright.
- (7) react-router 7.
- (8) Errors show the backend's typed message (the client read the wrong JSON shape before), in the page's cards.

Also done in phase 5:
- Apps can read shares through `nos-app-<id>` (nAI).
- Users and SMB accounts are kept in step, with a self-service password change.
- Only admins change system settings.
- There is a Groups page.
- One notification system remains (alerts).
- Pool discovery works as the nos user.
- Pool ids are URL-safe.
- SSE streams work.
- `lsblk` runs on Debian 12.

---

## Phase 6: Packaging, systemd and ISO

1. **One copy of each unit** in `deploy/systemd/`, consumed by both the debs and the ISO. Today `nosd.service` and `nos-agent.service` differ between `deploy/` and `packaging/iso/.../etc/systemd/system/`, and the ISO copies override the packaged ones.
2. **nos-agent sandbox** that actually permits its job:
   - drop `PrivateDevices=yes` (it needs `/dev/sd*` for mkfs, cryptsetup and smartctl)
   - `ProtectSystem=strict` needs `ReadWritePaths=` for `/etc/fstab /etc/crypttab /etc/samba /etc/exports /etc/nftables* /etc/wireguard /etc/caddy /srv /mnt /var/lib/nos`
   - add the capabilities it needs (`CAP_CHOWN CAP_DAC_OVERRIDE CAP_FOWNER CAP_NET_ADMIN CAP_SYS_RAWIO CAP_MKNOD`)
   - `RestrictAddressFamilies=AF_UNIX` blocks its own registration call to `127.0.0.1:9000` and docker compose networking
3. nosd: `ProtectSystem=full` is fine with `ReadWritePaths=/etc/nos`. Add `RuntimeDirectory=nos` for `/run/nos`. Remove the writes to `/tmp` (it has `PrivateTmp`).
4. Decide on `installer/nos-installer` (a Go installer that isn't built or shipped; its module also has no `go.sum` entries, so it doesn't build) and the live-system `/usr/sbin/nos-installer` shell script (the rename replaced the placeholder with the former `nithronos-installer` script): delete both, or make one a supported alternative to the Debian installer.
5. Prune `hooks/normal/` (25 hooks, several overlapping: `10-enable-services`, `91-enable-nos-services`, `95-ensure-caddy-enabled`, `041-enable-otp-announce`, `96-setup-otp-generator`) and the preset file.
5. Caddy: verify the shipped Caddyfile serves the UI and proxies `/api` and WebDAV, with the self-signed TLS path (`nos-tls-gen`).
6. Secure Boot: track shim signing as a known limitation. **Done (documented):** the ISO uses Debian's signed shim, GRUB and kernel; Secure Boot isn't tested in CI (OVMF with enrolled keys could be added to the ISO install test), and ZFS (DKMS, unsigned) needs MOK enrollment or Secure Boot off (`docs/installer.md`).
8. From phase 4: the ISO keeps its own copies of `nosd.service` and `caddy.service` in `includes.chroot` that differ from `deploy/systemd` (the ISO's nosd has no sandbox at all); use the packaged units. (Phase 4 already removed the ISO's Python stand-in `nos-agent`, which overwrote the Go agent from the deb so every agent endpoint returned 404 on ISO installs, and its unit override.)
7. From phase 4: give the WireGuard and HTTPS managers agent operations (like `/v1/firewall/*`); make HTTPS edit only its own Caddy site block instead of regenerating the Caddyfile; decide which ACME challenges nOS supports with Debian's caddy; no outside service for the public address without the admin's consent.


**Phase 6 status:**
- (1, 8) One copy of each unit. The ISO's nosd, nos-agent, caddy, TLS and service drop-ins are gone; the packages' units are used, and Caddy gets a `caddy.service.d/nos.conf` drop-in.
- (2) nos-agent's sandbox was fixed in phase 4: it keeps the hardening that doesn't block its jobs.
- (3) nosd: AF_NETLINK, a RuntimeDirectory, write access to `/etc/nos` and `/var/lib/nos`. The HTTPS manager's `/tmp` write is gone with its rewrite.
- (4) The Go installer and the live `nos-installer` script are removed; the Debian installer is the supported path.
- (5) The live-build hooks are pruned to the build plumbing (kernel, GRUB theme), and the unused presets are gone.
- (5b) Caddy: one Caddyfile, shipped and installed by the `nos` package. It proxies `/api` and `/dav`, has no HSTS, and has a per-machine TLS key (the ISO baked one into the image).
- (6) Secure Boot is documented as a known limitation.
- (7) WireGuard and HTTPS run through the agent (`/v1/wireguard/*`, `/v1/caddy/public-site`). ACME uses HTTP-01 only. No outside service is asked for the public address.
- The firewall works for the first time. One table (`inet nos`) is loaded at boot by `nos-firewall.service`, leaves Docker alone, and the rendering bugs are fixed (a bare `accept`, UDP rules written as TCP).
- The ISO install test fails on any unit that fails at boot and checks WebDAV through Caddy.

Carried forward:
- nosd needs Docker for app logs, status and health checks, but `nos` isn't in the `docker` group, which is root-equivalent anyway. These should become read-only agent operations; until then they don't work on installed systems.
- Secure Boot isn't tested in CI.

---

## Phase 7: NithronSync clients (re-scope honestly)

- `clients/mobile` has no Xcode/Gradle projects beyond a manifest and plist, so it can't be built. The QR scanner is a mock (`QRScannerScreen.tsx:29`).
- `clients/desktop` is a Wails skeleton (tray "navigate to settings" TODO, placeholder icon).
- `clients/sync-core`: the CLI has "for now" stubs, and uploads are full files rather than deltas. It now compiles (phase 0), but golangci-lint reports 28 issues, mostly ignored errors in the engine (conflict download, rename, queue writes), which can lose data; fix them and make the linter blocking. The delta matcher recomputes the weak hash at every offset instead of using `RollingHash`.
- `clients/desktop` doesn't build (no `go.sum`, needs the Wails toolchain and a built `frontend/dist`); add it to CI.

Order (all required for v1): finish sync-core and the server side (Phase 4) together with a CLI client and an end-to-end test first, then the desktop client, then mobile (generate real iOS/Android projects, real camera QR scanning). Correct the README, which currently marks Pre-v1/NithronSync and A10 as complete.

---

## Phase 8: Testing and CI

- The storage E2E, Playwright, ISO smoke and docs jobs still swallow failures with `|| echo "... not yet implemented"`. Make each one real and blocking.

- QEMU install-and-boot test (Phase 1) as a required check on PRs touching `packaging/`.
- FE/BE contract check required.
- Integration tests for nosd and the agent on a loopback-device Btrfs pool (`tests/e2e/storage-test.sh` exists; make it run in CI).
- Playwright UI smoke test of every routed page.
- Upgrade test (`tests/upgrade`) for snapshot-before-update and rollback once Phase 4 wires it.
- From phase 5: `NOS_TEST_SKIP_AUTH=1` turns off sign-in, admin and CSRF checks in the shipped binary (`routes_protected.go`, `deps.go`, `sync_handler.go`). Tests should inject a test router instead, so no environment variable can disable authentication on a real system.

---

## NithronAI (nAI) integration constraints

nAI (`../NithronAI`, developed separately) will ship first as a NithronOS catalog app and later as the built-in assistant (an MCP server over the NithronOS API). The revision must keep these working:

- **App catalog** (Phase 4): accept nAI's template in `NithronAI/packaging/nithronos/` as-is: `catalog-entry.yaml`, `nai/compose.yaml` (Compose v2), `nai/schema.json`, `icon.svg`. Container naming `nos-app-<id>-<service>-1`, and the `SHARES_PATH`, `PUID`/`PGID` and port variables. `/apps/schema/{id}` must serve the real `schema.json`.
- **Shares root**: keep shares under `/srv/shares` (nAI mounts it read-only).
- **API contract**: keep `nos_client_contract.json` generated from the real routes and versioned (Phase 3.10); nAI's MCP server is built from it.
- **Non-browser auth** (Phases 2–4): scoped API tokens for services (read-only by default), using the same bearer-token path as the NithronSync device tokens instead of cookie + CSRF.
- **Groups and share ACLs** (Phase 4, minimal groups): expose group membership and per-share ACLs over the API so nAI can map share permissions to its own groups.
- **Later**: NithronOS account sign-in for nAI (OIDC, README A4).

## Roadmap alignment

The README overstates completion. Proposed re-baseline:

| Release | Contents |
|---|---|
| **v0.9.6-pre-alpha: "it installs"** | Phases 0, 1, 2 + Phase 6 items 1–3. Installed system boots, shows OTP, setup works, no critical auth holes. |
| **v0.10-alpha: "it's honest"** | Phases 3, 5 and the core of 4 (storage, shares, dashboard, monitoring, backup wiring, updates/snapshots). No mock data, no dead endpoints. |
| **v1.0** | Rest of Phase 4, Phase 7 (NithronSync server, sync-core, desktop **and mobile**), Phase 8 complete. Matches `RELEASE-CHECKLIST-V1.md`. |
| **Post-v1 A1–A10** | Keep as listed. Un-tick **A10 (Sync v2)** and the **Pre-v1/NithronSync** checklist until Phase 7 lands. Note that A3 (cloud backup), A6 (WireGuard/QR) and A8 (alerts) depend on Phase 4 wiring that is half done today. |

---

## Decisions (maintainer, 2026-11-21)

1. **OTP**: 6 digits, 15-minute TTL, rotated automatically by nosd until an admin exists.
2. **Docker and Caddy**: Docker from Docker's official apt repo (`docker-ce`, `docker-compose-plugin`), added by the installer; all code standardises on `docker compose` (v2) instead of the mix of `docker-compose` v1 and v2 calls found today. Caddy from Debian `main`.
3. **ZFS**: installed by default (`zfs-dkms` + `zfsutils-linux` from `contrib`, with `linux-headers-amd64`).
4. **Duplicate UI pages**: chosen during Phase 5 by keeping whichever variant is wired to real endpoints and has more of the functionality; the other is merged in and deleted.
5. **Groups/policy (A4) and DNS settings**: build minimal working versions now.
6. **NithronSync desktop and mobile clients**: required for v1 (not deferred). Phase 7 is in scope for v1.
7. **Workflow**: one branch and one pull request per phase (`revision/phase-N-<slug>`).
8. **Name**: nOS, with NithronOS as the long form. Everything is renamed, including package names, paths and the GitHub repository (Nithron/nOS); its own PR after Phase 3, with a migration for existing installs.

Decided (2026-11-27): nOS's domain is **nos.nithron.com**: the documentation at `https://nos.nithron.com/docs/`, the APT repository at `https://nos.nithron.com/apt` (`docs/updates.md`, `docs-site/mkdocs.yml`, the mobile app's docs link). Company pages and contact addresses stay on nithron.com.

Open for later: Debian base. The ISO targets bookworm (Debian 12, now in LTS); moving to trixie (Debian 13) belongs in Phase 6.
