# nAI app for NithronOS: integration handoff

> Written 2026-11-21 by the Claude session working on the **NithronAI** repo
> (`/mnt/vault/Dev/Nithron/NithronAI`, github.com/Nithron-Intelligence/nAI), for the Claude session working on NithronOS.
> The user asked for nAI to be added to the NithronOS app catalog and for this note to explain it.
> Delete this file once the work below is done.

## What nAI is

nAI is "the AI that knows your NAS": it indexes folders (NithronOS shares) **in place and read-only**, keeps the index in
sync (inotify watcher + periodic incremental rescans), and answers questions with citations (file, page, section),
using hybrid keyword + multilingual semantic search. Answers come from an optional LLM (Ollama on the NAS or on a PC
with a GPU, any OpenAI-compatible server, or a cloud API) or, without one, as exact quotes. It runs CPU-only, as one
container (FastAPI API + web UI on port 8000), with all state in one SQLite file. Same license as NithronOS (NCL v1.0).
Version 0.4.0 is on NithronAI `main` (commit `6686b59`).

## What I added here (uncommitted, in your working tree)

| Path | What |
|------|------|
| `usr/share/nithronos/apps/templates/nai/compose.yaml` | the app's compose template |
| `usr/share/nithronos/apps/templates/nai/schema.json` | install form (JSON Schema, all values strings) |
| `usr/share/nithronos/apps/templates/nai/README.md` | user docs (your CI wants a README per template) |
| `usr/share/nithronos/apps/templates/nai/icon.svg` | icon (entries reference icons, though the UI shows category icons) |
| `usr/share/nithronos/apps/catalog.yaml` | new `nai` entry appended at the end; nothing else changed |
| `NAI_INTEGRATION.md` | this file |

Nothing else in the repo was touched. Your branch at the time was `revision/phase-1-firstboot` with a clean tree.
**Please commit these in a separate commit (not mixed into the firstboot work)**, e.g.
`feat(apps): add nAI to the app catalog`. Ask the user before committing/pushing; in the NithronAI repo they want commits
made as them with no Claude attribution, so check whether the same applies here.

### How it's adapted to NithronOS (from reading `pkg/apps/renderer.go`, `lifecycle.go`, `nos-app@.service`)

- **Service name `app`, container `nos-app-nai-app-1`.** `RenderCaddySnippet` always proxies to `nos-app-<id>-app-1`.
  Caddy's `handle_path /apps/nai/*` strips the prefix and `flush_interval -1` keeps nAI's SSE streaming working; the
  nAI web UI uses relative URLs, so it works under `/apps/nai/` as well as on its own port.
- **Absolute data path `/srv/apps/nai/data`.** `nos-app@.service` runs `docker compose --project-directory
  /srv/apps/%i/config`, so a relative `./data` would resolve to `/srv/apps/<id>/config/data`, outside the Btrfs
  data subvolume that `ensureDataSubvolume`/snapshots manage.
- **Explicit `deploy.resources.limits` (2 CPUs, 3g by default; `MEMORY_LIMIT`/`CPU_LIMIT` in the form).**
  `applySecurityDefaults` adds 2.0 CPU / 1024M to any service without `deploy`, and the catalog's
  `defaults.resources` is never applied. Measured nAI peak: ~0.9 GB with the embedding model, ~1.4 GB with the
  reranker loaded, before OCR. 1024M would get it OOM-killed.
- **`NAI_TRUSTED_PROXIES=["172.16.0.0/12"]`**: nAI only honours `X-Forwarded-For` from trusted proxies; Caddy's traffic
  should arrive from a Docker network, so rate limiting is per client instead of one bucket for the whole household.
- `${VAR:-default}` everywhere matches your renderer's regex; schema values are strings like the other templates.

### Verified

- Catalog YAML and schema JSON pass the same checks as your CI (`App Catalog templates validation`).
- Ran **your Go code** on it from a scratch module (`replace nithronos/backend/nosd => .../backend/nosd`):
  `LoadBuiltinCatalog` → `ValidateParams` → `RenderComposeFile` for defaults and for a "GPU PC" parameter set.
  Rendered output keeps the 3g limit, maps params correctly, rejects `MEMORY_LIMIT=lots`; `docker compose config`
  accepts the rendered files.
- In the NithronAI repo: the same image ran with these settings, read-only shares, PUID/PGID, OCR, and health checks.
- **Not verified**: installing through nosd on a real NithronOS box (the image is now public, so this can be tested).

## Blockers / things for you to fix or decide

