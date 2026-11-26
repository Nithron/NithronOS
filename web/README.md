# nOS Web Frontend

React + TypeScript frontend for nOS.

## Development

```bash
npm install
npm run dev
```

## API Client Usage

All API calls must go through the centralized HTTP client at `web/src/lib/nos-client.ts`.

### Adding a New Endpoint

1. Add the endpoint to the appropriate group in `nos-client.ts`:

```typescript
// In the http object
apps: {
  // ... existing endpoints
  myNewEndpoint: (id: string) => httpCore.get(`/v1/apps/${id}/new-feature`),
}
```

2. Use it in your component:

```typescript
import http from '@/lib/nos-client';

const data = await http.apps.myNewEndpoint('app-123');
```

### Path Rules

- **Never** include `/api` in your paths - it's automatically prefixed
- Use `/v1/...` for versioned endpoints
- Use `/setup/...`, `/auth/...` etc. for unversioned endpoints
- The client will throw an error in development if you include `/api`

### IMPORTANT: No Placeholders Policy

**All data must come from real backend endpoints. No fake defaults!**

❌ **DON'T** return fake data on error:
```typescript
// BAD - hides backend issues
try {
  return await http.get('/v1/metrics')
} catch {
  return { cpuPct: 0, memory: 0 } // DON'T DO THIS!
}
```

✅ **DO** let errors bubble up:
```typescript
// GOOD - UI can show proper error state
return await http.get('/v1/metrics')
```

If a backend endpoint doesn't exist yet:
1. Show "Not available yet" in the UI
2. Add a TODO comment with expected endpoint
3. Never show fake zeros or placeholder data

### SSE and WebSocket

For real-time data, use the helper functions:

```typescript
import { openSSE, openWS } from '@/lib/nos-client';

// Server-Sent Events
const eventSource = openSSE('/v1/updates/stream');
eventSource.onmessage = (event) => {
  console.log(JSON.parse(event.data));
};

// WebSocket
const ws = openWS('/v1/apps/123/logs');
ws.onmessage = (event) => {
  console.log(event.data);
};
```

### Error Handling

The client automatically:
- Adds CSRF tokens from cookies
- Retries on 401 with refresh token
- Normalizes error responses
- Includes credentials (cookies) with all requests

### Migration from Direct fetch()

Before:
```typescript
const res = await fetch('/api/v1/apps', { credentials: 'include' });
const data = await res.json();
```

After:
```typescript
import http from '@/lib/nos-client';
const data = await http.get('/v1/apps');
```

## Testing

```bash
npm run test
npm run typecheck
```

## Building

```bash
npm run build
```
