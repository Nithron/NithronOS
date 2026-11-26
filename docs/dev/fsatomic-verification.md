## fsatomic verification report

### Summary

- fsatomic helper present and used across stores. Tests pass locally.

### Helper presence

- internal package: `backend/nosd/internal/fsatomic`
- Functions:
  - SaveJSON(ctx, path, v, perm): tmp write → fsync(tmp) → fsync(dir) → rename → fsync(dir). Windows-friendly rename with retries; cleans tmp on error.
  - LoadJSON(path, v): removes orphan `.tmp`, returns exists flag.
  - WithLock(path, fn): advisory `.lock` (flock on Unix; create-excl with retries on Windows).

### Stores migrated

| Store | Path | Status | Files |
| --- | --- | --- | --- |
| Users | `/etc/nos/users.json` | OK | `internal/auth/store/store.go`
| First-boot | `/var/lib/nos/state/firstboot.json` | OK | `main.go`, `internal/server/router.go`
| Sessions | `/var/lib/nos/sessions.json` | Not implemented | N/A
| Rate limit | `/var/lib/nos/ratelimit.json` | Not implemented | N/A
| Update state | `/var/lib/nos/updates/state.json` | OK | `internal/updates/service.go`
| Update records | `/var/lib/nos/updates/records.json` | OK | `internal/updates/service.go`
| Shares store | path via `NOS_SHARES_PATH` (devdata in tests) | OK | `internal/shares/store.go`

Notes:
- Sessions and rate limiter JSON stores are not present yet. When added, follow fsatomic pattern.

### Direct JSON writes audit

- Grep confirms no direct writes for these stores remain. One test writes a dev `shares.json` directly (test-only): `internal/server/shares_api_test.go:126`.

### Permissions and locking

- Secrets/state (users/firstboot/shares): 0600 via SaveJSON or default.
- Update state and records: 0640.
- Writers use `WithLock(path, ...)` where cross-process coordination is needed (users, shares, update records append, updates index save).

### Tests

- `internal/fsatomic`: concurrent writers and orphan `.tmp` recovery.
- `internal/auth/store`: concurrent save and crash-style tmp handling.
- All nosd module tests pass locally: `go test ./...`.

### Follow-ups (if/when implemented)

- Implement future `sessions.json` and `ratelimit.json` using `WithLock + SaveJSON/LoadJSON` with 0600 perms.


