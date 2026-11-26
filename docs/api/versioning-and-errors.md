## API versioning, typed errors, and OpenAPI types

### Base path and shims
- All application routes are mounted under `/api/v1`.
- Temporary shims can return 301 (redirect) or 410 (gone) for old paths if needed during migrations.

### Standard error envelope
Back-end errors follow a single shape:

```json
{ "error": { "code": "string", "message": "string", "retryAfterSec": 0 } }
```

- Use helpers in `backend/nosd/pkg/httpx`:
  - `WriteError(w, status, message)`
  - `WriteTypedError(w, status, code, message, retryAfterSec)`
- 429 responses also set `Retry-After` header (seconds).

### OpenAPI spec
- `docs/api/openapi.json` is generated from the nosd router (`go run ./cmd/route-dump -format openapi -version "$(cat ../../VERSION)"` in `backend/nosd`); CI checks it is current. See `docs/dev/api.md`.


