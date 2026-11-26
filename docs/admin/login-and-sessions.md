# Auth & Sessions (Admin Guide)

This guide summarizes authentication, sessions, and recovery for NithronOS.

## Login
- Username + password (Argon2id, PHC format)
- Optional TOTP (6 digits, 30s period, ±1 step window)
- Account lockout after repeated failures; generic error responses

## Cookies
- `nos_session`: access cookie, 15 minutes; names the server-side session. httpOnly; SameSite=Lax; Secure
- `nos_refresh`: refresh cookie for the same session, 7 days. With "Remember me" it survives closing the browser; without it, it is a browser-session cookie. httpOnly; SameSite=Lax; Secure
- `nos_csrf`: CSRF token used with `X-CSRF-Token` header for state-changing requests

## Server-side sessions
- Signing in creates a session record (`/var/lib/nos/sessions.json` by default): session id, user, sign-in IP network (/24 for IPv4, /64 for IPv6), browser user agent, created / last-seen / expiry times.
- Every request is checked against it: a signed, unexpired `nos_session` cookie is accepted only while its session exists. Signing out, revoking a session, changing a password or deleting a user therefore takes effect immediately.
- Sessions last 7 days from the last refresh; expired ones are removed.
- Signing out ends the current session only. Changing your password ends your other sessions; deleting a user ends all of theirs.

## Session management APIs
- `GET /api/v1/auth/sessions`: lists own sessions
  - Response: `[{ sid, createdAt, lastSeenAt, expiresAt, ipPrefix, userAgent, current }]`
- `POST /api/v1/auth/sessions/revoke`
  - Body: `{ "scope": "current" | "all" | "sid", "sid"?: "<sid>" }`
  - Current scope clears auth cookies; actions are audit-logged

## Refresh hardening
- `POST /api/v1/auth/refresh` extends the session and replaces the refresh token; the session id stays the same.
- The token that was just replaced is accepted for 30 more seconds, so two tabs refreshing at once don't sign each other out.
- An older refresh token means the cookie was copied: every session of that user ends.
- Sessions from before this scheme (NithronOS 0.9.x) are not carried over; users sign in once more after upgrading.

## Rate limits
- OTP: default 5/min per IP; configurable window and limit
- Login: default 5/15m per IP and per username; persisted across restarts
- Standard 429 error with `Retry-After` header and structured body `{ error: { code: "rate.limited", retryAfterSec } }`

## Recovery
- See [Recovery](recovery.md) for console-only procedures to reset password, disable 2FA, or regenerate a first-boot OTP.
