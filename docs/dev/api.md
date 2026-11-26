# API (v1)

## Base and versioning
- All routes under `/api/v1`.
- Legacy routes may 301/410 temporarily.

## Error envelope
Responses use a standard shape:
```json
{ "error": { "code": "string", "message": "string", "retryAfterSec": 0 } }
```

## OpenAPI and the API contract
- `docs/api/openapi.json` lists every route nosd serves. It is generated from the router, never edited by hand:
```bash
cd backend/nosd && go run ./cmd/route-dump -format openapi -version "$(cat ../../VERSION)" > ../../docs/api/openapi.json
```
- A running nosd serves the same document at `GET /api/v1/openapi.json`.
- CI fails when `docs/api/openapi.json` is out of date, and when the web UI calls an API path that doesn't exist (`scripts/check-fe-api-paths.mjs`; known gaps are listed in `docs/api/fe-known-missing.json` until the UI is fixed).
- Request and response schemas are not described yet; `web/src/api/schema.ts` was generated from an older, hand-written spec and is no longer regenerated.
