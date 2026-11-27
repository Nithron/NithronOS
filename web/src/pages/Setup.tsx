import { useEffect, useState } from 'react'
import { Meter, StatusMark, Working } from '@/components/ui/pixel'
import { useNavigate } from 'react-router-dom'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import QRCode from 'qrcode'
import BrandHeader from '@/components/BrandHeader'
import { useGlobalNotice } from '@/lib/globalNotice'
import { api, APIError, ProxyMisconfiguredError, getErrorMessage } from '@/lib/nos-client'
import { toast } from '@/components/ui/toast'
import { TIMEZONE_REGIONS, getTimezoneInfo } from '@/lib/timezone-data'

// ============================================================================
// Type Definitions
// ============================================================================

type SetupStep = 'welcome' | 'otp' | 'admin' | 'system' | 'network' | 'telemetry' | 'totp' | 'done'

// ============================================================================
// Password Validation
// ============================================================================

const passwordStrength = (password: string): number => {
  let score = 0
  if (password.length >= 12) score++
  if (/[a-z]/.test(password)) score++
  if (/[A-Z]/.test(password)) score++
  if (/[0-9]/.test(password)) score++
  if (/[^A-Za-z0-9]/.test(password)) score++
  return Math.min(score, 4)
}

const CreateAdminSchema = z.object({
  username: z.string()
    .min(3, 'Username must be at least 3 characters')
    .max(32, 'Username must be at most 32 characters')
    .regex(/^[a-z0-9_-]+$/, 'Username can only contain lowercase letters, numbers, dash and underscore'),
  password: z.string()
    .min(12, 'Password must be at least 12 characters')
    .refine(p => passwordStrength(p) >= 3, {
      message: 'Password must include uppercase, lowercase, numbers, and symbols'
    }),
  confirmPassword: z.string(),
  enableTotp: z.boolean().optional(),
}).refine(data => data.password === data.confirmPassword, {
  message: 'Passwords do not match',
  path: ['confirmPassword'],
})

type CreateAdminInput = z.infer<typeof CreateAdminSchema>

// ============================================================================
// Main Setup Component
// ============================================================================

