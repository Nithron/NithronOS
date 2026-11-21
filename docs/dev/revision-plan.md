# NithronOS Revision Plan

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
| Installer / ISO | BIOS and UEFI boot different preseeds (one interactive), package install failures are ignored, no security mirror. Four preseed files. |
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

1. **The installer path is inconsistent and its result is unverified.** (Corrected after re-reading `build.sh`: an earlier draft said installed systems get no nosd at all, which is probably wrong.) `build.sh` runs live-build with `--debian-installer live`, so the installer copies the *live filesystem*, which has the NithronOS debs installed by the `20-install-local-debs` hook, onto the disk. What can still go wrong:
   - That hook ends in `|| true`, so if `dpkg -i` of nosd fails the ISO still builds, without nosd.
   - **BIOS and UEFI use different preseeds**: isolinux uses `preseed/nithronos.cfg` with `priority=critical`; GRUB (UEFI) uses `/cdrom/preseed.cfg` with `priority=low`, i.e. an interactive install with different `late_command` and `pkgsel` settings. Two more preseeds (`includes.installer/preseed.cfg`, `debian-installer/preseed.cfg`) are not referenced by any boot entry.
   - `auto/config` and `build.sh` both configure live-build; `auto/config` adds the Docker repo and a 100-package list (nginx, golang, nodejs, npm, build-essential, aide, rkhunter, cloud-init, ...).
   - `build.sh` disables the Debian **security** mirror (`LB_SECURITY=false`), so installed systems get no security updates unless something else adds `bookworm-security`.

   The first Phase 1 step is therefore to build the ISO and do a BIOS and a UEFI install in QEMU, and record what the target actually has (nosd installed and enabled, units present, OTP on console).
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
- **Installer**: one preseed for BIOS and UEFI, one live-build config (fold `auto/config` into `build.sh` or the reverse). Keep the live-installer approach, but make the deb install hook fail the build when a NithronOS package doesn't install, and verify in `late_command` that `nosd`, `nos-agent` and `caddy` are installed and enabled on the target. Trim the package list to what an appliance needs. Restore the Debian security mirror. Docker from Docker's apt repo (kept in the target's sources), Caddy from Debian. Fix `pkgsel`.
- Fix the Setup UI copy and add an expiry countdown.
- **QEMU acceptance test** (`tests/smoke/iso-boot-test.sh`): install to a virtual disk, reboot, assert that the serial console shows a 6-digit OTP and that `/api/v1/setup/otp/verify` accepts it. Run it in CI.

---

## Phase 2: Security fixes (CRITICAL)

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

Add regression tests for each (auth flow tests exist in `router_flow_test.go` to extend).

---

## Phase 3: Backend restructuring

The backend grew by adding parallel implementations instead of finishing one. Consolidate before adding features, so features land once.

1. **Dead packages**: the duplicates `internal/{backup,firewall,https,wireguard,observability}` were deleted in phase 0. Still unused: `pkg/appsdk` (app manifests, README A9), `pkg/validate` (share names and path checks; reuse it for input validation), `pkg/webhooks` (reuse it for notification delivery). Wire or delete each. Decide on `pkg/updates` (1.7k LOC, unused) and `pkg/snapcfg` (unused, although the README advertises "snapshots on update, enabled by default"): wire them in Phase 4 or delete them.
2. **One user store**: today `auth.NewStore` (`pkg/auth`), `userstore` (`internal/auth/store`) and `pkg/auth.UserManager` all read and write `/etc/nos/users.json`. Keep `internal/auth/store` and port the TOTP and recovery logic into it.
3. **One session system**: `internal/sessions`, `internal/auth/session` and `pkg/auth/session_manager.go`, plus a legacy `SessionCodec` cookie. Pick opaque server-side sessions, and make `requireAuth` verify them (today the binding middleware explicitly "does not enforce").
4. **One shares implementation**: `internal/shares`, `pkg/shares`, `SharesHandlerV1` (mock fallback) and `SharesHandlerV2`.
5. **One network stack**: `pkg/net` and `pkg/firewall` vs the dead `internal/*`; merge `net_handler.go` (stub TODOs) and `network_config_handler.go` into one API.
6. **One apps layer**: `internal/apps` wraps `pkg/apps`, and there is a fallback `apps.Catalog`. Fail fast instead of silently registering stub routes.
7. **Split `router.go` (2.5k lines)** into `routes_auth.go`, `routes_setup.go`, `routes_storage.go`, and so on, each with a `Register(r chi.Router, deps)` function. A dependency container is built once in `main.go`. Remove duplicate registrations (`/storage/devices` twice, `/health/*` both public at the root and mounted under auth, the `/system/*/…` wildcard duplicates in the route dump).
8. **No silent mock fallbacks**: return `503 agent.unavailable` instead of fake disks (`router.go:1093`), fake SMART (`smart_handler.go:110`), fake jobs (`jobs_handler.go:213`) or sample health (`health_handler.go:75,99`).
9. Remove dead `ensureFirstBootOTP` in `main.go`; propagate `context` instead of `context.TODO()` (~25 sites).
10. **API contract as the source of truth**: generate `docs/api/openapi.yaml` from the routes (or the reverse), make `/api/v1/openapi.json` serve it, and generate the FE types from it.

---

## Phase 4: Implement missing backend functionality

Grouped by feature. Each item lists the evidence.

**Backup & replication**: `BackupHandler` exists but `router.go:197` sets it to `nil`, so no `/api/v1/backup/*` routes exist, yet the UI (`api/backup.ts`, `Remote.tsx`) calls 15+ of them. Wire the scheduler, replicator and restorer. Fill `replication.go:382-586` (placeholder paths, `Progress = 50`) and `restore.go:226`.

**Monitoring & alerts**: `pkg/monitor` uses the cgo-only `mattn/go-sqlite3`, but the nosd .deb is built with `CGO_ENABLED=0`, so its storage fails at runtime; switch to pure-Go `modernc.org/sqlite`. `NewMonitorHandler` is never mounted; the UI calls `/v1/monitor/{overview,devices,btrfs,services,timeseries,alerts/*}`. Implement alert template processing (`alerts/engine.go:364`) and notification webhook/syslog delivery (`notifications/manager.go:332,338`).

**Storage**: `pools.json` has two incompatible schemas: `pools_options.go` writes `{records:[...]}` while `pools_import.go` and `pools_apply_create.go` write a bare list, so each overwrites the other's data. Pick one store. Balance start/cancel (`balance_handler.go:75,104`), scrub cancel (`router.go:1143`), SMART self-test trigger (`smart_handler.go:283,344`), `/pools/{id}/subvolumes`, and the mock pools/devices in `storage_handler.go:270,302`.

**Dashboard**: shares and apps widgets return mock data (`internal/api/dashboard_handler.go:192,253,271`).

**Network / remote access**: `net_handler.go:474-633` are TODOs/placeholders (store WG/HTTPS config, enable WG, configure HTTPS, apply firewall, interface/port enumeration). Real WG keypairs. Implement or remove the UI-only endpoints (`/network/dns`, `/network/https/{generate,upload,toggle}`, `/network/wireguard/*`, `/firewall/*`).

**Updates**: UI calls `/updates/{channel,progress,progress/stream,snapshots,version}`, which don't exist. Decide on `pkg/updates` + `pkg/snapcfg` (pre-update snapshot + rollback) and wire them; `updater.go:593` "just mark it as failed".

**Apps**: log streaming (`internal/apps/manager.go:292`, `apps_handler.go:306`), `/apps/schema/{id}` (the UI uses hard-coded `mockSchemas`), and git catalog sources (`pkg/apps/catalog.go:173`).

**Users / RBAC**: `/v1/groups` and `/v1/policy` are called by the UI but don't exist. Decide scope (README A4) and either implement minimal groups or remove them from the UI.

**System**: `/system/config` and `/settings/advanced` (used by `SettingsConnected.tsx`); real service CPU/log data (`services_health_handler.go:276,322`, `system_handler.go:142,154`).

**NithronSync server**:
- Move the device-token routes **out of** the cookie+CSRF group. Today `/api/v1/sync/*` sits under `requireAuth` + `requireCSRF`, so bearer-token clients always get 401.
- Persist tokens (`pkg/auth/token.go:485`, memory only).
- Implement offline queue processing (`sync/offline/manager.go:412-472` "simulate processing") and delta upload (`sync-core/engine.go:810` does full uploads).

---

## Phase 5: Web UI

1. **One API client.** Today there are `api/client.ts`, `api/http.ts`, `lib/nos-client.ts`, `lib/api.ts`, `hooks/use-api.ts` and per-feature `api/*.ts`. Keep `lib/nos-client.ts` (baseURL `/api`) plus generated OpenAPI types; port the callers and delete the rest.
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

---

## Phase 6: Packaging, systemd and ISO

1. **One copy of each unit** in `deploy/systemd/`, consumed by both the debs and the ISO. Today `nosd.service` and `nos-agent.service` differ between `deploy/` and `packaging/iso/.../etc/systemd/system/`, and the ISO copies override the packaged ones.
2. **nos-agent sandbox** that actually permits its job:
   - drop `PrivateDevices=yes` (it needs `/dev/sd*` for mkfs, cryptsetup and smartctl)
   - `ProtectSystem=strict` needs `ReadWritePaths=` for `/etc/fstab /etc/crypttab /etc/samba /etc/exports /etc/nftables* /etc/wireguard /etc/caddy /srv /mnt /var/lib/nos`
   - add the capabilities it needs (`CAP_CHOWN CAP_DAC_OVERRIDE CAP_FOWNER CAP_NET_ADMIN CAP_SYS_RAWIO CAP_MKNOD`)
   - `RestrictAddressFamilies=AF_UNIX` blocks its own registration call to `127.0.0.1:9000` and docker compose networking
3. nosd: `ProtectSystem=full` is fine with `ReadWritePaths=/etc/nos`. Add `RuntimeDirectory=nos` for `/run/nos`. Remove the writes to `/tmp` (it has `PrivateTmp`).
4. Prune `hooks/normal/` (25 hooks, several overlapping: `10-enable-services`, `91-enable-nos-services`, `95-ensure-caddy-enabled`, `041-enable-otp-announce`, `96-setup-otp-generator`) and the preset file.
5. Caddy: verify the shipped Caddyfile serves the UI and proxies `/api` and WebDAV, with the self-signed TLS path (`nos-tls-gen`).
6. Secure Boot: track shim signing as a known limitation.

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

Open for later: Debian base. The ISO targets bookworm (Debian 12, now in LTS); moving to trixie (Debian 13) belongs in Phase 6.
