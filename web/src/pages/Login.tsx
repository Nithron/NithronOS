import { useState, useEffect } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import BrandHeader from '@/components/BrandHeader'
import { api, APIError, ProxyMisconfiguredError, getErrorMessage } from '@/lib/nos-client'
import { toast } from '@/components/ui/toast'
import { useGlobalNotice } from '@/lib/globalNotice'
import { useAuth } from '@/lib/auth'

// ============================================================================
// Form Schema
// ============================================================================

const LoginSchema = z.object({
  username: z.string().min(1, 'Username is required'),
  password: z.string().min(1, 'Password is required'),
  rememberMe: z.boolean().optional(),
})

type LoginInput = z.infer<typeof LoginSchema>

// ============================================================================
// Login Component
// ============================================================================

export function Login() {
  const navigate = useNavigate()
  const location = useLocation()
  const { notice } = useGlobalNotice()
  const { checkSession } = useAuth()
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [requiresCode, setRequiresCode] = useState(false)
  const [totpCode, setTotpCode] = useState('')
  const [savedCredentials, setSavedCredentials] = useState<LoginInput | null>(null)
  
  // Check if backend is reachable
  const isBackendUnreachable = notice?.title.includes('Backend unreachable')
  
  // Get return URL from location state
  const returnTo = (location.state as any)?.returnTo || '/'
  
  const {
    register,
    handleSubmit,
    formState: { errors },
    setFocus,
  } = useForm<LoginInput>({
    resolver: zodResolver(LoginSchema),
    defaultValues: {
      username: '',
      password: '',
      rememberMe: false,
    },
  })
  
  // Focus username field on mount
  useEffect(() => {
    if (!isBackendUnreachable) {
      setFocus('username')
    }
  }, [setFocus, isBackendUnreachable])
  
  // Check if we should redirect to setup
  useEffect(() => {
    checkSetupState()
  }, [])
  
  const checkSetupState = async () => {
    try {
      const state = await api.setup.getState()
      if (state.firstBoot) {
        navigate('/setup', { replace: true })
      }
    } catch (err) {
      // If setup returns 410, it's complete - stay on login
      if (err instanceof APIError && err.status === 410) {
        return
      }
      // For other errors, just continue to login
      console.error('Setup state check failed:', err)
    }
  }
  
  const onSubmit = async (data: LoginInput) => {
    try {
      setLoading(true)
      setError(null)
      
      // Save credentials in case we need them for TOTP
      setSavedCredentials(data)
      
      // Include TOTP code if required
      const loginData = {
        ...data,
        code: requiresCode ? totpCode.replace(/\s+/g, '') : undefined,
      }
      
      await api.auth.login(loginData)
      
      // Success - update session and navigate
      toast.success('Successfully signed in')
      
      // Check session to update auth context
      await checkSession()
      
      // Navigate to return URL or dashboard
      navigate(returnTo, { replace: true })
    } catch (err) {
      if (err instanceof APIError) {
        // Check for TOTP requirement
        if (err.status === 401 && 
            (err.message?.toLowerCase().includes('code required') || 
             err.message?.toLowerCase().includes('totp'))) {
          setRequiresCode(true)
          setError('Two-factor authentication required')
          setTotpCode('')
          // Focus TOTP input after render
          setTimeout(() => {
            const totpInput = document.getElementById('totpCode')
            if (totpInput) totpInput.focus()
          }, 100)
          return
        }
        
        // Handle rate limiting
        if (err.status === 429) {
          const retryMsg = err.retryAfterSec 
            ? `Too many attempts. Try again in ${err.retryAfterSec}s`
            : 'Too many attempts. Please try again later.'
          setError(retryMsg)
          toast.error(retryMsg)
          return
        }
        
        // Handle account locked
        if (err.status === 423) {
          setError('Account temporarily locked. Please try again later.')
          toast.error('Account temporarily locked')
          return
        }
        
        // Handle setup incomplete - redirect to setup
        if (err.status === 403 && err.code === 'setup.incomplete') {
          setError('Setup is not complete. Redirecting to setup...')
          toast.info('Please complete the setup process first')
          setTimeout(() => {
            navigate('/setup', { replace: true })
          }, 1500)
          return
        }
        
        // Handle invalid credentials
        if (err.status === 401) {
          setError('Invalid username or password')
          // Keep username, clear password
          setFocus('password')
          return
        }
        
        // Other API errors
        setError(getErrorMessage(err))
      } else if (err instanceof ProxyMisconfiguredError) {
        setError('Backend unreachable. Check your proxy configuration.')
      } else {
        setError(getErrorMessage(err))
      }
    } finally {
      setLoading(false)
    }
  }
  
  const handleTotpSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    
    if (!savedCredentials) {
      setError('Session expired. Please enter credentials again.')
      setRequiresCode(false)
      return
    }
    
    const cleanCode = totpCode.replace(/\s+/g, '')
    if (cleanCode.length < 6) {
      setError('Please enter a valid 6-digit code or recovery code')
      return
    }
    
    // Re-submit with TOTP code
    await onSubmit(savedCredentials)
  }
  
  // Don't render login content if backend is unreachable
  if (isBackendUnreachable) {
    return (
      <div className="min-h-screen w-full flex items-center justify-center">
        <div className="text-center">
          <h1 className="screen-title mb-4">Backend unreachable</h1>
          <p className="text-muted-foreground">
            The server is not responding. Please check your proxy configuration.
          </p>
        </div>
      </div>
    )
  }
  
  return (
    <div className="min-h-screen w-full flex items-center justify-center">
      <div className="w-full max-w-sm p-6">
        <BrandHeader />
        <h1 className="screen-title mb-6">Sign in</h1>
        
        {!requiresCode ? (
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            <div>
              <label htmlFor="username" className="block text-sm font-medium mb-2">
                Username
              </label>
              <input
                id="username"
                type="text"
                className="field"
                placeholder="Enter your username"
                autoComplete="username"
                disabled={loading}
                {...register('username')}
              />
              {errors.username && (
                <p className="text-xs text-status-failed-text mt-1">{errors.username.message}</p>
              )}
            </div>
            
            <div>
              <label htmlFor="password" className="block text-sm font-medium mb-2">
                Password
              </label>
              <input
                id="password"
                type="password"
                className="field"
                placeholder="Enter your password"
                autoComplete="current-password"
                disabled={loading}
                {...register('password')}
              />
              {errors.password && (
                <p className="text-xs text-status-failed-text mt-1">{errors.password.message}</p>
              )}
            </div>
            
            <div className="flex items-center">
              <input
                id="rememberMe"
                type="checkbox"
                className="mr-2"
                disabled={loading}
                {...register('rememberMe')}
              />
              <label htmlFor="rememberMe" className="text-sm">
                Remember me
              </label>
            </div>
            
            {error && !requiresCode && (
              <div className="text-sm text-status-failed-text" role="alert" aria-live="polite">
                {error}
              </div>
            )}
            
            <button
              type="submit"
              className="btn btn-primary w-full py-3"
              disabled={loading}
            >
              {loading ? 'Signing in…' : 'Sign in'}
            </button>
          </form>
        ) : (
          <form onSubmit={handleTotpSubmit} className="space-y-4">
            <div className="mb-4">
              <p className="text-sm text-muted-foreground">
                Signing in as <strong>{savedCredentials?.username}</strong>
              </p>
            </div>
            
            <div>
              <label htmlFor="totpCode" className="block text-sm font-medium mb-2">
                Two-factor authentication code
              </label>
              <input
                id="totpCode"
                type="text"
                className="field text-center tracking-widest font-mono"
                placeholder="123 456"
                value={totpCode}
                onChange={(e) => setTotpCode(e.target.value)}
                autoFocus
                autoComplete="one-time-code"
                disabled={loading}
              />
              <p className="text-xs text-muted-foreground mt-1">
                Enter the 6-digit code from your authenticator app or a recovery code
              </p>
            </div>
            
            {error && (
              <div className="text-sm text-status-failed-text" role="alert" aria-live="polite">
                {error}
              </div>
            )}
            
            <button
              type="submit"
              className="btn btn-primary w-full py-3"
              disabled={loading}
            >
              {loading ? 'Verifying…' : 'Verify and sign in'}
            </button>
            
            <button
              type="button"
              className="text-sm text-muted-foreground underline w-full text-center"
              onClick={() => {
                setRequiresCode(false)
                setError(null)
                setTotpCode('')
                setSavedCredentials(null)
              }}
            >
              Use different account
            </button>
          </form>
        )}
      </div>
    </div>
  )
}