export default function Setup() {
  const navigate = useNavigate()
  const { notice } = useGlobalNotice()
  const [step, setStep] = useState<SetupStep>('welcome')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [setupToken, setSetupToken] = useState<string | null>(null)
  // Why the wizard went back to the code step, if it did.
  const [otpNotice, setOtpNotice] = useState<string | null>(null)
  const [adminCreds, setAdminCreds] = useState<{ username: string; password: string } | null>(null)
  const [enableTotp, setEnableTotp] = useState(false)
  
  // Check if backend is reachable
  const isBackendUnreachable = notice?.title.includes('Backend unreachable')

  // Check setup state on mount
  useEffect(() => {
    checkSetupState()
  }, [])

  const checkSetupState = async () => {
    try {
      setLoading(true)
      setError(null)
      
      const state = await api.setup.getState()

      if (!state.firstBoot) {
        // Setup already complete: leave any session alone (this page used
        // to sign out whoever opened it).
        setError('Setup already completed')
        toast.info('Setup already completed.')
        setTimeout(() => navigate('/'), 2000)
        return
      }
      // Clear any stale auth cookies from an earlier installation.
      try {
        await api.auth.logout()
      } catch {
        // Ignore logout errors
      }
      if (!state.otpRequired) {
        // nosd keeps a valid code while setup is pending, so this means it
        // could not create one (for example an unwritable state directory).
        setError('The server could not create a setup code. Check the nosd service log on the server.')
      }
    } catch (err) {
      if (err instanceof APIError && err.status === 410) {
        // Setup complete (410 Gone)
        setError('Setup already completed')
        toast.info('Setup already completed.')
        setTimeout(() => navigate('/'), 2000)
      } else if (err instanceof ProxyMisconfiguredError) {
        setError('Backend unreachable. Check your proxy configuration.')
      } else {
        setError(getErrorMessage(err))
      }
    } finally {
      setLoading(false)
    }
  }

  // Don't render setup content if backend is unreachable
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

  if (loading) {
    return (
      <div className="min-h-screen w-full flex items-center justify-center">
        <div className="text-center">
          <Working>Loading setup</Working>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen w-full flex items-center justify-center overflow-y-auto">
      <div className="relative w-full max-w-md mx-auto p-6 py-12">
        <div className="mb-8">
          <BrandHeader />
        </div>
        <h1 className="screen-title mb-6">First-time setup</h1>
        
        {error && error.includes('Setup already completed') ? (
          <div className="text-center">
            <div className="mb-4 text-status-warning-text">{error}</div>
            <button 
              className="btn btn-primary"
              onClick={() => navigate('/login')}
            >
              Go to sign in
            </button>
          </div>
        ) : (
          <>
            <SetupSteps currentStep={step} />
            
            {step === 'welcome' && (
              <StepWelcome 
                onContinue={() => setStep('otp')}
              />
            )}
            
            {step === 'otp' && (
              <>
                {otpNotice && (
                  <div role="status" className="mb-4 rounded-md border border-status-warning/40 bg-status-warning-tint p-3 text-sm">
                    {otpNotice}
                  </div>
                )}
                <StepOTP 
                  onSuccess={(token) => {
                    setOtpNotice(null)
                    setSetupToken(token)
                    setStep('admin')
                  }}
                />
              </>
            )}
            
            {step === 'admin' && setupToken && (
              <StepCreateAdmin
                token={setupToken}
                onExpired={() => {
                  setSetupToken(null)
                  setOtpNotice('The setup session ended after 10 minutes. Enter the code shown on the server screen again; nothing was lost.')
                  setStep('otp')
                }}
                onSuccess={(totpEnabled, creds) => {
                  if (creds) {
                    setAdminCreds(creds)
                    setEnableTotp(totpEnabled)
                  }
                  setStep('system')
                }}
              />
            )}
            
            {step === 'system' && setupToken && (
              <StepSystemConfig
                onSuccess={() => setStep('network')}
              />
            )}
            
            {step === 'network' && setupToken && (
              <StepNetworkConfig
                onSuccess={() => setStep('telemetry')}
              />
            )}
            
            {step === 'telemetry' && setupToken && (
              <StepTelemetry
                onSuccess={async () => {
                  if (enableTotp && adminCreds) {
                    setStep('totp')
                  } else {
                    // Mark setup as complete if no TOTP. The administrator is
                    // signed in, which also works after the token expired.
                    try {
                      await api.setup.complete(setupToken)
                    } catch (err) {
                      toast.error(`Setup could not be finished: ${getErrorMessage(err)}`)
                      return
                    }
                    // Attempt to login so subsequent steps use nos-client session
                    try {
                      if (adminCreds) {
                        await api.auth.login({ username: adminCreds.username, password: adminCreds.password })
                      }
                    } catch {}
                    setStep('done')
                  }
                }}
              />
            )}
            
            {step === 'totp' && adminCreds && setupToken && (
              <StepTOTPEnroll
                credentials={adminCreds}
                onSuccess={async () => {
                  // Login after TOTP enrollment (session needed for subsequent API calls)
                  try {
                    await api.auth.login({
                      username: adminCreds.username,
                      password: adminCreds.password,
                    })
                  } catch (loginErr) {
                    console.warn('Auto-login after TOTP enrollment failed:', loginErr)
                  }
                  
                  // Mark setup as complete after TOTP
                  try {
                    await api.setup.complete(setupToken)
                  } catch (err) {
                    toast.error(`Setup could not be finished: ${getErrorMessage(err)}`)
                    return
                  }
                  setStep('done')
                }}
              />
            )}
            
            {step === 'done' && (
              <StepDone onContinue={() => navigate('/login')} />
            )}
          </>
        )}
      </div>
    </div>
  )
}

// ============================================================================
// Setup Steps Indicator
// ============================================================================

function SetupSteps({ currentStep }: { currentStep: SetupStep }) {
  const steps = [
    { id: 'welcome', label: 'Welcome' },
    { id: 'otp', label: 'OTP' },
    { id: 'admin', label: 'Admin' },
    { id: 'system', label: 'System' },
    { id: 'network', label: 'Network' },
    { id: 'telemetry', label: 'Telemetry' },
    { id: 'totp', label: '2FA' },
    { id: 'done', label: 'Done' },
  ]
  
  const currentIndex = steps.findIndex(s => s.id === currentStep)
  
  return (
    <ol className="mb-8 flex flex-wrap gap-x-5 gap-y-2 border-b border-hairline pb-3 text-sm">
      {steps.map((step, index) => (
        <li key={step.id} aria-current={index === currentIndex ? 'step' : undefined}>
          <StatusMark
            state={index < currentIndex ? 'ok' : index === currentIndex ? 'running' : 'off'}
            className={index === currentIndex ? 'font-[560] text-ink' : 'text-ink-2'}
          >
            {step.label}
          </StatusMark>
        </li>
      ))}
    </ol>
  )
}

// ============================================================================
// Step 0: Welcome Screen with OTP Instructions
// ============================================================================

function StepWelcome({ onContinue }: { onContinue: () => void }) {
  return (
    <div className="space-y-6">
      <div className="text-center space-y-3">
        <h2 className="text-2xl font-[560]">Welcome to nOS!</h2>
        <p className="text-muted-foreground">
          Let's get your system set up and ready to use.
        </p>
      </div>
      
      <div className="border border-muted rounded p-4 space-y-3">
        <h3 className="font-medium">Find your one-time setup code</h3>
        <p className="text-sm text-muted-foreground">
          The 6-digit code is shown on the server's own screen (monitor or serial console),
          above the login prompt, together with the address to open.
        </p>
        <div className="border border-hairline p-3">
          <p className="text-xs text-muted-foreground mb-1">No screen attached? Over SSH:</p>
          <code className="block bg-background rounded p-2 text-xs font-mono">
            sudo journalctl -u nosd | grep 'setup code' | tail -n 1
          </code>
        </div>
        <div className="text-xs text-muted-foreground">
          Each code is valid for 15 minutes. When it expires, or after 5 wrong tries, a new
          code replaces it on the server screen automatically.
        </div>
      </div>

      <div className="flex gap-2 text-xs">
        <a href="https://docs.nithron.com" target="_blank" rel="noopener noreferrer" 
           className="text-primary hover:underline flex-1 text-center">
          📚 Documentation
        </a>
        <a href="https://github.com/Nithron/nOS/discussions" target="_blank" rel="noopener noreferrer" 
           className="text-primary hover:underline flex-1 text-center">
          💬 Community
        </a>
        <a href="https://github.com/Nithron/nOS/issues" target="_blank" rel="noopener noreferrer" 
           className="text-primary hover:underline flex-1 text-center">
          🐛 Issues
        </a>
      </div>
      
      <button
        className="btn btn-primary w-full py-3"
        onClick={onContinue}
      >
        I have the code - Continue
      </button>
    </div>
  )
}

// ============================================================================
// Step 1: OTP Verification
// ============================================================================

function StepOTP({ onSuccess }: { onSuccess: (token: string) => void }) {
  const [otp, setOtp] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [expiresAt, setExpiresAt] = useState<Date | null>(null)
  const [now, setNow] = useState(() => Date.now())

  // Show when the current code expires; refresh after it does, because the
  // server then issues a new code on its screen.
  useEffect(() => {
    let cancelled = false
    const load = async () => {
      try {
        const st = await api.setup.getState()
        if (!cancelled && st?.expiresAt) setExpiresAt(new Date(st.expiresAt))
      } catch {
        // the submit path reports errors
      }
    }
    void load()
    const tick = setInterval(() => setNow(Date.now()), 1000)
    return () => {
      cancelled = true
      clearInterval(tick)
    }
  }, [])
  useEffect(() => {
    if (!expiresAt || now < expiresAt.getTime()) return
    let cancelled = false
    api.setup
      .getState()
      .then((st) => {
        if (!cancelled && st?.expiresAt) setExpiresAt(new Date(st.expiresAt))
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [expiresAt, now])
  const secondsLeft = expiresAt ? Math.max(0, Math.round((expiresAt.getTime() - now) / 1000)) : null
  
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    
    const cleanOtp = otp.replace(/\s+/g, '')
    if (cleanOtp.length !== 6) {
      setError('Please enter the 6-digit code')
      return
    }
    
    try {
      setLoading(true)
      setError(null)
      
      const response = await api.setup.verifyOTP(cleanOtp) as any
      
      if (response?.token) {
        toast.success('Code verified')
        onSuccess(response.token)
      }
    } catch (err) {
      if (err instanceof APIError) {
        if (err.code === 'setup.otp.expired') {
          setError('That code expired. A new code is now shown on the server screen.')
          setOtp('')
          api.setup
            .getState()
            .then((st) => st?.expiresAt && setExpiresAt(new Date(st.expiresAt)))
            .catch(() => {})
        } else if (err.code === 'setup.complete') {
          setError('Setup was already completed. Please sign in.')
        } else if (err.status === 400 || err.status === 401 || err.status === 403) {
          setError("That code isn't correct. Check the code on the server screen; after 5 wrong tries it is replaced with a new one.")
        } else if (err.status === 429) {
          const retryMsg = err.retryAfterSec 
            ? `Too many attempts. Try again in ${err.retryAfterSec}s`
            : 'Too many attempts. Please try again later.'
          setError(retryMsg)
        } else {
          setError(getErrorMessage(err))
        }
      } else {
        setError(getErrorMessage(err))
      }
    } finally {
      setLoading(false)
    }
  }
  
  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div>
        <label htmlFor="otp" className="block text-sm font-medium mb-2">
          One-time setup code
        </label>
        <input
          id="otp"
          type="text"
          className="field text-center tracking-widest font-mono text-lg"
          placeholder="123 456"
          value={otp}
          onChange={(e) => setOtp(e.target.value)}
          autoFocus
          autoComplete="off"
          maxLength={7} // 6 digits + 1 space
          disabled={loading}
        />
        <p className="text-xs text-muted-foreground mt-1">
          Enter the 6-digit code shown on the server screen
          {secondsLeft !== null && secondsLeft > 0 && (
            <> (valid for {Math.floor(secondsLeft / 60)}:{String(secondsLeft % 60).padStart(2, '0')} more)</>
          )}
          {secondsLeft === 0 && <> (expired; a new code is on the server screen)</>}
        </p>
      </div>
      
      {error && (
        <div className="text-sm text-status-failed-text">
          {error}
        </div>
      )}
      
      <button
        type="submit"
        className="btn btn-primary w-full py-3"
        disabled={loading}
      >
        {loading ? 'Verifying...' : 'Verify code'}
      </button>
    </form>
  )
}

// ============================================================================
// Step 2: Create Admin Account
// ============================================================================

function StepCreateAdmin({ 
  token, 
  onSuccess,
  onExpired,
}: { 
  token: string
  onSuccess: (enableTotp: boolean, creds?: { username: string; password: string }) => void
  /** The setup token (10 minutes) ran out: start again from the code. */
  onExpired: () => void
}) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  
  const {
    register,
    handleSubmit,
    watch,
    formState: { errors },
  } = useForm<CreateAdminInput>({
    resolver: zodResolver(CreateAdminSchema),
    defaultValues: {
      username: '',
      password: '',
      confirmPassword: '',
      enableTotp: false,
    },
  })
  
  const password = watch('password')
  const strength = passwordStrength(password || '')
  
  const onSubmit = async (data: CreateAdminInput) => {
    try {
      setLoading(true)
      setError(null)
      
      await api.setup.createAdmin(token, {
        username: data.username,
        password: data.password,
        enable_totp: data.enableTotp,
      })
      
      toast.success('Admin account created successfully')
      
      // Login immediately after creating admin (needed for steps 4-7)
      if (!data.enableTotp) {
        try {
          await api.auth.login({
            username: data.username,
            password: data.password,
          })
        } catch (loginErr) {
          console.warn('Auto-login after admin creation failed:', loginErr)
          // Continue anyway - user can login manually if needed
        }
      }
      
      if (data.enableTotp) {
        onSuccess(true, { username: data.username, password: data.password })
      } else {
        onSuccess(false)
      }
    } catch (err) {
      if (err instanceof APIError) {
        if (err.code === 'setup.session.invalid') {
          onExpired()
          return
        }
        if (err.code === 'setup.write_failed') {
          setError('Cannot write configuration. Check server permissions for /etc/nos/users.json')
        } else if (err.code === 'input.username_taken') {
          setError('Username already taken')
        } else if (err.code === 'input.weak_password') {
          setError('Password too weak. Use at least 12 characters with mixed case, numbers, and symbols.')
        } else {
          setError(getErrorMessage(err))
        }
      } else {
        setError(getErrorMessage(err))
      }
    } finally {
      setLoading(false)
    }
  }
  
  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
      <div>
        <label htmlFor="username" className="block text-sm font-medium mb-2">
          Username
        </label>
        <input
          id="username"
          type="text"
          className="field"
          placeholder="admin"
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
          placeholder="••••••••••••"
          autoComplete="new-password"
          disabled={loading}
          {...register('password')}
        />
        <PasswordStrengthMeter strength={strength} />
        {errors.password && (
          <p className="text-xs text-status-failed-text mt-1">{errors.password.message}</p>
        )}
      </div>
      
      <div>
        <label htmlFor="confirmPassword" className="block text-sm font-medium mb-2">
          Confirm password
        </label>
        <input
          id="confirmPassword"
          type="password"
          className="field"
          placeholder="••••••••••••"
          autoComplete="new-password"
          disabled={loading}
          {...register('confirmPassword')}
        />
        {errors.confirmPassword && (
          <p className="text-xs text-status-failed-text mt-1">{errors.confirmPassword.message}</p>
        )}
      </div>
      
      <div className="flex items-center">
        <input
          id="enableTotp"
          type="checkbox"
          className="mr-2"
          disabled={loading}
          {...register('enableTotp')}
        />
        <label htmlFor="enableTotp" className="text-sm">
          Enable two-factor authentication (recommended)
        </label>
      </div>
      
      {error && <div className="text-sm text-status-failed-text">{error}</div>}
      
      <button
        type="submit"
        className="btn btn-primary w-full py-3"
        disabled={loading}
      >
        {loading ? 'Creating the administrator…' : 'Create administrator account'}
      </button>
    </form>
  )
}

