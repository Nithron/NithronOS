import { createContext, useContext, useEffect, useState, useCallback, ReactNode } from 'react'
import { Working } from '@/components/ui/pixel'
import { useNavigate, useLocation } from 'react-router-dom'
import { api, AuthSession, APIError, ProxyMisconfiguredError } from './nos-client'
import { toast } from '@/components/ui/toast'

// ============================================================================
// Auth Context
// ============================================================================

interface AuthContextType {
  session: AuthSession | null
  loading: boolean
  error: string | null
  login: (username: string, password: string, code?: string, rememberMe?: boolean) => Promise<void>
  logout: () => Promise<void>
  checkSession: () => Promise<void>
}

const AuthContext = createContext<AuthContextType | null>(null)

// ============================================================================
// Auth Provider
// ============================================================================

interface AuthProviderProps {
  children: ReactNode
}

export function AuthProvider({ children }: AuthProviderProps) {
  const [session, setSession] = useState<AuthSession | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()
  const location = useLocation()

  // Check current session
  const checkSession = useCallback(async () => {
    try {
      setLoading(true)
      setError(null)
      
      // First check if we're in first-boot mode
      try {
        const setupState = await api.setup.getState()
        if (setupState.firstBoot) {
          // Force to setup if first boot
          if (location.pathname !== '/setup') {
            navigate('/setup', { replace: true })
          }
          setSession(null)
          return
        }
      } catch (err) {
        // If setup endpoint returns 410, setup is complete
        if (err instanceof APIError && err.status === 410) {
          // Continue to auth check
        } else if (err instanceof ProxyMisconfiguredError) {
          // Let global notice handle this
          setError('Backend unreachable')
          return
        } else {
          // Other errors, assume setup might be needed
          console.error('Setup state check failed:', err)
        }
      }
      
      // Check auth session
      const authSession = await api.auth.session() as AuthSession
      setSession(authSession)
      
      // If we have a session but we're on login/setup, redirect to dashboard
      if (authSession?.user && (location.pathname === '/login' || location.pathname === '/setup')) {
        navigate('/', { replace: true })
      }
    } catch (err) {
      if (err instanceof APIError && err.status === 401) {
        // Not authenticated
        setSession(null)
        if (location.pathname !== '/login' && location.pathname !== '/setup' && !location.pathname.startsWith('/help')) {
          navigate('/login', { replace: true })
        }
      } else if (err instanceof ProxyMisconfiguredError) {
        setError('Backend unreachable')
      } else {
        console.error('Session check failed:', err)
        setError('Failed to check authentication')
      }
    } finally {
      setLoading(false)
    }
  }, [navigate, location.pathname])

  // Login
  const login = useCallback(async (
    username: string,
    password: string,
    code?: string,
    rememberMe?: boolean
  ) => {
    try {
      setLoading(true)
      setError(null)
      
      await api.auth.login({
        username,
        password,
        code: code?.replace(/\s+/g, ''),
        rememberMe,
      })
      
      // Fetch session after successful login
      const authSession = await api.auth.session() as AuthSession
      setSession(authSession)
      
      // Navigate to dashboard
      navigate('/', { replace: true })
      toast.success('Successfully signed in')
    } catch (err) {
      if (err instanceof APIError) {
        if (err.status === 401) {
          // Check if it's a TOTP requirement
          if (err.message?.toLowerCase().includes('code required') || 
              err.message?.toLowerCase().includes('totp')) {
            throw new Error('code_required')
          }
          setError('Invalid username or password')
        } else if (err.status === 423) {
          setError('Account temporarily locked')
        } else if (err.status === 429) {
          const retryMsg = err.retryAfterSec 
            ? `Too many attempts. Try again in ${err.retryAfterSec}s`
            : 'Too many attempts. Please try again later.'
          setError(retryMsg)
        } else {
          setError(err.message || 'Login failed')
        }
      } else {
        setError('Login failed')
      }
      throw err
    } finally {
      setLoading(false)
    }
  }, [navigate])

  // Logout
  const logout = useCallback(async () => {
    try {
      setLoading(true)
      await api.auth.logout()
      setSession(null)
      navigate('/login', { replace: true })
      toast.success('Successfully signed out')
    } catch (err) {
      console.error('Logout failed:', err)
      // Even if logout fails, clear local session and redirect
      setSession(null)
      navigate('/login', { replace: true })
    } finally {
      setLoading(false)
    }
  }, [navigate])

  // Check session on mount and when location changes
  useEffect(() => {
    checkSession()
  }, []) // Only on mount, not on every location change to avoid loops

  // Listen for storage events (logout from another tab)
  useEffect(() => {
    const handleStorageChange = (e: StorageEvent) => {
      if (e.key === 'nos_logout') {
        setSession(null)
        navigate('/login', { replace: true })
      }
    }
    window.addEventListener('storage', handleStorageChange)
    return () => window.removeEventListener('storage', handleStorageChange)
  }, [navigate])

  const value = {
    session,
    loading,
    error,
    login,
    logout,
    checkSession,
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

// ============================================================================
// useAuth Hook
// ============================================================================

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}

// ============================================================================
// Auth Guard Component
// ============================================================================

interface AuthGuardProps {
  children: ReactNode
  requireAuth?: boolean
  requireAdmin?: boolean
}

export function AuthGuard({ 
  children, 
  requireAuth = true,
  requireAdmin = false 
}: AuthGuardProps) {
  const { session, loading } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()

  useEffect(() => {
    if (loading) return

    if (requireAuth && !session?.user) {
      // Save intended destination
      const returnTo = location.pathname + location.search
      navigate('/login', { 
        replace: true,
        state: { returnTo }
      })
    }

    if (requireAdmin && session?.user) {
      const isAdmin = session.user.roles?.includes('admin')
      if (!isAdmin) {
        toast.error('Admin access required')
        navigate('/', { replace: true })
      }
    }
  }, [session, loading, requireAuth, requireAdmin, navigate, location])

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <Working />
      </div>
    )
  }

  if (requireAuth && !session?.user) {
    return null
  }

  return <>{children}</>
}
