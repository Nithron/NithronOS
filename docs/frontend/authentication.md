# Authentication & Setup Flow

## Overview

nOS uses a secure authentication system with support for first-boot setup, two-factor authentication (TOTP), and automatic session management. The frontend implements a complete authentication flow that integrates with the backend API.

## Architecture

### Key Components

1. **API Client** (`web/src/lib/api-client.ts`)
   - Centralized API communication
   - Automatic token refresh on 401
   - Error handling and user-friendly messages
   - CSRF token management
   - Proxy misconfiguration detection

2. **Auth Context** (`web/src/lib/auth.tsx`)
   - Session state management
   - Login/logout functionality
   - Token storage (sessionStorage/localStorage)
   - Automatic session refresh

3. **Route Guards** (`web/src/App.tsx`)
   - Protected routes requiring authentication
   - Setup state checking
   - Automatic redirects

## First-Boot Setup Flow

### 1. Setup State Detection

When the application loads, it checks `/api/setup/state` to determine if this is a first-boot scenario:

```typescript
{
  firstBoot: boolean,
  otpRequired: boolean
}
```

If `firstBoot` is true, users are redirected to `/setup`.

### 2. OTP Verification (if required)

Some deployments require a one-time password for initial setup:

- **Endpoint**: `POST /api/setup/verify-otp`
- **Purpose**: Verify deployment-specific OTP
- **Error Handling**:
  - 401: Invalid or expired OTP
  - 403: Permission denied (needs sudo/root)
  - 429: Rate limited

### 3. Admin Account Creation

After OTP verification (or if not required), create the admin account:

- **Endpoint**: `POST /api/setup/create-admin`
- **Validation**:
  - Username: 3-50 characters, alphanumeric
  - Password: 8+ characters, strength indicator
  - Password confirmation must match
- **Response**: May require TOTP enrollment

### 4. TOTP Enrollment (Optional)

If the backend requires 2FA:

- **Endpoint**: `POST /api/auth/totp/enroll`
- **Process**:
  1. Display QR code and secret key
  2. User scans with authenticator app
  3. Verify with 6-digit code
  4. Can be skipped (configure later in settings)

## Login Flow

### Standard Login

1. **Endpoint**: `POST /api/auth/login`
2. **Request**:
   ```json
   {
     "username": "string",
     "password": "string",
     "rememberMe": boolean,
     "totpCode": "string (optional)"
   }
   ```
3. **Response**:
   ```json
   {
     "accessToken": "jwt",
     "refreshToken": "jwt",
     "user": {
       "username": "string",
       "roles": ["admin"]
     }
   }
   ```

### Two-Factor Authentication

If user has TOTP enabled:

1. Initial login returns 403 with `requires_totp`
2. UI shows TOTP code field
3. Retry login with `totpCode` included

### Error Handling

- **401**: Invalid credentials
- **403**: TOTP required or account locked
- **429**: Rate limited (shows retry timer)
- **502/503**: Backend unreachable (shows help banner)

## Session Management

### Token Storage

- **Session Storage**: Default for temporary sessions
- **Local Storage**: When "Remember Me" is checked
- **Tokens**:
  - Access Token: Short-lived (15 minutes)
  - Refresh Token: Long-lived (7-30 days)

### Automatic Token Refresh

When an API call returns 401:

1. Attempt refresh via `POST /api/v1/auth/refresh`
2. If successful, retry original request
3. If refresh fails, redirect to login

```typescript
// Handled automatically by ApiClient
const response = await api.someEndpoint()
// If 401, refreshes token and retries transparently
```

### Session Checking

The `useSession` hook provides session state:

```typescript
const { session, loading, error } = useSession()

if (session) {
  // User is authenticated
  console.log(session.user.username)
}
```

## Route Protection

### Protected Routes

All main application routes require authentication:

```typescript
<Route element={<ProtectedLayout />}>
  <Route path="/" element={<Dashboard />} />
  <Route path="/storage" element={<Storage />} />
  // ... other protected routes
</Route>
```

### Auth Guard Component

The `AuthGuard` component:
- Checks authentication state
- Redirects to `/login` if not authenticated
- Shows loading state during verification
- Handles setup state (forces `/setup` if needed)

## Security Features

### CSRF Protection

- Token fetched from `/api/csrf-token`
- Included in all mutating requests
- Automatically managed by API client

### Rate Limiting

- Login attempts: 5 per minute
- OTP verification: 3 per minute
- Shows countdown timer when limited
- Exponential backoff on repeated failures

### Password Security

- Minimum 8 characters
- Real-time strength indicator
- Bcrypt hashing on backend
- No password hints or recovery (admin reset only)

## Error States

### Global Error Banner

For critical errors (backend down), a global banner appears:

```typescript
// Triggered by ProxyMisconfiguredError
{
  kind: 'error',
  title: 'Backend Unreachable',
  message: 'Cannot connect to the nOS backend',
  action: { label: 'View Help', href: '/help/proxy' }
}
```

### Form Validation

- Real-time validation as user types
- Clear error messages
- Field-level error states
- Disabled submit until valid

## Testing

### Unit Tests

Located in `web/src/__tests__/`:
- `auth-flow.test.tsx`: Login component tests
- `setup-flow.test.tsx`: Setup component tests

### E2E Tests

- `auth-e2e.test.tsx`: Full authentication flow
- Tests setup → login → session → logout

### Running Tests

```bash
cd web
npm test                    # Run all tests
npm test auth-flow         # Run specific test
npm run test:coverage      # With coverage
```

## API Endpoints Reference

### Setup Endpoints

- `GET /api/setup/state` - Check setup status
- `POST /api/setup/verify-otp` - Verify setup OTP
- `POST /api/setup/create-admin` - Create admin account

### Auth Endpoints

- `POST /api/auth/login` - User login
- `POST /api/auth/logout` - User logout
- `POST /api/v1/auth/refresh` - Refresh access token
- `GET /api/auth/session` - Get current session
- `POST /api/auth/totp/enroll` - Start TOTP enrollment
- `POST /api/auth/totp/verify` - Verify TOTP code

## Troubleshooting

### Common Issues

1. **"Backend Unreachable" Error**
   - Check if backend is running (`systemctl status nosd`)
   - Verify proxy configuration
   - Check `/help/proxy` for detailed help

2. **"Session Expired" on Page Refresh**
   - Normal if not using "Remember Me"
   - Check browser storage settings
   - Verify refresh token validity

3. **Rate Limiting**
   - Wait for countdown to complete
   - Check for automation/scripts
   - Consider increasing limits in production

4. **TOTP Issues**
   - Verify device time is synchronized
   - Check authenticator app settings
   - Use backup codes if available

## Development

### Environment Variables

```env
VITE_API_URL=http://localhost:9000  # Backend API URL
```

### Mock Authentication

For development without backend:

```typescript
// In api-client.ts, add mock mode
if (import.meta.env.DEV && import.meta.env.VITE_MOCK_API) {
  return mockResponse()
}
```

### Debug Logging

Enable auth debug logs:

```typescript
localStorage.setItem('debug:auth', 'true')
```