// ============================================================================
// Password Strength Meter
// ============================================================================

function PasswordStrengthMeter({ strength }: { strength: number }) {
  // Blocks that fill one by one, the strength in words: too weak is
  // failed, fair a warning, good and strong in order.
  const labels = ['Very weak', 'Weak', 'Fair', 'Good', 'Strong']
  const tone = strength <= 1 ? 'failed' : strength === 2 ? 'warning' : 'ok'

  return (
    <div className="mt-2 flex items-center gap-3">
      <Meter value={strength + 1} max={5} blocks={5} label="Password strength" showValue={false} tone={tone} />
      <span className="text-sm">{labels[strength]}</span>
    </div>
  )
}

// ============================================================================
// Step 3: TOTP Enrollment (optional)
// ============================================================================

function StepTOTPEnroll({ 
  credentials, 
  onSuccess 
}: { 
  credentials: { username: string; password: string }
  onSuccess: () => void
}) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [qrCode, setQrCode] = useState<string | null>(null)
  const [secret, setSecret] = useState<string | null>(null)
  const [totpCode, setTotpCode] = useState('')
  const [recoveryCodes, setRecoveryCodes] = useState<string[] | null>(null)
  
  // Login and get TOTP enrollment data
  useEffect(() => {
    enrollTOTP()
  }, [])
  
  const enrollTOTP = async () => {
    try {
      setLoading(true)
      
      // First login with the new admin credentials
      await api.auth.login({
        username: credentials.username,
        password: credentials.password,
      })
      
      // Get TOTP enrollment data
      const enrollData = await api.auth.totp.enroll() as any
      
      // Generate QR code
      if (enrollData?.qr_png_base64) {
        setQrCode(`data:image/png;base64,${enrollData.qr_png_base64}`)
      } else if (enrollData?.otpauth_url) {
        const dataUrl = await QRCode.toDataURL(enrollData.otpauth_url)
        setQrCode(dataUrl)
      }
      
      // Extract secret from otpauth URL
      try {
        const url = new URL(enrollData?.otpauth_url || '')
        const extractedSecret = url.searchParams.get('secret')
        if (extractedSecret) {
          setSecret(extractedSecret)
        }
      } catch {
        // Ignore URL parsing errors
      }
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setLoading(false)
    }
  }
  
  const handleVerify = async (e: React.FormEvent) => {
    e.preventDefault()
    
    const cleanCode = totpCode.replace(/\s+/g, '')
    if (cleanCode.length !== 6) {
      setError('Please enter the 6-digit code from your authenticator app')
      return
    }
    
    try {
      setLoading(true)
      setError(null)
      
      const response = await api.auth.totp.verify(cleanCode) as any
      
      if (response?.recovery_codes) {
        setRecoveryCodes(response.recovery_codes)
      } else {
        // No recovery codes returned, just proceed
        onSuccess()
      }
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setLoading(false)
    }
  }
  
  const handleCopyRecoveryCodes = () => {
    if (recoveryCodes) {
      navigator.clipboard.writeText(recoveryCodes.join('\n'))
      toast.success('Recovery codes copied to clipboard')
    }
  }
  
  const handleDownloadRecoveryCodes = () => {
    if (recoveryCodes) {
      const blob = new Blob([recoveryCodes.join('\n')], { type: 'text/plain' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'nos-recovery-codes.txt'
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(url)
    }
  }
  
  // Show recovery codes if we have them
  if (recoveryCodes) {
    return (
      <div className="space-y-4">
        <h2 className="text-lg font-[560]">Recovery codes</h2>
        <p className="text-sm text-muted-foreground">
          Save these recovery codes in a safe place. Each code can be used once to sign in if you lose access to your authenticator.
        </p>
        
        <div className="border border-hairline p-4 font-mono text-sm space-y-1">
          {recoveryCodes.map((code, index) => (
            <div key={index}>{code}</div>
          ))}
        </div>
        
        <div className="flex gap-2">
          <button
            type="button"
            className="btn btn-line flex-1"
            onClick={handleCopyRecoveryCodes}
          >
            Copy
          </button>
          <button
            type="button"
            className="btn btn-line flex-1"
            onClick={handleDownloadRecoveryCodes}
          >
            Download
          </button>
        </div>
        
        <button
          type="button"
          className="btn btn-primary w-full"
          onClick={onSuccess}
        >
          Continue to setup complete
        </button>
      </div>
    )
  }
  
  // Show QR code and verification form
  return (
    <div className="space-y-4">
      <h2 className="text-lg font-[560]">Enable two-factor authentication</h2>
      
      {loading && !qrCode && (
        <div className="text-center py-8">
          <div className="text-muted-foreground">Loading...</div>
        </div>
      )}
      
      {qrCode && (
        <>
          <div className="text-center">
            <img src={qrCode} alt="TOTP QR Code" className="mx-auto" />
          </div>
          
          {secret && (
            <div className="text-center">
              <p className="text-xs text-muted-foreground mb-1">Manual entry code:</p>
              <code className="text-sm font-mono bg-card px-2 py-1 rounded">{secret}</code>
            </div>
          )}
          
          <form onSubmit={handleVerify} className="space-y-4">
            <div>
              <label htmlFor="totpCode" className="block text-sm font-medium mb-2">
                Verification code
              </label>
              <input
                id="totpCode"
                type="text"
                className="field text-center tracking-widest font-mono"
                placeholder="123 456"
                value={totpCode}
                onChange={(e) => setTotpCode(e.target.value)}
                autoFocus
                autoComplete="off"
                maxLength={7}
                disabled={loading}
              />
              <p className="text-xs text-muted-foreground mt-1">
                Enter the 6-digit code from your authenticator app
              </p>
            </div>
            
            {error && <div className="text-sm text-status-failed-text">{error}</div>}
            
            <button
              type="submit"
              className="btn btn-primary w-full"
              disabled={loading}
            >
              {loading ? 'Verifying...' : 'Verify and Enable 2FA'}
            </button>
          </form>
          <button
            type="button"
            className="btn btn-line w-full"
            onClick={onSuccess}
            disabled={loading}
          >
            Skip for now
          </button>
        </>
      )}
    </div>
  )
}

// ============================================================================
// Step 4: Setup Complete
// ============================================================================

function StepDone({ onContinue }: { onContinue: () => void }) {
  return (
    <div className="text-center space-y-4">
      <div className="text-4xl mb-4">✅</div>
      <h2 className="text-lg font-[560]">Setup complete!</h2>
      <p className="text-sm text-muted-foreground">
        Your nOS admin account has been created successfully.
        You can now sign in to access the dashboard.
      </p>
      <button
        className="btn btn-primary w-full py-3"
        onClick={onContinue}
      >
        Go to sign in
      </button>
    </div>
  )
}

// ============================================================================
// Step: System Configuration (Hostname, Timezone)
// ============================================================================

function StepSystemConfig({ onSuccess }: { onSuccess: () => void }) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [hostname, setHostname] = useState('')
  const [selectedRegion, setSelectedRegion] = useState('UTC')
  const [timezone, setTimezone] = useState('UTC')
  const [ntp, setNtp] = useState(true)
  
  useEffect(() => {
    loadCurrentConfig()
  }, [])
  
  // When region changes, auto-select the first timezone in that region
  useEffect(() => {
    const region = TIMEZONE_REGIONS[selectedRegion]
    if (region && region.timezones.length > 0) {
      setTimezone(region.timezones[0].id)
    }
  }, [selectedRegion])
  
  const loadCurrentConfig = async () => {
    try {
      const [timezoneRes, ntpRes] = await Promise.all([
        api.system.getTimezone(),
        api.system.getNTP(),
      ])
      
      // Don't pre-populate hostname during setup - let user choose
      const currentTz = (timezoneRes as any).timezone || 'UTC'
      setTimezone(currentTz)
      
      // Find which region this timezone belongs to
      const tzInfo = getTimezoneInfo(currentTz)
      if (tzInfo) {
        setSelectedRegion(tzInfo.region)
      }
      
      setNtp((ntpRes as any).enabled)
    } catch (err) {
      console.error('Failed to load system config:', err)
    }
  }
  
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    
    if (!hostname || hostname.length < 1) {
      setError('Please enter a hostname')
      return
    }
    
    // Validate hostname (RFC 1123)
    const hostnameRegex = /^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$/
    if (!hostnameRegex.test(hostname)) {
      setError('Invalid hostname format')
      return
    }
    
    try {
      setLoading(true)
      setError(null)
      
      await Promise.all([
        api.system.setHostname({ hostname }),
        api.system.setTimezone({ timezone }),
        api.system.setNTP({ enabled: ntp }),
      ])
      
      toast.success('System configuration updated')
      onSuccess()
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setLoading(false)
    }
  }
  
  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <h2 className="text-lg font-[560]">System configuration</h2>
      
      <div>
        <label htmlFor="hostname" className="block text-sm font-medium mb-2">
          Hostname
        </label>
        <input
          id="hostname"
          type="text"
          className="field"
          placeholder="nos"
          value={hostname}
          onChange={(e) => setHostname(e.target.value)}
          disabled={loading}
        />
        <p className="text-xs text-muted-foreground mt-1">
          The name for this system on the network
        </p>
      </div>
      
      <div>
        <label htmlFor="region" className="block text-sm font-medium mb-2">
          Region
        </label>
        <select
          id="region"
          className="field"
          value={selectedRegion}
          onChange={(e) => setSelectedRegion(e.target.value)}
          disabled={loading}
        >
          {Object.keys(TIMEZONE_REGIONS).map((region) => (
            <option key={region} value={region}>{TIMEZONE_REGIONS[region].name}</option>
          ))}
        </select>
      </div>
      
      <div>
        <label htmlFor="timezone" className="block text-sm font-medium mb-2">
          Timezone
        </label>
        <select
          id="timezone"
          className="field"
          value={timezone}
          onChange={(e) => setTimezone(e.target.value)}
          disabled={loading}
        >
          {TIMEZONE_REGIONS[selectedRegion]?.timezones.map((tz) => (
            <option key={tz.id} value={tz.id}>{tz.name}</option>
          ))}
        </select>
      </div>
      
      <div className="flex items-center">
        <input
          id="ntp"
          type="checkbox"
          className="mr-2"
          checked={ntp}
          onChange={(e) => setNtp(e.target.checked)}
          disabled={loading}
        />
        <label htmlFor="ntp" className="text-sm">
          Enable automatic time synchronization (NTP)
        </label>
      </div>
      
      {error && <div className="text-sm text-status-failed-text">{error}</div>}
      
      <button
        type="submit"
        className="btn btn-primary w-full"
        disabled={loading}
      >
        {loading ? 'Saving...' : 'Continue'}
      </button>
    </form>
  )
}

