/**
 * Sync Encryption Settings Page
 * Manage end-to-end encryption for NithronSync
 */

import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  Shield,
  Key,
  Lock,
  Unlock,
  RefreshCw,
  AlertTriangle,
  Copy,
  Eye,
  EyeOff,
  HardDrive,
  Settings,
  Download,
} from 'lucide-react';

import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  CardFooter,
} from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { toast } from 'sonner';

import {
  getEncryptionStatus,
  initializeEncryption,
  unlockEncryption,
  lockEncryption,
  changeEncryptionPassword,
  generateRecoveryKey,
  recoverWithKey,
  getEncryptionSettings,
  updateEncryptionSettings,
  listKeys,
  encryptionKeys,
  EncryptionSettings,
} from '@/api/encryption';

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

export default function SyncEncryption() {
  const queryClient = useQueryClient();
  
  // Dialog states
  const [initDialogOpen, setInitDialogOpen] = useState(false);
  const [unlockDialogOpen, setUnlockDialogOpen] = useState(false);
  const [changePasswordOpen, setChangePasswordOpen] = useState(false);
  const [recoveryDialogOpen, setRecoveryDialogOpen] = useState(false);
  const [showRecoveryKey, setShowRecoveryKey] = useState(false);
  const [recoveryKey, setRecoveryKey] = useState('');
  
  // Form states
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [oldPassword, setOldPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [recoveryInput, setRecoveryInput] = useState('');
  const [showPassword, setShowPassword] = useState(false);

  // Fetch status
  const {
    data: status,
    isLoading: loadingStatus,
    refetch: refetchStatus,
  } = useQuery({
    queryKey: encryptionKeys.status(),
    queryFn: getEncryptionStatus,
  });

  // Fetch settings
  const { data: settings, isLoading: _loadingSettings } = useQuery({
    queryKey: encryptionKeys.settings(),
    queryFn: getEncryptionSettings,
  });

  // Fetch keys
  const { data: keys, isLoading: loadingKeys } = useQuery({
    queryKey: encryptionKeys.keys(),
    queryFn: listKeys,
  });

  // Initialize mutation
  const initMutation = useMutation({
    mutationFn: () => initializeEncryption(password),
    onSuccess: (data) => {
      setRecoveryKey(data.recovery_key);
      setShowRecoveryKey(true);
      queryClient.invalidateQueries({ queryKey: encryptionKeys.all });
      setInitDialogOpen(false);
      setPassword('');
      setConfirmPassword('');
      toast.success('Encryption initialized successfully');
    },
    onError: (error: Error) => {
      toast.error('Failed to initialize encryption', { description: error.message });
    },
  });

  // Unlock mutation
  const unlockMutation = useMutation({
    mutationFn: () => unlockEncryption(password),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: encryptionKeys.all });
      setUnlockDialogOpen(false);
      setPassword('');
      toast.success('Encryption unlocked');
    },
    onError: (error: Error) => {
      toast.error('Failed to unlock', { description: error.message });
    },
  });

  // Lock mutation
  const lockMutation = useMutation({
    mutationFn: lockEncryption,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: encryptionKeys.all });
      toast.success('Encryption locked');
    },
    onError: (error: Error) => {
      toast.error('Failed to lock', { description: error.message });
    },
  });

  // Change password mutation
  const changePasswordMutation = useMutation({
    mutationFn: () => changeEncryptionPassword(oldPassword, newPassword),
    onSuccess: () => {
      setChangePasswordOpen(false);
      setOldPassword('');
      setNewPassword('');
      toast.success('Password changed successfully');
    },
    onError: (error: Error) => {
      toast.error('Failed to change password', { description: error.message });
    },
  });

  // Generate recovery key mutation
  const generateRecoveryMutation = useMutation({
    mutationFn: generateRecoveryKey,
    onSuccess: (data) => {
      setRecoveryKey(data.recovery_key);
      setShowRecoveryKey(true);
      toast.success('New recovery key generated');
    },
    onError: (error: Error) => {
      toast.error('Failed to generate recovery key', { description: error.message });
    },
  });

  // Recover mutation
  const recoverMutation = useMutation({
    mutationFn: () => recoverWithKey(recoveryInput),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: encryptionKeys.all });
      setRecoveryDialogOpen(false);
      setRecoveryInput('');
      toast.success('Encryption recovered successfully');
    },
    onError: (error: Error) => {
      toast.error('Recovery failed', { description: error.message });
    },
  });

  // Update settings mutation
  const updateSettingsMutation = useMutation({
    mutationFn: updateEncryptionSettings,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: encryptionKeys.settings() });
      toast.success('Settings updated');
    },
    onError: (error: Error) => {
      toast.error('Failed to update settings', { description: error.message });
    },
  });

  const copyRecoveryKey = () => {
    navigator.clipboard.writeText(recoveryKey);
    toast.success('Recovery key copied to clipboard');
  };

  if (loadingStatus) {
    return (
      <div className="container mx-auto py-6 space-y-6">
        <Skeleton className="h-12 w-64" />
        <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
          <Skeleton className="h-40" />
          <Skeleton className="h-40" />
          <Skeleton className="h-40" />
        </div>
      </div>
    );
  }

  return (
    <div className="container mx-auto py-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">End-to-End Encryption</h1>
          <p className="text-muted-foreground">
            Secure your files with client-side encryption
          </p>
        </div>
        <Button variant="outline" onClick={() => refetchStatus()}>
          <RefreshCw className="h-4 w-4 mr-2" />
          Refresh
        </Button>
      </div>

      {/* Recovery Key Dialog (shown after init) */}
      {showRecoveryKey && (
        <Alert className="border-yellow-500/50 bg-yellow-500/5">
          <AlertTriangle className="h-5 w-5 text-yellow-500" />
          <AlertTitle>Save Your Recovery Key</AlertTitle>
          <AlertDescription className="space-y-3">
            <p>
              This is your only chance to save your recovery key. Store it in a safe place.
              You'll need it if you forget your password.
            </p>
            <div className="flex items-center gap-2">
              <code className="flex-1 p-3 bg-muted rounded-md font-mono text-sm break-all">
                {recoveryKey}
              </code>
              <Button variant="outline" size="icon" onClick={copyRecoveryKey}>
                <Copy className="h-4 w-4" />
              </Button>
            </div>
            <Button variant="outline" onClick={() => setShowRecoveryKey(false)}>
              I've saved my recovery key
            </Button>
          </AlertDescription>
        </Alert>
      )}

      {/* Status Cards */}
      <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-4">
        {/* Encryption Status */}
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Encryption Status
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              {status?.enabled ? (
                <>
                  <Shield className="h-8 w-8 text-green-500" />
                  <div>
                    <p className="text-2xl font-bold">Enabled</p>
                    <p className="text-xs text-muted-foreground">{status.algorithm}</p>
                  </div>
                </>
              ) : (
                <>
                  <Shield className="h-8 w-8 text-muted-foreground" />
                  <div>
                    <p className="text-2xl font-bold">Disabled</p>
                    <p className="text-xs text-muted-foreground">Not configured</p>
                  </div>
                </>
              )}
            </div>
          </CardContent>
        </Card>

        {/* Master Key Status */}
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Master Key
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              {status?.master_key_initialized ? (
                <>
                  <Key className="h-8 w-8 text-green-500" />
                  <div>
                    <p className="text-2xl font-bold">Ready</p>
                    <p className="text-xs text-muted-foreground">Initialized</p>
                  </div>
                </>
              ) : (
                <>
                  <Key className="h-8 w-8 text-yellow-500" />
                  <div>
                    <p className="text-2xl font-bold">Not Set</p>
                    <p className="text-xs text-muted-foreground">Setup required</p>
                  </div>
                </>
              )}
            </div>
          </CardContent>
        </Card>

        {/* Encrypted Shares */}
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Encrypted Shares
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              <HardDrive className="h-8 w-8 text-blue-500" />
              <div>
                <p className="text-2xl font-bold">{status?.shares_encrypted || 0}</p>
                <p className="text-xs text-muted-foreground">
                  {status?.total_encrypted_files || 0} files
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Encrypted Size */}
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Encrypted Data
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              <Lock className="h-8 w-8 text-purple-500" />
              <div>
                <p className="text-2xl font-bold">
                  {formatBytes(status?.total_encrypted_size || 0)}
                </p>
                <p className="text-xs text-muted-foreground">Total encrypted</p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Actions */}
      {!status?.master_key_initialized ? (
        <Card>
          <CardHeader>
            <CardTitle>Setup Encryption</CardTitle>
            <CardDescription>
              Initialize end-to-end encryption to secure your files
            </CardDescription>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground mb-4">
              End-to-end encryption ensures that only you can access your files.
              Your data is encrypted on your device before being uploaded, and only you
              have the keys to decrypt it.
            </p>
            <Button onClick={() => setInitDialogOpen(true)}>
              <Shield className="h-4 w-4 mr-2" />
              Initialize Encryption
            </Button>
          </CardContent>
        </Card>
      ) : (
        <Tabs defaultValue="settings">
          <TabsList>
            <TabsTrigger value="settings">
              <Settings className="h-4 w-4 mr-2" />
              Settings
            </TabsTrigger>
            <TabsTrigger value="keys">
              <Key className="h-4 w-4 mr-2" />
              Keys
            </TabsTrigger>
            <TabsTrigger value="recovery">
              <Download className="h-4 w-4 mr-2" />
              Recovery
            </TabsTrigger>
          </TabsList>

          {/* Settings Tab */}
          <TabsContent value="settings" className="mt-4">
            <Card>
              <CardHeader>
                <CardTitle>Encryption Settings</CardTitle>
                <CardDescription>
                  Configure how encryption works for your sync shares
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                <div className="space-y-2">
                  <Label>Encryption Algorithm</Label>
                  <Select
                    value={settings?.default_algorithm}
                    onValueChange={(value) =>
                      updateSettingsMutation.mutate({
                        default_algorithm: value as EncryptionSettings['default_algorithm'],
                      })
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="xchacha20-poly1305">
                        XChaCha20-Poly1305 (Recommended)
                      </SelectItem>
                      <SelectItem value="chacha20-poly1305">
                        ChaCha20-Poly1305
                      </SelectItem>
                      <SelectItem value="aes-256-gcm">
                        AES-256-GCM
                      </SelectItem>
                    </SelectContent>
                  </Select>
                  <p className="text-xs text-muted-foreground">
                    XChaCha20-Poly1305 is recommended for its security and performance
                  </p>
                </div>

                <div className="flex items-center justify-between">
                  <div className="space-y-0.5">
                    <Label>Encrypt New Shares</Label>
                    <p className="text-xs text-muted-foreground">
                      Automatically enable encryption for new sync shares
                    </p>
                  </div>
                  <Switch
                    checked={settings?.encrypt_new_shares}
                    onCheckedChange={(checked) =>
                      updateSettingsMutation.mutate({ encrypt_new_shares: checked })
                    }
                  />
                </div>

                <div className="flex items-center justify-between">
                  <div className="space-y-0.5">
                    <Label>Encrypt Filenames</Label>
                    <p className="text-xs text-muted-foreground">
                      Also encrypt file and folder names (may impact search)
                    </p>
                  </div>
                  <Switch
                    checked={settings?.encrypt_filenames}
                    onCheckedChange={(checked) =>
                      updateSettingsMutation.mutate({ encrypt_filenames: checked })
                    }
                  />
                </div>

                <div className="space-y-2">
                  <Label>Key Rotation (days)</Label>
                  <Input
                    type="number"
                    value={settings?.key_rotation_days || 90}
                    onChange={(e) =>
                      updateSettingsMutation.mutate({
                        key_rotation_days: parseInt(e.target.value) || 90,
                      })
                    }
                    min={30}
                    max={365}
                  />
                  <p className="text-xs text-muted-foreground">
                    Automatically rotate encryption keys periodically
                  </p>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          {/* Keys Tab */}
          <TabsContent value="keys" className="mt-4">
            <Card>
              <CardHeader>
                <CardTitle>Encryption Keys</CardTitle>
                <CardDescription>
                  Manage your encryption keys
                </CardDescription>
              </CardHeader>
              <CardContent>
                {loadingKeys ? (
                  <div className="space-y-3">
                    <Skeleton className="h-16" />
                    <Skeleton className="h-16" />
                  </div>
                ) : (
                  <div className="space-y-3">
                    {keys?.map((key) => (
                      <div
                        key={key.id}
                        className="flex items-center justify-between p-4 border rounded-lg"
                      >
                        <div className="flex items-center gap-3">
                          <Key className="h-5 w-5 text-muted-foreground" />
                          <div>
                            <p className="font-medium capitalize">{key.type} Key</p>
                            <p className="text-xs text-muted-foreground">
                              Created {new Date(key.created_at).toLocaleDateString()}
                            </p>
                          </div>
                        </div>
                        <Badge variant="secondary">{key.algorithm}</Badge>
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
              <CardFooter className="flex gap-2">
                <Button variant="outline" onClick={() => setChangePasswordOpen(true)}>
                  Change Password
                </Button>
                <Button variant="outline" onClick={() => lockMutation.mutate()}>
                  <Lock className="h-4 w-4 mr-2" />
                  Lock Encryption
                </Button>
              </CardFooter>
            </Card>
          </TabsContent>

          {/* Recovery Tab */}
          <TabsContent value="keys" className="mt-4">
            <Card>
              <CardHeader>
                <CardTitle>Recovery Options</CardTitle>
                <CardDescription>
                  Recover access to your encrypted data
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <Alert>
                  <AlertTriangle className="h-4 w-4" />
                  <AlertTitle>Keep your recovery key safe</AlertTitle>
                  <AlertDescription>
                    If you lose your password and recovery key, your encrypted data
                    cannot be recovered.
                  </AlertDescription>
                </Alert>

                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    onClick={() => generateRecoveryMutation.mutate()}
                  >
                    Generate New Recovery Key
                  </Button>
                  <Button
                    variant="outline"
                    onClick={() => setRecoveryDialogOpen(true)}
                  >
                    Recover with Key
                  </Button>
                </div>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      )}

      {/* Initialize Dialog */}
      <Dialog open={initDialogOpen} onOpenChange={setInitDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Initialize Encryption</DialogTitle>
            <DialogDescription>
              Create a password to protect your encryption keys
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Password</Label>
              <div className="relative">
                <Input
                  type={showPassword ? 'text' : 'password'}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="Enter a strong password"
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="absolute right-0 top-0"
                  onClick={() => setShowPassword(!showPassword)}
                >
                  {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </Button>
              </div>
            </div>
            <div className="space-y-2">
              <Label>Confirm Password</Label>
              <Input
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                placeholder="Confirm your password"
              />
            </div>
            {password && password.length < 8 && (
              <p className="text-sm text-yellow-500">
                Password should be at least 8 characters
              </p>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setInitDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              onClick={() => initMutation.mutate()}
              disabled={
                password.length < 8 ||
                password !== confirmPassword ||
                initMutation.isPending
              }
            >
              Initialize
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Unlock Dialog */}
      <Dialog open={unlockDialogOpen} onOpenChange={setUnlockDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Unlock Encryption</DialogTitle>
            <DialogDescription>
              Enter your password to access encrypted data
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Password</Label>
              <Input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="Enter your password"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setUnlockDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              onClick={() => unlockMutation.mutate()}
              disabled={!password || unlockMutation.isPending}
            >
              <Unlock className="h-4 w-4 mr-2" />
              Unlock
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Change Password Dialog */}
      <Dialog open={changePasswordOpen} onOpenChange={setChangePasswordOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Change Password</DialogTitle>
            <DialogDescription>
              Update your encryption password
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Current Password</Label>
              <Input
                type="password"
                value={oldPassword}
                onChange={(e) => setOldPassword(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label>New Password</Label>
              <Input
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setChangePasswordOpen(false)}>
              Cancel
            </Button>
            <Button
              onClick={() => changePasswordMutation.mutate()}
              disabled={!oldPassword || newPassword.length < 8 || changePasswordMutation.isPending}
            >
              Change Password
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Recovery Dialog */}
      <Dialog open={recoveryDialogOpen} onOpenChange={setRecoveryDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Recover Encryption</DialogTitle>
            <DialogDescription>
              Enter your recovery key to regain access
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Recovery Key</Label>
              <Input
                value={recoveryInput}
                onChange={(e) => setRecoveryInput(e.target.value)}
                placeholder="Enter your recovery key"
                className="font-mono"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRecoveryDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              onClick={() => recoverMutation.mutate()}
              disabled={!recoveryInput || recoverMutation.isPending}
            >
              Recover
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

