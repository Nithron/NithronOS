# Authentication Migration Guide

This guide covers the changes made to the authentication and setup flows in nOS.

## Summary of Changes

The authentication system has been completely refactored to:
- Remove all mock data and connect to real backend APIs
- Add proper error handling and user feedback
- Implement automatic token refresh
- Add comprehensive test coverage
- Improve UX with loading states and validation

## Breaking Changes

### 1. API Client Refactoring

**Old**: `web/src/lib/api.ts` with basic fetch wrapper

**New**: `web/src/lib/api-client.ts` with advanced features

```typescript
// Before
import api from '@/lib/api'
const data = await api('/endpoint')

// After
import { api } from '@/lib/api-client'
const data = await api.endpoint.method()
```

### 2. Toast System

**Old**: Various toast implementations

**New**: Unified toast system with `pushToast` export

```typescript
// Before
import { toast } from 'somewhere'
toast('Message')

// After
import { pushToast } from '@/components/ui/toast'
pushToast.success('Success!')
pushToast.error('Error occurred')
```

### 3. Authentication Context

**New**: Auth state managed via React Context

```typescript
// Add to components needing auth:
import { useAuth } from '@/lib/auth'

function Component() {
  const { session, login, logout } = useAuth()
  // ...
}
```

## File Changes

### New Files

1. **`web/src/lib/api-client.ts`**
   - Centralized API client with error handling
   - Automatic token refresh
   - CSRF protection

2. **`web/src/lib/auth.tsx`**
   - Authentication context provider
   - Session management hooks
   - Token storage utilities

3. **`web/src/__tests__/auth-flow.test.tsx`**
   - Login component unit tests
   - TOTP flow tests
   - Error handling tests

4. **`web/src/__tests__/setup-flow.test.tsx`**
   - Setup flow unit tests
   - OTP verification tests
   - Admin creation tests

5. **`web/src/__tests__/auth-e2e.test.tsx`**
   - End-to-end authentication tests
   - Session management tests
   - Rate limiting tests

### Modified Files

1. **`web/src/App.tsx`**
   - Added `AuthProvider` wrapper
   - Implemented `AuthGuard` for protected routes
   - Added `SetupGuard` for first-boot detection

2. **`web/src/pages/Login.tsx`**
   - Connected to real `/api/auth/login` endpoint
   - Added TOTP support
   - Improved error handling with specific messages

3. **`web/src/pages/Setup.tsx`**
   - Connected to `/api/setup/*` endpoints
   - Added OTP verification step
   - Added TOTP enrollment flow
   - Password strength indicator

4. **`web/src/components/layout/AppShell.tsx`**
   - Integrated logout functionality
   - Added user session display
   - Fixed TypeScript types

5. **`web/src/components/ui/toast.tsx`**
   - Added `pushToast` export
   - Added `Toasts` component export
   - Fixed unused parameter warnings

## Migration Steps

### 1. Update Imports

Replace all API imports:

```typescript
// Old
import api from '@/lib/api'

// New
import { api } from '@/lib/api-client'
```

Update toast imports:

```typescript
// Old
import { toast } from '@/components/ui/toast'

// New
import { pushToast } from '@/components/ui/toast'
```

### 2. Wrap App with Providers

Ensure your app root has all providers:

```typescript
<GlobalNoticeProvider>
  <ToastProvider />
  <AuthProvider>
    <RouterProvider router={router} />
  </AuthProvider>
</GlobalNoticeProvider>
```

### 3. Update API Calls

Convert API calls to use the new client:

```typescript
// Old
const response = await fetch('/api/auth/login', {
  method: 'POST',
  body: JSON.stringify(data)
})

// New
const response = await api.auth.login(data)
```

### 4. Add Error Handling

Use the new error types:

```typescript
import { APIError, ProxyMisconfiguredError } from '@/lib/api-client'

try {
  await api.someEndpoint()
} catch (err) {
  if (err instanceof ProxyMisconfiguredError) {
    // Backend is down
  } else if (err instanceof APIError) {
    if (err.status === 429) {
      // Rate limited
    }
  }
}
```

### 5. Protect Routes

Add auth guards to protected routes:

```typescript
<Route
  path="/dashboard"
  element={
    <AuthGuard requireAuth={true}>
      <Dashboard />
    </AuthGuard>
  }
/>
```

## Backend Requirements

Ensure your backend implements these endpoints:

### Setup Endpoints
- `GET /api/setup/state`
- `POST /api/setup/verify-otp`
- `POST /api/setup/create-admin`

### Auth Endpoints
- `POST /api/auth/login`
- `POST /api/auth/logout`
- `POST /api/auth/refresh`
- `GET /api/auth/session`
- `POST /api/auth/totp/enroll`
- `POST /api/auth/totp/verify`

### Error Responses

Return consistent error format:

```json
{
  "error": "error_code",
  "message": "Human readable message"
}
```

## Testing

### Run Tests

```bash
# All tests
npm test

# Specific test file
npm test auth-flow

# With coverage
npm run test:coverage

# Watch mode
npm test -- --watch
```

### Test New Features

1. **First Boot Flow**
   - Clear browser storage
   - Navigate to app
   - Should redirect to `/setup`
   - Complete setup process

2. **Login Flow**
   - Enter credentials
   - Test "Remember Me"
   - Test TOTP if enabled

3. **Session Management**
   - Login successfully
   - Wait for token expiry (or modify token)
   - Verify automatic refresh

4. **Error Handling**
   - Stop backend service
   - Try to login
   - Should show "Backend Unreachable" banner

## Rollback Plan

If you need to rollback:

1. Restore old files from git:
   ```bash
   git checkout main -- web/src/lib/api.ts
   git checkout main -- web/src/pages/Login.tsx
   git checkout main -- web/src/pages/Setup.tsx
   ```

2. Remove new files:
   ```bash
   rm web/src/lib/api-client.ts
   rm web/src/lib/auth.tsx
   rm -rf web/src/__tests__/auth-*.test.tsx
   ```

3. Revert App.tsx changes:
   ```bash
   git checkout main -- web/src/App.tsx
   ```

## Common Issues

### TypeScript Errors

If you see TypeScript errors after migration:

1. Clear TypeScript cache:
   ```bash
   rm -rf web/tsconfig.tsbuildinfo
   ```

2. Reinstall dependencies:
   ```bash
   cd web && npm ci
   ```

3. Restart TypeScript service in your IDE

### Test Failures

If tests fail after migration:

1. Clear test cache:
   ```bash
   npm test -- --clearCache
   ```

2. Update test mocks:
   ```typescript
   vi.mock('@/lib/api-client', () => ({
     api: { /* your mocks */ }
   }))
   ```

### Runtime Errors

Check browser console for:
- Missing providers (Auth, Toast, etc.)
- Incorrect API endpoints
- CORS issues

## Support

For issues or questions:

1. Check the [Authentication Documentation](./authentication.md)
2. Review test files for examples
3. Check backend logs for API errors
4. Open an issue with:
   - Error messages
   - Browser console output
   - Network request details