// ============================================================================
// Step: Network Configuration
// ============================================================================

function StepNetworkConfig({ onSuccess }: { onSuccess: () => void }) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [interfaces, setInterfaces] = useState<any[]>([])
  const [selectedInterface, setSelectedInterface] = useState<string>('')
  const [dhcp, setDhcp] = useState(true)
  const [ipAddress, setIpAddress] = useState('')
  const [gateway, setGateway] = useState('')
  const [dns, setDns] = useState<string[]>(['8.8.8.8', '8.8.4.4'])
  
  useEffect(() => {
    loadInterfaces()
  }, [])
  
  const loadInterfaces = async () => {
    try {
      const res = await api.network.getInterfaces() as any
      const ifaces = res?.interfaces || []
      setInterfaces(ifaces)
      
      // Auto-select first ethernet interface
      const ethInterface = ifaces.find((i: any) => 
        i.type === 'ethernet' && i.state === 'up'
      ) || ifaces[0]
      
      if (ethInterface) {
        setSelectedInterface(ethInterface.name)
        setDhcp(ethInterface.dhcp)
        
        if (ethInterface.ipv4_address && ethInterface.ipv4_address.length > 0) {
          setIpAddress(ethInterface.ipv4_address[0])
        }
        
        if (ethInterface.gateway) {
          setGateway(ethInterface.gateway)
        }
        
        if (ethInterface.dns && ethInterface.dns.length > 0) {
          setDns(ethInterface.dns)
        }
      }
    } catch (err) {
      console.error('Failed to load interfaces:', err)
    }
  }
  
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    
    if (!selectedInterface) {
      setError('Please select a network interface')
      return
    }
    
    if (!dhcp) {
      if (!ipAddress || !gateway) {
        setError('Static configuration requires IP address and gateway')
        return
      }
      
      // Validate IP format
      const ipRegex = /^(\d{1,3}\.){3}\d{1,3}\/\d{1,2}$/
      if (!ipRegex.test(ipAddress)) {
        setError('Invalid IP address format (use CIDR notation, e.g., 192.168.1.100/24)')
        return
      }
    }
    
    try {
      setLoading(true)
      setError(null)
      
      await api.network.configureInterface(selectedInterface, {
        dhcp,
        ipv4_address: dhcp ? undefined : ipAddress,
        ipv4_gateway: dhcp ? undefined : gateway,
        dns: dhcp ? undefined : dns.filter(d => d),
      })
      
      toast.success('Network configuration updated')
      onSuccess()
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setLoading(false)
    }
  }
  
  const handleSkip = () => {
    toast.info('Using current network configuration')
    onSuccess()
  }
  
  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <h2 className="text-lg font-[560]">Network configuration</h2>
      
      <div>
        <label htmlFor="interface" className="block text-sm font-medium mb-2">
          Network interface
        </label>
        <select
          id="interface"
          className="field"
          value={selectedInterface}
          onChange={(e) => setSelectedInterface(e.target.value)}
          disabled={loading}
        >
          <option value="">Select interface...</option>
          {interfaces.map((iface) => (
            <option key={iface.name} value={iface.name}>
              {iface.name} ({iface.type}) - {iface.state}
            </option>
          ))}
        </select>
      </div>
      
      <div className="flex items-center">
        <input
          id="dhcp"
          type="checkbox"
          className="mr-2"
          checked={dhcp}
          onChange={(e) => setDhcp(e.target.checked)}
          disabled={loading}
        />
        <label htmlFor="dhcp" className="text-sm">
          Use DHCP (automatic configuration)
        </label>
      </div>
      
      {!dhcp && (
        <>
          <div>
            <label htmlFor="ipAddress" className="block text-sm font-medium mb-2">
              IP address (CIDR)
            </label>
            <input
              id="ipAddress"
              type="text"
              className="field"
              placeholder="192.168.1.100/24"
              value={ipAddress}
              onChange={(e) => setIpAddress(e.target.value)}
              disabled={loading}
            />
          </div>
          
          <div>
            <label htmlFor="gateway" className="block text-sm font-medium mb-2">
              Gateway
            </label>
            <input
              id="gateway"
              type="text"
              className="field"
              placeholder="192.168.1.1"
              value={gateway}
              onChange={(e) => setGateway(e.target.value)}
              disabled={loading}
            />
          </div>
          
          <div>
            <label htmlFor="dns1" className="block text-sm font-medium mb-2">
              DNS servers
            </label>
            <input
              id="dns1"
              type="text"
              className="field mb-2"
              placeholder="Primary DNS"
              value={dns[0] || ''}
              onChange={(e) => setDns([e.target.value, dns[1] || ''])}
              disabled={loading}
            />
            <input
              id="dns2"
              type="text"
              className="field"
              placeholder="Secondary DNS (optional)"
              value={dns[1] || ''}
              onChange={(e) => setDns([dns[0] || '', e.target.value])}
              disabled={loading}
            />
          </div>
        </>
      )}
      
      {error && <div className="text-sm text-status-failed-text">{error}</div>}
      
      <div className="flex gap-2">
        <button
          type="button"
          className="btn btn-line flex-1"
          onClick={handleSkip}
          disabled={loading}
        >
          Skip
        </button>
        <button
          type="submit"
          className="btn btn-primary flex-1"
          disabled={loading}
        >
          {loading ? 'Saving...' : 'Continue'}
        </button>
      </div>
    </form>
  )
}

