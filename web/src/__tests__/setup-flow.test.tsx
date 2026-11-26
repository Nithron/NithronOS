import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BrowserRouter } from 'react-router-dom'

import Setup from '../pages/Setup'
import http from '../lib/nos-client'
import { GlobalNoticeProvider } from '../lib/globalNotice'

// Mock the API client
vi.mock('../lib/nos-client', () => ({
  default: {
    setup: {
      getState: vi.fn(),
      verifyOTP: vi.fn(),
      createAdmin: vi.fn(),
    },
    auth: {
      totp: {
        enroll: vi.fn(),
        verify: vi.fn(),
      },
    },
  },
  api: {
    setup: {
      getState: vi.fn(),
      verifyOTP: vi.fn(),
      createAdmin: vi.fn(),
    },
    auth: {
      totp: {
        enroll: vi.fn(),
        verify: vi.fn(),
      },
    },
    system: {
      setHostname: vi.fn(),
      setTimezone: vi.fn(),
      setNTP: vi.fn(),
    },
    network: {
      getInterfaces: vi.fn(),
      setInterface: vi.fn(),
    },
    telemetry: {
      setConfig: vi.fn(),
    },
  },
  APIError: class APIError extends Error {
    constructor(public status: number, message: string) {
      super(message)
    }
  },
  ProxyMisconfiguredError: class ProxyMisconfiguredError extends Error {
    constructor(message: string) {
      super(message)
      this.name = 'ProxyMisconfiguredError'
    }
  },
  getErrorMessage: (err: any) => err?.message || 'An error occurred',
}))

// Mock navigation
const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

// Mock QRCode component
vi.mock('qrcode.react', () => ({
  QRCodeSVG: ({ value }: { value: string }) => (
    <div data-testid="qrcode">{value}</div>
  ),
}))



