# Remote Access & Firewall Toggle — Requirements (v0)

## Goal
Provide a safe, guided way to switch exposure of the web UI between **lan-only** (default), **vpn-only**, **tunnel**, and **direct** modes with plan → confirm → apply → rollback.

## Modes
- **lan-only**: allow 443 (+22 optional) from RFC1918 only.
- **vpn-only**: allow 443 from VPN subnets (placeholder CIDRs now; detect actual WG/Tailscale later).
- **tunnel**: rely on Cloudflare Tunnel; keep nftables as lan-only.
- **direct**: allow 443 (and 80 for ACME if chosen) from anywhere. Force 2FA.

## Backend (`nosd`) API
- `GET /api/firewall/status` → `{ mode, nft_present, ufw_present, firewalld_present, last_applied_at }`
- `POST /api/firewall/plan { mode }` → returns textual summary of rules that would be applied.
- `POST /api/firewall/apply { mode, twoFactorToken? }` → applies after checks.
- `POST /api/firewall/rollback` → restores previous ruleset if backup exists.

### Preconditions & Safeguards
- Non–lan-only modes require **2FA** enabled for the requesting admin.
- Detect and block if **UFW** or **firewalld** is active (return 409 with instructions).
- Validate ruleset with `nft -c -f` before apply.
- Backup current ruleset: `nft list ruleset > /etc/nos/firewall/backup-<ts>.nft`.
- Persist rules in `/etc/nftables.d/nos.nft` and ensure `nftables.service` enabled.

## Agent (`nos-agent`)
- `POST /v1/firewall/apply { ruleset_text, persist: true }`
  - Write temp file, **check** (`nft -c`), **backup**, **apply**, **persist**. Return `{ ok, backup_path }`.
  - Hard limit input size (≤ 200 KB), sanitize, log to journal (AUTHPRIV).

## UI (Settings → Remote)
- Show **current mode** and system detection (nft/UFW/firewalld).
- Mode selector (radio): lan-only / vpn-only / tunnel / direct.
- **Plan** button → shows rules summary modal.
- **Apply** button → if mode ≠ lan-only, require TOTP entry; show progress + success/fail.
- **Rollback** button → enabled if a backup exists from the last change.
- Prominent warnings for **direct** mode; link to enabling 2FA.

## Acceptance Criteria
- Changing mode from UI updates status within 3s.
- Backups are created on every apply; rollback restores prior behavior.
- Applying with UFW/firewalld active yields a clear error and help text.
- Direct mode is blocked if 2FA is not enabled.

## Out of Scope (v0)
- Automatic detection of actual VPN CIDRs (stub values OK).
- Cloudflare Tunnel onboarding flow (separate requirement).