// ============================================================================
// Step: Telemetry Consent
// ============================================================================

function StepTelemetry({ onSuccess }: { onSuccess: () => void }) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [consent, setConsent] = useState(false)
  const [dataTypes, setDataTypes] = useState({
    system_info: true,
    usage_stats: true,
    error_reports: true,
  })
  
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    
    try {
      setLoading(true)
      setError(null)
      
      const selectedTypes = Object.entries(dataTypes)
        .filter(([_, enabled]) => enabled)
        .map(([type]) => type)
      
      await api.telemetry.setConsent({
        enabled: consent,
        data_types: consent ? selectedTypes : [],
      })
      
      toast.success(
        consent 
          ? 'Thank you for helping improve nOS!' 
          : 'Telemetry disabled'
      )
      onSuccess()
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setLoading(false)
    }
  }
  
  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <h2 className="text-lg font-[560]">Help Improve nOS</h2>
      
      <p className="text-sm text-muted-foreground">
        Would you like to share anonymous usage data to help us improve nOS? 
        This is completely optional and can be changed later in settings.
      </p>
      
      <div className="border border-muted rounded p-4 space-y-2">
        <div className="flex items-center">
          <input
            id="consent"
            type="checkbox"
            className="mr-2"
            checked={consent}
            onChange={(e) => setConsent(e.target.checked)}
            disabled={loading}
          />
          <label htmlFor="consent" className="text-sm font-medium">
            Enable anonymous telemetry
          </label>
        </div>
        
        {consent && (
          <div className="ml-6 space-y-2 text-sm">
            <div className="flex items-center">
              <input
                id="system_info"
                type="checkbox"
                className="mr-2"
                checked={dataTypes.system_info}
                onChange={(e) => setDataTypes({...dataTypes, system_info: e.target.checked})}
                disabled={loading}
              />
              <label htmlFor="system_info">
                System information (OS version, hardware specs)
              </label>
            </div>
            
            <div className="flex items-center">
              <input
                id="usage_stats"
                type="checkbox"
                className="mr-2"
                checked={dataTypes.usage_stats}
                onChange={(e) => setDataTypes({...dataTypes, usage_stats: e.target.checked})}
                disabled={loading}
              />
              <label htmlFor="usage_stats">
                Usage statistics (feature usage, performance metrics)
              </label>
            </div>
            
            <div className="flex items-center">
              <input
                id="error_reports"
                type="checkbox"
                className="mr-2"
                checked={dataTypes.error_reports}
                onChange={(e) => setDataTypes({...dataTypes, error_reports: e.target.checked})}
                disabled={loading}
              />
              <label htmlFor="error_reports">
                Error reports (crash logs, error messages)
              </label>
            </div>
          </div>
        )}
      </div>
      
      <div className="border border-hairline p-3 text-xs text-muted-foreground">
        <strong>Privacy promise:</strong> We never collect personal data, file names, 
        file contents, or any identifiable information. All data is aggregated and 
        anonymous. You can view exactly what data is sent in the logs.
      </div>
      
      {error && <div className="text-sm text-status-failed-text">{error}</div>}
      
      <button
        type="submit"
        className="btn btn-primary w-full"
        disabled={loading}
      >
        {loading ? 'Saving...' : 'Continue'}
      </button>
    </form>
  )
}