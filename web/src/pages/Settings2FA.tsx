import { useState } from 'react'
import { 
  Shield,
  Smartphone,
  Lock,
  Unlock,
  AlertCircle,
  Copy,
  Download,
  RefreshCw,
  Trash2,
  Info,
} from 'lucide-react'
import { PageHeader } from '@/components/ui/page-header'
import { Card } from '@/components/ui/card-enhanced'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import { cn } from '@/lib/utils'
import { toast } from '@/components/ui/toast'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/nos-client'
import QRCode from 'qrcode'

// 2FA Status type
interface TwoFactorStatus {
  enabled: boolean
  method: 'totp' | 'sms' | 'email' | null
  backupCodesCount: number
  lastUsed?: string
  devices?: AuthDevice[]
}

// Auth Device type
interface AuthDevice {
  id: string
  name: string
  type: 'totp' | 'webauthn' | 'passkey'
  addedAt: string
  lastUsed?: string
}

// Recovery Code type


export function Settings2FA() {
  const [showEnableDialog, setShowEnableDialog] = useState(false)
  const [showDisableDialog, setShowDisableDialog] = useState(false)
  const [showRecoveryDialog, setShowRecoveryDialog] = useState(false)
  const [verificationCode, setVerificationCode] = useState('')
  const [password, setPassword] = useState('')
  const [qrCodeUrl, setQrCodeUrl] = useState('')
  const [secret, setSecret] = useState('')
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([])
  const queryClient = useQueryClient()

  // Fetch 2FA status
  const { data: status, isLoading } = useQuery({
    queryKey: ['2fa-status'],
    queryFn: async () => {
      const response = await api.get<any>('/v1/auth/2fa/status')
      return response as TwoFactorStatus
    },
  })

  // Enable 2FA mutation
  const enableMutation = useMutation({
    mutationFn: async () => {
      // Step 1: Initialize 2FA
      // nosd: {secret, uri (otpauth://), qr_code, backup_codes}
      const initResponse = await api.post<any>('/v1/auth/2fa/enroll', { password })
      const { secret, uri, backup_codes } = initResponse

      setSecret(secret)
      setRecoveryCodes(backup_codes || [])

      // Generate QR code
      const qrDataUrl = await QRCode.toDataURL(uri)
      setQrCodeUrl(qrDataUrl)
      
      return initResponse
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.message || 'Failed to enable 2FA')
    }
  })

  // Verify 2FA mutation
  const verifyMutation = useMutation({
    mutationFn: async () => {
      const response = await api.post<any>('/v1/auth/2fa/verify', {
        code: verificationCode,
        secret,
      })
      return response
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['2fa-status'] })
      toast.success('Two-factor authentication enabled successfully')
      setShowEnableDialog(false)
      resetEnableState()
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.message || 'Invalid verification code')
    }
  })

  // Disable 2FA mutation
  const disableMutation = useMutation({
    mutationFn: async () => {
      const response = await api.post<any>('/v1/auth/2fa/disable', {
        password,
        code: verificationCode,
      })
      return response
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['2fa-status'] })
      toast.success('Two-factor authentication disabled')
      setShowDisableDialog(false)
      resetDisableState()
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.message || 'Failed to disable 2FA')
    }
  })

  // Generate recovery codes mutation
  const generateRecoveryMutation = useMutation({
    mutationFn: async () => {
      const response = await api.post<any>('/v1/auth/2fa/backup-codes', { password })
      return response.backup_codes
    },
    onSuccess: (codes) => {
      setRecoveryCodes(codes)
      queryClient.invalidateQueries({ queryKey: ['2fa-status'] })
      toast.success('New recovery codes generated')
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.message || 'Failed to generate recovery codes')
    }
  })

  const resetEnableState = () => {
    setPassword('')
    setVerificationCode('')
    setQrCodeUrl('')
    setSecret('')
    setRecoveryCodes([])
  }

  const resetDisableState = () => {
    setPassword('')
    setVerificationCode('')
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
    toast.success('Copied to clipboard')
  }

  const downloadRecoveryCodes = () => {
    const content = recoveryCodes.join('\n')
    const blob = new Blob([content], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'nos-recovery-codes.txt'
    a.click()
    URL.revokeObjectURL(url)
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-8">
        <RefreshCw className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  return (
    <div className="container mx-auto py-6 space-y-6">
      <PageHeader
        title="Two-Factor Authentication"
        description="Add an extra layer of security to your account"
        icon={Shield}
      />

      {/* Status Card */}
      <Card>
        <div className="p-6">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-4">
              <div className={cn(
                "p-3 rounded-full",
                status?.enabled ? "bg-green-100 dark:bg-green-900" : "bg-gray-100 dark:bg-gray-800"
              )}>
                {status?.enabled ? (
                  <Lock className="h-6 w-6 text-green-600 dark:text-green-400" />
                ) : (
                  <Unlock className="h-6 w-6 text-gray-600 dark:text-gray-400" />
                )}
              </div>
              <div>
                <h3 className="text-lg font-semibold">
                  Two-Factor Authentication is {status?.enabled ? 'Enabled' : 'Disabled'}
                </h3>
                <p className="text-sm text-muted-foreground mt-1">
                  {status?.enabled 
                    ? 'Your account is protected with two-factor authentication'
                    : 'Enable two-factor authentication for enhanced security'
                  }
                </p>
              </div>
            </div>
            <Button
              variant={status?.enabled ? "destructive" : "default"}
              onClick={() => status?.enabled ? setShowDisableDialog(true) : setShowEnableDialog(true)}
            >
              {status?.enabled ? 'Disable 2FA' : 'Enable 2FA'}
            </Button>
          </div>
        </div>
      </Card>

      {status?.enabled && (
        <>
          {/* Authentication Methods */}
          <Card>
            <div className="p-6">
              <h3 className="text-lg font-semibold mb-4">Authentication Method</h3>
              <div className="space-y-4">
                <div className="flex items-center justify-between p-4 border rounded-lg">
                  <div className="flex items-center gap-3">
                    <Smartphone className="h-5 w-5 text-muted-foreground" />
                    <div>
                      <p className="font-medium">Authenticator App</p>
                      <p className="text-sm text-muted-foreground">
                        Use an app like Google Authenticator or Authy
                      </p>
                    </div>
                  </div>
                  {status.method === 'totp' && (
                    <Badge variant="success">Active</Badge>
                  )}
                </div>
              </div>
            </div>
          </Card>

          {/* Recovery Codes */}
          <Card>
            <div className="p-6">
              <div className="flex items-center justify-between mb-4">
                <div>
                  <h3 className="text-lg font-semibold">Recovery Codes</h3>
                  <p className="text-sm text-muted-foreground mt-1">
                    Use these codes to access your account if you lose your device
                  </p>
                </div>
                <Badge variant="outline">
                  {status.backupCodesCount} codes remaining
                </Badge>
              </div>
              
              {status.backupCodesCount < 3 && (
                <Alert className="mb-4">
                  <AlertCircle className="h-4 w-4" />
                  <AlertDescription>
                    You have {status.backupCodesCount} recovery codes remaining. 
                    Consider generating new codes.
                  </AlertDescription>
                </Alert>
              )}
              
              <div className="flex gap-4">
                <Button 
                  variant="outline"
                  onClick={() => setShowRecoveryDialog(true)}
                >
                  View Recovery Codes
                </Button>
                <Button 
                  variant="outline"
                  onClick={() => {
                    if (confirm('This will invalidate your existing recovery codes. Continue?')) {
                      generateRecoveryMutation.mutate()
                    }
                  }}
                  disabled={generateRecoveryMutation.isPending}
                >
                  {generateRecoveryMutation.isPending ? (
                    <>
                      <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                      Generating...
                    </>
                  ) : (
                    <>
                      <RefreshCw className="mr-2 h-4 w-4" />
                      Generate New Codes
                    </>
                  )}
                </Button>
              </div>
            </div>
          </Card>

          {/* Trusted Devices */}
          <Card>
            <div className="p-6">
              <h3 className="text-lg font-semibold mb-4">Trusted Devices</h3>
              {status.devices && status.devices.length > 0 ? (
                <div className="space-y-2">
                  {status.devices.map((device) => (
                    <div key={device.id} className="flex items-center justify-between p-3 border rounded-lg">
                      <div className="flex items-center gap-3">
                        <Smartphone className="h-5 w-5 text-muted-foreground" />
                        <div>
                          <p className="font-medium">{device.name}</p>
                          <p className="text-sm text-muted-foreground">
                            Added {new Date(device.addedAt).toLocaleDateString()}
                            {device.lastUsed && ` • Last used ${new Date(device.lastUsed).toLocaleDateString()}`}
                          </p>
                        </div>
                      </div>
                      <Button variant="ghost" size="sm">
                        <Trash2 className="h-4 w-4 text-red-500" />
                      </Button>
                    </div>
                  ))}
                </div>
              ) : (
                <p className="text-muted-foreground">No trusted devices registered</p>
              )}
            </div>
          </Card>
        </>
      )}

      {/* Security Tips */}
      <Card>
        <div className="p-6">
          <h3 className="text-lg font-semibold mb-4">Security Tips</h3>
          <div className="space-y-3">
            <div className="flex gap-3">
              <Info className="h-5 w-5 text-blue-500 mt-0.5" />
              <div>
                <p className="font-medium">Use a dedicated authenticator app</p>
                <p className="text-sm text-muted-foreground">
                  Apps like Google Authenticator, Authy, or 1Password provide secure code generation
                </p>
              </div>
            </div>
            <div className="flex gap-3">
              <Info className="h-5 w-5 text-blue-500 mt-0.5" />
              <div>
                <p className="font-medium">Keep recovery codes safe</p>
                <p className="text-sm text-muted-foreground">
                  Store them in a secure location, separate from your device
                </p>
              </div>
            </div>
            <div className="flex gap-3">
              <Info className="h-5 w-5 text-blue-500 mt-0.5" />
              <div>
                <p className="font-medium">Never share your codes</p>
                <p className="text-sm text-muted-foreground">
                  nOS will never ask for your 2FA codes via email or phone
                </p>
              </div>
            </div>
          </div>
        </div>
      </Card>

      {/* Enable 2FA Dialog */}
      <Dialog open={showEnableDialog} onOpenChange={setShowEnableDialog}>
        <DialogContent className="sm:max-w-[500px]">
          <DialogHeader>
            <DialogTitle>Enable Two-Factor Authentication</DialogTitle>
          </DialogHeader>
          
          {!qrCodeUrl ? (
            <div className="space-y-4 py-4">
              <Alert>
                <Info className="h-4 w-4" />
                <AlertDescription>
                  You'll need an authenticator app on your phone to continue
                </AlertDescription>
              </Alert>
              
              <div>
                <Label htmlFor="password">Confirm Password</Label>
                <Input
                  id="password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="Enter your password"
                />
              </div>
              
              <DialogFooter>
                <Button variant="outline" onClick={() => setShowEnableDialog(false)}>
                  Cancel
                </Button>
                <Button 
                  onClick={() => enableMutation.mutate()}
                  disabled={!password || enableMutation.isPending}
                >
                  {enableMutation.isPending ? (
                    <>
                      <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                      Setting up...
                    </>
                  ) : (
                    'Continue'
                  )}
                </Button>
              </DialogFooter>
            </div>
          ) : (
            <div className="space-y-4 py-4">
              <div className="text-center">
                <p className="text-sm text-muted-foreground mb-4">
                  Scan this QR code with your authenticator app
                </p>
                {qrCodeUrl && (
                  <img src={qrCodeUrl} alt="QR Code" className="mx-auto" />
                )}
                <p className="text-xs text-muted-foreground mt-4">
                  Or enter this code manually:
                </p>
                <div className="flex items-center justify-center gap-2 mt-2">
                  <code className="text-sm font-mono bg-muted px-2 py-1 rounded">
                    {secret}
                  </code>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => copyToClipboard(secret)}
                  >
                    <Copy className="h-4 w-4" />
                  </Button>
                </div>
              </div>
              
              {recoveryCodes.length > 0 && (
                <Alert>
                  <AlertCircle className="h-4 w-4" />
                  <AlertDescription>
                    <strong>Save your recovery codes!</strong>
                    <div className="grid grid-cols-2 gap-2 mt-2">
                      {recoveryCodes.map((code, i) => (
                        <code key={i} className="text-xs font-mono bg-muted px-2 py-1 rounded">
                          {code}
                        </code>
                      ))}
                    </div>
                    <Button
                      variant="outline"
                      size="sm"
                      className="mt-2"
                      onClick={downloadRecoveryCodes}
                    >
                      <Download className="mr-2 h-4 w-4" />
                      Download Codes
                    </Button>
                  </AlertDescription>
                </Alert>
              )}
              
              <div>
                <Label htmlFor="verification">Verification Code</Label>
                <Input
                  id="verification"
                  value={verificationCode}
                  onChange={(e) => setVerificationCode(e.target.value)}
                  placeholder="Enter 6-digit code"
                  maxLength={6}
                />
              </div>
              
              <DialogFooter>
                <Button 
                  variant="outline" 
                  onClick={() => {
                    setShowEnableDialog(false)
                    resetEnableState()
                  }}
                >
                  Cancel
                </Button>
                <Button 
                  onClick={() => verifyMutation.mutate()}
                  disabled={verificationCode.length !== 6 || verifyMutation.isPending}
                >
                  {verifyMutation.isPending ? (
                    <>
                      <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                      Verifying...
                    </>
                  ) : (
                    'Verify & Enable'
                  )}
                </Button>
              </DialogFooter>
            </div>
          )}
        </DialogContent>
      </Dialog>

      {/* Disable 2FA Dialog */}
      <Dialog open={showDisableDialog} onOpenChange={setShowDisableDialog}>
        <DialogContent className="sm:max-w-[400px]">
          <DialogHeader>
            <DialogTitle>Disable Two-Factor Authentication</DialogTitle>
          </DialogHeader>
          
          <Alert variant="destructive">
            <AlertCircle className="h-4 w-4" />
            <AlertDescription>
              Disabling 2FA will make your account less secure. Are you sure?
            </AlertDescription>
          </Alert>
          
          <div className="space-y-4 py-4">
            <div>
              <Label htmlFor="disable-password">Password</Label>
              <Input
                id="disable-password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="Enter your password"
              />
            </div>
            
            <div>
              <Label htmlFor="disable-code">2FA Code</Label>
              <Input
                id="disable-code"
                value={verificationCode}
                onChange={(e) => setVerificationCode(e.target.value)}
                placeholder="Enter 6-digit code"
                maxLength={6}
              />
            </div>
          </div>
          
          <DialogFooter>
            <Button 
              variant="outline" 
              onClick={() => {
                setShowDisableDialog(false)
                resetDisableState()
              }}
            >
              Cancel
            </Button>
            <Button 
              variant="destructive"
              onClick={() => disableMutation.mutate()}
              disabled={!password || verificationCode.length !== 6 || disableMutation.isPending}
            >
              {disableMutation.isPending ? (
                <>
                  <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                  Disabling...
                </>
              ) : (
                'Disable 2FA'
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Recovery Codes Dialog */}
      <Dialog open={showRecoveryDialog} onOpenChange={setShowRecoveryDialog}>
        <DialogContent className="sm:max-w-[400px]">
          <DialogHeader>
            <DialogTitle>Recovery Codes</DialogTitle>
          </DialogHeader>
          
          <Alert>
            <Info className="h-4 w-4" />
            <AlertDescription>
              Enter your password to view recovery codes
            </AlertDescription>
          </Alert>
          
          <div className="space-y-4 py-4">
            <div>
              <Label htmlFor="recovery-password">Password</Label>
              <Input
                id="recovery-password"
                type="password"
                placeholder="Enter your password"
              />
            </div>
          </div>
          
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowRecoveryDialog(false)}>
              Cancel
            </Button>
            <Button>View Codes</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
