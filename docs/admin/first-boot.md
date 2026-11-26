### First-boot setup code

Until the first administrator exists, `nosd` keeps one valid 6-digit setup code. It is the only component that creates, shows or removes it.

- **Where it is shown**: above the login prompt on the machine's screen and serial console (nosd writes `/run/nos/firstboot.issue`, which `/etc/issue.d/50-nos-setup.issue` links to; `nos-issue-reload.path` refreshes the prompts), and once per code in the nosd journal:

  ```
  sudo journalctl -u nosd | grep 'setup code' | tail -n 1
  ```

- **Lifetime**: 15 minutes. When it expires, or after 5 wrong guesses, nosd issues a new code and the screen updates by itself. No restart is needed.
- **State file**: `/var/lib/nos/state/firstboot.json` (0600, owned by `nos`). It is removed when the first administrator is created, and the console banner goes away.
- **API**: `GET /api/v1/setup/state` returns `{ "firstBoot", "otpRequired", "expiresAt" }`. Once an administrator exists, the `/api/v1/setup/*` endpoints return 410 Gone with `{ "error": { "code": "setup.complete" } }`.

### HTTPS on first boot (self-signed)

On first boot, a local ECDSA certificate is generated with SANs for `127.0.0.1`, the primary IPv4, and `nos.local`. Caddy serves HTTPS using this cert. Browsers will warn because it is self-signed; this is expected. The certificate and key live under `/etc/ssl/nos/cert.pem` and `/etc/ssl/nos/key.pem`.

The console shows a banner after services start:

`nOS UI → https://<this-ip>/  (self-signed)`

To get a new code right away (local recovery mode only): `POST /api/v1/recovery/generate-otp`.

Where the local cert lives: `/etc/ssl/nos`.

#### Recovery endpoint (localhost only)

For recovery (admin/local only), you can reset first-boot state via:

`POST /api/setup/recover`

Body:

```
{ "confirm": "yes", "delete_users": false }
```

- `confirm` is required and must be `yes`.
- If `delete_users` is true, `/etc/nos/users.json` is removed. Use with care.

Access is restricted to localhost (127.0.0.1/::1).

#### Setup state rules

- Setup is considered complete only if the users database loads and contains at least one user (admin).
- If the users file is missing, empty, or invalid, setup remains in first-boot.
- While no administrator exists, a valid code always exists; an expired code is replaced on the next check.

# First Boot (Admin)

On first boot, `nosd` keeps a one-time 6-digit setup code (see above) and shows it on the console. The web UI detects setup state and routes to `/setup`.

## Flow
1) Setup code: enter the code from the server screen (valid 15 minutes). The UI receives a temporary setup token (memory only).
2) Create Admin: Choose a username and a strong password.
3) Optional 2FA: Enroll TOTP, confirm a 6‑digit code, and save recovery codes securely.
4) Finish: You can now sign in at `/login`.

After the first admin is created, `/api/setup/*` endpoints return `410 Gone`.

New code: wait for the current one to expire (the screen shows the new one), or use the local recovery endpoint above.

## Data paths
- Users: `/etc/nos/users.json`
- Secrets: `/etc/nos/secret.key` (32 bytes, 0600)
- First‑boot state: `/var/lib/nos/state/firstboot.json`

## Reset (dev/recovery)
To rerun first‑boot on next start, remove the users DB:
```bash
sudo rm -f /etc/nos/users.json
```
Restart `nosd`; a new setup code appears on the console. Repeat setup.
