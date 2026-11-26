# Real Data Only (M1-M3)

This document describes how the nOS web UI is wired to use real API data for all M1-M3 features.

## Overview

The frontend has been fully migrated to use real API endpoints for all implemented features. No mock or placeholder data is used for M1-M3 functionality. Features beyond M3 (M4+) remain gated with "Coming soon" messaging.

## API Client Architecture

### Core Client (`/web/src/lib/api.ts`)

The centralized API client provides:
- Automatic CSRF token handling
- Session management with token refresh
- Strict JSON parsing with Zod schemas
- Typed error handling with status codes
- Backend unreachable detection (502 errors)

### React Query Hooks (`/web/src/hooks/use-api.ts`)

All data fetching uses React Query (TanStack Query) for:
- Automatic caching and background refetching
- Optimistic updates
- Error retry with exponential backoff
- Loading and error states

## Wired Pages and Their API Calls

### Dashboard (`/web/src/pages/Dashboard.tsx`)

| Data | API Endpoint | Refresh Interval |
|------|--------------|------------------|
| System Info | `GET /api/v1/system/info` | 10s (stale) |
| Pool Summary | `GET /api/v1/storage/pools?summary=1` | 10s |
| SMART Summary | `GET /api/v1/health/smart/summary` | 10s |
| Scrub Status | `GET /api/v1/btrfs/scrub/status` | 5s (when running) |
| Balance Status | `GET /api/v1/btrfs/balance/status` | 5s (when running) |
| Recent Jobs | `GET /api/v1/jobs/recent?limit=10` | 5s |
| Shares List | `GET /api/v1/shares` | 10s |
| Installed Apps | `GET /api/v1/apps/installed` | 10s |

### Storage (`/web/src/pages/Storage.tsx`)

| Data | API Endpoint | Actions |
|------|--------------|---------|
| Pools List | `GET /api/v1/storage/pools` | View details |
| Devices List | `GET /api/v1/storage/devices` | View SMART |
| SMART Details | `GET /api/v1/health/smart/{device}` | - |
| Scrub Status | `GET /api/v1/btrfs/scrub/status` | Start/Cancel |
| Balance Status | `GET /api/v1/btrfs/balance/status` | Start/Cancel |
| Mount Options | `GET /api/v1/storage/pools/{uuid}/options` | Update |

### Shares (`/web/src/pages/Shares.tsx`)

| Data | API Endpoint | Actions |
|------|--------------|---------|
| Shares List | `GET /api/v1/shares` | CRUD operations |
| Share Details | `GET /api/v1/shares/{name}` | View/Edit |
| Create Share | `POST /api/v1/shares` | Create new |
| Update Share | `PUT /api/v1/shares/{name}` | Modify |
| Delete Share | `DELETE /api/v1/shares/{name}` | Remove |
| Test Share | `POST /api/v1/shares/{name}/test` | Validate |

### Apps (`/web/src/pages/AppCatalog.tsx`)

| Data | API Endpoint | Actions |
|------|--------------|---------|
| App Catalog | `GET /api/v1/apps/catalog` | Browse/Install |
| Installed Apps | `GET /api/v1/apps/installed` | Manage |
| App Details | `GET /api/v1/apps/{id}` | View status |
| Install App | `POST /api/v1/apps/install` | Deploy |
| Start/Stop/Restart | `POST /api/v1/apps/{id}/{action}` | Lifecycle |
| App Logs | `GET /api/v1/apps/{id}/logs` | Stream |
| Health Check | `POST /api/v1/apps/{id}/health` | Force check |

### Schedules (`/web/src/routes/settings/schedules.tsx`)

| Data | API Endpoint | Actions |
|------|--------------|---------|
| Schedules List | `GET /api/v1/schedules` | View/Edit |
| Create Schedule | `POST /api/v1/schedules` | Add new |
| Update Schedule | `PUT /api/v1/schedules/{id}` | Modify |
| Delete Schedule | `DELETE /api/v1/schedules/{id}` | Remove |

## Backend Endpoints

All backend handlers are located in `/backend/nosd/internal/server/`:

- `system_handler.go` - System info and services
- `health_handler.go` - SMART health monitoring
- `storage_handler.go` - Pools and devices
- `schedules_handler.go` - Cron schedules
- `shares_handler.go` - Network shares (v1 API)
- `jobs_handler.go` - Background jobs
- `apps_handler.go` - App catalog (from M3)

## Error Handling

### Backend Unreachable

When the backend is unreachable (502 error or HTML response), the UI displays:
```jsx
<Alert className="border-destructive">
  <AlertCircle className="h-4 w-4" />
  <AlertDescription>
    Backend unreachable or proxy misconfigured. 
    Please check that the backend service is running.
  </AlertDescription>
</Alert>
```

### API Errors

All API errors are handled with:
- Typed error messages from backend
- Toast notifications for user actions
- Per-tile error states (non-blocking)
- Automatic retry with backoff

### Loading States

Each data section shows loading states:
- Skeleton loaders for initial load
- Refresh indicators for updates
- Partial data display (non-blocking)

## Testing

### Unit Tests

API wiring is tested in `/web/src/__tests__/api-wiring.test.ts`:
- Verifies correct endpoints are called
- Tests error handling
- Validates response parsing

### Integration Tests

Run the backend and verify:
```bash
# Start backend
cd backend/nosd
go run .

# In another terminal, test endpoints
curl http://localhost:8090/api/v1/system/info
curl http://localhost:8090/api/v1/storage/pools
curl http://localhost:8090/api/v1/shares
```

### Manual Testing

1. Turn off backend: UI shows "Backend unreachable" banner
2. Turn on backend: Data loads automatically
3. Create/modify resources: Changes reflect immediately
4. Check network tab: Only `/api/v1/*` calls, no mock data

## Excluded Features (M4+)

The following remain with placeholder/mock data as they are M4+ features:

- **Remote** page - Backup destinations and jobs
- **User Management** - User/role administration
- **Firewall Planning** - Network security rules
- **HTTPS/LE Automation** - Certificate management
- **WireGuard/Tunnels** - Remote access

These features show "Coming soon" messaging and do not make API calls.

## Development Guidelines

### Adding New API Endpoints

1. Define types in `/web/src/lib/api.ts`:
```typescript
export const MyDataSchema = z.object({
  id: z.string(),
  // ... fields
})
export type MyData = z.infer<typeof MyDataSchema>
```

2. Add endpoint to `endpoints` object:
```typescript
myFeature: {
  list: () => api.get<MyData[]>('/v1/myfeature'),
  get: (id: string) => api.get<MyData>(`/v1/myfeature/${id}`),
}
```

3. Create React Query hook:
```typescript
export function useMyFeature() {
  return useQuery({
    queryKey: ['myfeature'],
    queryFn: endpoints.myFeature.list,
    staleTime: 10_000,
  })
}
```

4. Use in component:
```typescript
const { data, isLoading, error } = useMyFeature()
```

### Removing Mock Data

Search for and remove:
- `mock` - Mock data variables
- `fixture` - Test fixtures
- `faker` - Fake data generation
- `TODO demo` - Demo placeholders
- Hard-coded arrays/objects in components

Replace with:
- API hooks from `/web/src/hooks/use-api.ts`
- Loading states while fetching
- Error boundaries for failures
- Empty states when no data

## Performance Considerations

- **Stale Time**: Most queries use 10-30s stale time
- **Refetch Interval**: Running operations poll every 5s
- **Query Invalidation**: Mutations invalidate related queries
- **Parallel Queries**: Dashboard fetches all data in parallel
- **Partial Updates**: Tiles load independently (non-blocking)