1. ~~**Image not published yet.**~~ **Resolved 2026-11-22:** `ghcr.io/nithron/nithronai/nai-core:0.4.0`
   (also `:0.4`, `:0`) is published and **public** (linux/amd64 + linux/arm64); an anonymous pull and start were
   verified. Release: https://github.com/Nithron-Intelligence/nAI/releases/tag/v0.4.0
2. **Data folder ownership.** `setAppOwnership` runs `chown -R nos:nos /srv/apps/<id>`, but app containers run as
   `PUID:PGID` (1000:1000 by default, here and in jellyfin/code-server). Unless `nos` happens to be uid 1000, nAI can't
   write `/srv/apps/nai/data` (its DB, uploads and model cache). Options: chown the data dir to the app's PUID:PGID at
   install, or make the renderer provide `NOS_UID`/`NOS_GID` so templates can default to them.
3. **Read access to shares.** nAI reads `/srv/shares` as `PUID:PGID`. Files it can't read are skipped and logged.
   Shares are ACL-controlled (owners/readers, users/groups), so nAI needs a principal with read access to the shares an
   admin wants indexed, e.g. a dedicated `nai` group added to share readers, passed via `group_add` / `PGID`.
4. **Existing templates have the same issues as above** (worth fixing while you're there):
   - Relative `./data/...` volumes (code-server, jellyfin, n8n, nextcloud) land in `config/data`, not the
     snapshotted data subvolume.
   - `jellyfin`'s `container_name` is `nos-app-jellyfin-jellyfin-1`, which doesn't match the Caddy snippet's
     `nos-app-jellyfin-app-1`, so `/apps/jellyfin/` can't reach it (the other templates set `...-app-1` correctly).
   - Catalog `defaults.resources` is ignored by the renderer (always 2.0 / 1024M unless the template sets `deploy`).
   - `AppCatalog.tsx` maps capitalised categories (`'Productivity'`) while the catalog uses lowercase (`productivity`),
     so every app falls back to the generic icon.
5. **Please confirm networking.** The Caddy snippet and the catalog `health.url` address containers by name
   (`nos-app-nai-app-1:8000`). That only resolves if Caddy/nosd share a Docker network with the app. If Caddy runs on the
   host, it should proxy to `127.0.0.1:<host port>` instead. Also check that Caddy's traffic really comes from
   `172.16.0.0/12` (otherwise adjust the `TRUSTED_PROXIES` default).

## Next integration steps (Phase 2 of the nAI plan, after the above works)

- **Share permissions → nAI groups (most valuable).** nAI restricts each *source* to `allowed_groups`; a user sees a source
  when they're in one of its groups (admins see everything). nosd could keep this in sync:
  - sources: `GET/POST /sources`, `PATCH /sources/{id}` with `{"allowed_groups": [...]}`, `POST /sources/{id}/scan`
  - users: `POST /auth/register` (admin token), `PUT /auth/users/{id}/groups` with `{"groups": [...]}`
  - All nAI API calls use `Authorization: Bearer <jwt>` from `POST /auth/login`. OpenAPI at `/docs` on the app.
- **Single sign-on**: today nAI has its own accounts (first account = admin). Plan: accept NithronOS identities.
- **Dashboard embedding**: the UI already works under `/apps/nai/`; a NithronOS dashboard widget could call
  `/ask/stream` (Server-Sent Events: `citations`, `token`…, `done`) and open citations via `/documents/{id}/file`.
- **MCP server for NithronOS** (nAI's Phase 2): nAI will call the NithronOS API (`nos_client_contract.json`) read-only
  first (pool usage, snapshots, backups), then confirmed actions. Keeping that contract current helps.

## Reproduce the render check

```bash
mkdir /tmp/rendercheck && cd /tmp/rendercheck
cat > go.mod <<'EOF2'
module rendercheck
go 1.24
require nithronos/backend/nosd v0.0.0
replace nithronos/backend/nosd => /mnt/vault/Dev/Nithron/NithronOS/backend/nosd
EOF2
cp /mnt/vault/Dev/Nithron/NithronOS/backend/nosd/go.sum .
# main.go: apps.NewCatalogManager(<apps dir>, ...).LoadBuiltinCatalog(), pick entry "nai",
# apps.NewTemplateRenderer(<apps dir>).ValidateParams(...) and .RenderComposeFile(...)
GOFLAGS=-mod=mod go run .
```

Questions about the nAI side go to the user (or the NithronAI session); nAI's own notes are in the NithronAI repo
README and `packaging/nithronos/README.md`.