describe.skip('Setup Flow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockNavigate.mockClear()
  })

  const renderSetup = () => {
    return render(
      <BrowserRouter>
        <GlobalNoticeProvider>
          <Setup />
        </GlobalNoticeProvider>
      </BrowserRouter>
    )
  }

  describe('Setup State Check', () => {
    it('should redirect to login if setup is already complete', async () => {
      vi.mocked(http.setup.getState).mockRejectedValueOnce({
        status: 410,
        message: 'Setup already completed',
      })
      
      renderSetup()
      const user = userEvent.setup()
      
      // Wait for error message and button
      await waitFor(() => {
        expect(screen.getByText(/setup already completed/i)).toBeInTheDocument()
      })
      
      // Click button to go to login
      await user.click(screen.getByRole('button', { name: /go to sign in/i }))
      
      expect(mockNavigate).toHaveBeenCalledWith('/login')
    })

    it('should show OTP form when first boot requires OTP', async () => {
      vi.mocked(http.setup.getState).mockResolvedValueOnce({
        firstBoot: true,
        otpRequired: true,
      })
      
      renderSetup()
      
      await waitFor(() => {
        expect(screen.getByLabelText(/one-time password/i)).toBeInTheDocument()
        expect(screen.getByLabelText(/one-time password/i)).toBeInTheDocument()
      })
    })

    it('should show waiting message when OTP not required', async () => {
      vi.mocked(http.setup.getState).mockResolvedValueOnce({
        firstBoot: true,
        otpRequired: false,
      })
      
      renderSetup()
      
      await waitFor(() => {
        expect(screen.getByText(/waiting for otp generation/i)).toBeInTheDocument()
      })
    })
  })

  describe('OTP Verification', () => {
    beforeEach(async () => {
      vi.mocked(http.setup.getState).mockResolvedValueOnce({
        firstBoot: true,
        otpRequired: true,
      })
    })

    it('should validate OTP format', async () => {
      renderSetup()
      const user = userEvent.setup()
      
      await waitFor(() => {
        expect(screen.getByLabelText(/one-time password/i)).toBeInTheDocument()
      })
      
      const otpInput = screen.getByLabelText(/one-time password/i)
      const verifyButton = screen.getByRole('button', { name: /verify/i })
      
      // Too short
      await user.type(otpInput, '123')
      await user.click(verifyButton)
      
      await waitFor(() => {
        expect(screen.getByText(/must be at least 6 characters/i)).toBeInTheDocument()
      })
    })

    it('should handle successful OTP verification', async () => {
      vi.mocked(http.setup.verifyOTP).mockResolvedValueOnce({ ok: true, token: 'mock-token' })
      
      renderSetup()
      const user = userEvent.setup()
      
      await waitFor(() => {
        expect(screen.getByLabelText(/one-time password/i)).toBeInTheDocument()
      })
      
      await user.type(screen.getByLabelText(/one-time password/i), 'test-otp-123')
      await user.click(screen.getByRole('button', { name: /verify/i }))
      
      await waitFor(() => {
        expect(http.setup.verifyOTP).toHaveBeenCalledWith('test-otp-123')
        expect(screen.getByRole('button', { name: /create admin account/i })).toBeInTheDocument()
      })
    })

    it('should handle invalid OTP error', async () => {
      vi.mocked(http.setup.verifyOTP).mockRejectedValueOnce({
        status: 401,
        message: 'Invalid OTP',
      })
      
      renderSetup()
      const user = userEvent.setup()
      
      await waitFor(() => {
        expect(screen.getByLabelText(/one-time password/i)).toBeInTheDocument()
      })
      
      await user.type(screen.getByLabelText(/one-time password/i), 'wrong-otp')
      await user.click(screen.getByRole('button', { name: /verify/i }))
      
      await waitFor(() => {
        expect(screen.getByText(/invalid/i)).toBeInTheDocument()
      })
    })

    it('should show permission hint on 403 error', async () => {
      vi.mocked(http.setup.verifyOTP).mockRejectedValueOnce({
        status: 403,
        message: 'Forbidden',
      })
      
      renderSetup()
      const user = userEvent.setup()
      
      await waitFor(() => {
        expect(screen.getByLabelText(/one-time password/i)).toBeInTheDocument()
      })
      
      await user.type(screen.getByLabelText(/one-time password/i), 'test-otp')
      await user.click(screen.getByRole('button', { name: /verify/i }))
      
      await waitFor(() => {
        expect(screen.getByText(/permission denied/i)).toBeInTheDocument()
      })
    })
  })

  describe('Admin Account Creation', () => {
    beforeEach(async () => {
      vi.mocked(http.setup.getState).mockResolvedValueOnce({
        firstBoot: true,
        otpRequired: false,
      })
    })

    it('should validate username and password', async () => {
      renderSetup()
      const user = userEvent.setup()
      
      await waitFor(() => {
        expect(screen.getByLabelText(/username/i)).toBeInTheDocument()
      })
      
      const createButton = screen.getByRole('button', { name: /create admin account/i })
      await user.click(createButton)
      
      await waitFor(() => {
        expect(screen.getByText(/username must be at least 3 characters/i)).toBeInTheDocument()
        expect(screen.getByText(/password must be at least 8 characters/i)).toBeInTheDocument()
      })
    })

    it('should validate password confirmation', async () => {
      renderSetup()
      const user = userEvent.setup()
      
      await waitFor(() => {
        expect(screen.getByLabelText(/username/i)).toBeInTheDocument()
      })
      
      await user.type(screen.getByLabelText(/username/i), 'admin')
      await user.type(screen.getByLabelText(/password/i), 'StrongPassword123!')
      await user.type(screen.getByLabelText(/confirm password/i), 'DifferentPassword')
      
      const createButton = screen.getByRole('button', { name: /create admin account/i })
      await user.click(createButton)
      
      await waitFor(() => {
        expect(screen.getByText(/passwords do not match/i)).toBeInTheDocument()
      })
    })

    it('should show password strength indicator', async () => {
      renderSetup()
      const user = userEvent.setup()
      
      await waitFor(() => {
        expect(screen.getByLabelText(/password/i)).toBeInTheDocument()
      })
      
      const passwordInput = screen.getByLabelText(/password/i)
      
      // Weak password
      await user.clear(passwordInput)
      await user.type(passwordInput, 'weak')
      await waitFor(() => {
        expect(screen.getByText(/weak/i)).toBeInTheDocument()
      })
      
      // Medium password
      await user.clear(passwordInput)
      await user.type(passwordInput, 'Medium123')
      await waitFor(() => {
        expect(screen.getByText(/medium/i)).toBeInTheDocument()
      })
      
      // Strong password
      await user.clear(passwordInput)
      await user.type(passwordInput, 'Strong@Password123!')
      await waitFor(() => {
        expect(screen.getByText(/strong/i)).toBeInTheDocument()
      })
    })

    it('should handle successful admin creation without TOTP', async () => {
      vi.mocked(http.setup.createAdmin).mockResolvedValueOnce({
        success: true,
        totpRequired: false,
      })
      
      renderSetup()
      const user = userEvent.setup()
      
      await waitFor(() => {
        expect(screen.getByLabelText(/username/i)).toBeInTheDocument()
      })
      
      await user.type(screen.getByLabelText(/username/i), 'admin')
      await user.type(screen.getByLabelText(/password/i), 'StrongPassword123!')
      await user.type(screen.getByLabelText(/confirm password/i), 'StrongPassword123!')
      
      const createButton = screen.getByRole('button', { name: /create admin account/i })
      await user.click(createButton)
      
      await waitFor(() => {
        expect(http.setup.createAdmin).toHaveBeenCalledWith({
          username: 'admin',
          password: 'StrongPassword123!',
        })
        expect(mockNavigate).toHaveBeenCalledWith('/login', { replace: true })
      })
    })

    it('should show TOTP enrollment when required', async () => {
      vi.mocked(http.setup.createAdmin).mockResolvedValueOnce({
        success: true,
        totpRequired: true,
      })
      
      vi.mocked(http.auth.totp.enroll).mockResolvedValueOnce({

        otpauth_url: 'otpauth://totp/nOS:admin?secret=MOCK_SECRET',
      })
      
      renderSetup()
      const user = userEvent.setup()
      
      await waitFor(() => {
        expect(screen.getByLabelText(/username/i)).toBeInTheDocument()
      })
      
      await user.type(screen.getByLabelText(/username/i), 'admin')
      await user.type(screen.getByLabelText(/password/i), 'StrongPassword123!')
      await user.type(screen.getByLabelText(/confirm password/i), 'StrongPassword123!')
      
      const createButton = screen.getByRole('button', { name: /create admin account/i })
      await user.click(createButton)
      
      await waitFor(() => {
        expect(screen.getByText(/enable two-factor authentication/i)).toBeInTheDocument()
        expect(screen.getByTestId('qrcode')).toBeInTheDocument()
        expect(screen.getByText(/MOCK_SECRET/)).toBeInTheDocument()
      })
    })
  })

  describe('TOTP Enrollment', () => {
    it('should handle TOTP verification after enrollment', async () => {
      // Setup state
      vi.mocked(http.setup.getState).mockResolvedValueOnce({
        firstBoot: true,
        otpRequired: false,
      })
      
      // Admin creation requires TOTP
      vi.mocked(http.setup.createAdmin).mockResolvedValueOnce({
        success: true,
        totpRequired: true,
      })
      
      // TOTP enrollment
      vi.mocked(http.auth.totp.enroll).mockResolvedValueOnce({

        otpauth_url: 'otpauth://totp/nOS:admin?secret=MOCK_SECRET',
      })
      
      // TOTP verification
      vi.mocked(http.auth.totp.verify).mockResolvedValueOnce({ ok: true })
      
      renderSetup()
      const user = userEvent.setup()
      
      // Complete admin creation
      await waitFor(() => {
        expect(screen.getByLabelText(/username/i)).toBeInTheDocument()
      })
      
      await user.type(screen.getByLabelText(/username/i), 'admin')
      await user.type(screen.getByLabelText(/password/i), 'StrongPassword123!')
      await user.type(screen.getByLabelText(/confirm password/i), 'StrongPassword123!')
      await user.click(screen.getByRole('button', { name: /create admin account/i }))
      
      // Should show TOTP enrollment
      await waitFor(() => {
        expect(screen.getByText(/enable two-factor authentication/i)).toBeInTheDocument()
      })
      
      // Enter TOTP code
      await user.type(screen.getByLabelText(/verification code/i), '123456')
      await user.click(screen.getByRole('button', { name: /verify and continue/i }))
      
      await waitFor(() => {
        expect(http.auth.totp.verify).toHaveBeenCalledWith('123456')
        expect(mockNavigate).toHaveBeenCalledWith('/login', { replace: true })
      })
    })

    it('should allow skipping TOTP enrollment', async () => {
      vi.mocked(http.setup.getState).mockResolvedValueOnce({
        firstBoot: true,
        otpRequired: false,
      })
      
      vi.mocked(http.setup.createAdmin).mockResolvedValueOnce({
        success: true,
        totpRequired: true,
      })
      
      vi.mocked(http.auth.totp.enroll).mockResolvedValueOnce({

        otpauth_url: 'otpauth://totp/nOS:admin?secret=MOCK_SECRET',
      })
      
      renderSetup()
      const user = userEvent.setup()
      
      // Complete admin creation
      await waitFor(() => {
        expect(screen.getByLabelText(/username/i)).toBeInTheDocument()
      })
      
      await user.type(screen.getByLabelText(/username/i), 'admin')
      await user.type(screen.getByLabelText(/password/i), 'StrongPassword123!')
      await user.type(screen.getByLabelText(/confirm password/i), 'StrongPassword123!')
      await user.click(screen.getByRole('button', { name: /create admin account/i }))
      
      // Should show TOTP enrollment
      await waitFor(() => {
        expect(screen.getByText(/enable two-factor authentication/i)).toBeInTheDocument()
      })
      
      // Skip TOTP
      await user.click(screen.getByRole('button', { name: /skip for now/i }))
      
      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/login', { replace: true })
      })
    })
  })
})
