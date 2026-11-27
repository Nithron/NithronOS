import { useState, useEffect, useRef } from 'react';
import { Mark } from '@/components/ui/pixel';
import QRCode from 'qrcode';
import { pairingLink } from '@/lib/pairing';
import { AddIcon, ChosenIcon, CopyIcon, DeleteIcon, DesktopIcon, EditIcon, FailedIcon, LaptopIcon, PhoneIcon, RefreshIcon, TerminalIcon } from '@/components/icons'
import { Button, buttonVariants } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { toast } from '@/components/ui/toast';
import {
  listDevices,
  revokeDevice,
  updateDevice,
  registerDevice,
  formatDeviceType,
  formatBytes,
  formatRelativeTime,
  isDeviceActive,
  type SyncDevice,
  type DeviceType,
} from '@/api/sync';

// QR Code component
function TokenQRCode({ 
  link,
  size = 200 
}: { 
  link: string;
  size?: number;
}) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!canvasRef.current || !link) return;
    const syncUrl = link;

    QRCode.toCanvas(canvasRef.current, syncUrl, {
      width: size,
      margin: 2,
      color: {
        // A QR code is black on white in both themes: scanners fail on
        // inverted codes.
        dark: '#000000', // ui-check: fixed colour
        light: '#ffffff', // ui-check: fixed colour
      },
      errorCorrectionLevel: 'M',
    }, (err) => {
      if (err) {
        console.error('QR code generation error:', err);
        setError('Failed to generate QR code');
      }
    });
  }, [link, size]);

  if (error) {
    return (
      <div className="flex items-center justify-center p-4 bg-status-failed-tint border border-status-failed rounded-lg">
        <FailedIcon className="h-5 w-5 text-status-failed-text mr-2" />
        <span className="text-sm text-status-failed-text">{error}</span>
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center">
      <canvas 
        ref={canvasRef} 
        data-testid="qr-code"
        className="rounded-lg border border-hairline"
      />
      <p className="text-xs text-muted-foreground mt-2 text-center max-w-[200px]">
        Scan with NithronSync app to connect this device
      </p>
    </div>
  );
}


// Device icon component
function DeviceIcon({ type, className = 'h-5 w-5' }: { type: DeviceType; className?: string }) {
  switch (type) {
    case 'windows':
      return <DesktopIcon className={className} />;
    case 'linux':
      return <TerminalIcon className={className} />;
    case 'macos':
      return <LaptopIcon className={className} />;
    case 'android':
    case 'ios':
      return <PhoneIcon className={className} />;
    default:
      return <DesktopIcon className={className} />;
  }
}

// Device card component
function DeviceCard({
  device,
  onEdit,
  onRevoke,
}: {
  device: SyncDevice;
  onEdit: (device: SyncDevice) => void;
  onRevoke: (device: SyncDevice) => void;
}) {
  const isActive = isDeviceActive(device);

  return (
    <Card className="relative overflow-hidden">
      
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-3">
            <div className="text-ink-2">
              <DeviceIcon type={device.device_type} className="h-6 w-6" />
            </div>
            <div>
              <CardTitle className="text-lg font-[560]">{device.device_name}</CardTitle>
              <CardDescription className="flex items-center gap-2">
                <span>{formatDeviceType(device.device_type)}</span>
                {device.os_version && (
                  <>
                    <span className="text-ink">•</span>
                    <span>{device.os_version}</span>
                  </>
                )}
              </CardDescription>
            </div>
          </div>
          <Badge variant="outline" className="shrink-0">
            <Mark state={isActive ? 'ok' : 'off'} />
            {isActive ? 'Active' : 'Offline'}
          </Badge>
        </div>
      </CardHeader>
      
      <CardContent>
        <div className="grid grid-cols-2 gap-4 text-sm mb-4">
          <div>
            <span className="text-muted-foreground">Last sync</span>
            <p className="font-medium">{formatRelativeTime(device.last_sync_at)}</p>
          </div>
          <div>
            <span className="text-muted-foreground">Last seen</span>
            <p className="font-medium">{formatRelativeTime(device.last_seen_at)}</p>
          </div>
          <div>
            <span className="text-muted-foreground">Total synced</span>
            <p className="font-medium">{formatBytes(device.bytes_synced)}</p>
          </div>
          <div>
            <span className="text-muted-foreground">Sync count</span>
            <p className="font-medium">{device.sync_count.toLocaleString()}</p>
          </div>
        </div>
        
        {device.last_ip && (
          <div className="text-sm text-muted-foreground mb-4">
            Last IP: <code className="font-mono text-xs text-ink">{device.last_ip}</code>
          </div>
        )}
        
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={() => onEdit(device)}>
            <EditIcon className="h-4 w-4 mr-1" />
            Rename
          </Button>
          <Button variant="destructive" size="sm" onClick={() => onRevoke(device)}>
            <DeleteIcon className="h-4 w-4 mr-1" />
            Revoke
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

// Main component
export default function SyncDevices() {
  const [devices, setDevices] = useState<SyncDevice[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  
  // Dialog states
  const [editDevice, setEditDevice] = useState<SyncDevice | null>(null);
  const [editName, setEditName] = useState('');
  const [revokeDeviceTarget, setRevokeDeviceTarget] = useState<SyncDevice | null>(null);
  const [showAddDialog, setShowAddDialog] = useState(false);
  
  // New device form
  const [newDeviceName, setNewDeviceName] = useState('');
  const [newDeviceType, setNewDeviceType] = useState<DeviceType>('windows');
  const [registeredDevice, setRegisteredDevice] = useState<{
    device_id: string;
    device_token: string;
    refresh_token: string;
    server_key_sha256?: string;
  } | null>(null);
  const [copiedToken, setCopiedToken] = useState(false);

  // Load devices
  const loadDevices = async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await listDevices();
      setDevices(response.devices || []);
    } catch (err) {
      setError('Failed to load sync devices');
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadDevices();
  }, []);

  // Handle edit
  const handleEditSave = async () => {
    if (!editDevice || !editName.trim()) return;
    
    try {
      await updateDevice(editDevice.id, { device_name: editName.trim() });
      toast.success('Device renamed successfully');
      setEditDevice(null);
      loadDevices();
    } catch {
      toast.error('Failed to rename device');
    }
  };

  // Handle revoke
  const handleRevoke = async () => {
    if (!revokeDeviceTarget) return;
    
    try {
      await revokeDevice(revokeDeviceTarget.id);
      toast.success('Device access revoked');
      setRevokeDeviceTarget(null);
      loadDevices();
    } catch {
      toast.error('Failed to revoke device');
    }
  };

  // Handle register new device
  const handleRegister = async () => {
    if (!newDeviceName.trim()) return;
    
    try {
      const response = await registerDevice({
        device_name: newDeviceName.trim(),
        device_type: newDeviceType,
      });
      setRegisteredDevice(response);
      toast.success('Device registered successfully');
      loadDevices();
    } catch (err: any) {
      if (err?.message?.includes('limit')) {
        toast.error('Maximum device limit reached');
      } else {
        toast.error('Failed to register device');
      }
    }
  };

  const registeredLink = registeredDevice
    ? pairingLink({
        server: window.location.origin,
        deviceToken: registeredDevice.device_token,
        refreshToken: registeredDevice.refresh_token,
        serverKey: registeredDevice.server_key_sha256,
        deviceName: newDeviceName,
      })
    : '';

  // Copy the pairing link to the clipboard
  const copyToken = () => {
    if (!registeredDevice) return;
    navigator.clipboard.writeText(registeredLink);
    setCopiedToken(true);
    setTimeout(() => setCopiedToken(false), 2000);
  };

  // Reset add dialog
  const resetAddDialog = () => {
    setShowAddDialog(false);
    setNewDeviceName('');
    setNewDeviceType('windows');
    setRegisteredDevice(null);
    setCopiedToken(false);
  };

  // Stats
  const activeDevices = devices.filter(isDeviceActive).length;
  const totalSynced = devices.reduce((acc, d) => acc + d.bytes_synced, 0);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="screen-title">Sync devices</h1>
          <p className="text-muted-foreground">
            Manage devices connected to NithronSync
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={loadDevices} disabled={loading}>
            <RefreshIcon className={`h-4 w-4 mr-2 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
          <Button onClick={() => setShowAddDialog(true)}>
            <AddIcon className="h-4 w-4 mr-2" />
            Add device
          </Button>
        </div>
      </div>

      {/* Stats Cards */}
      <div className="grid border-l border-t border-hairline md:grid-cols-4 [&>*]:border-0 [&>*]:border-b [&>*]:border-r [&>*]:border-hairline">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total devices</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="pixel-display pixel-24">{devices.length}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Active now</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="pixel-display pixel-24">{activeDevices}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Offline</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="pixel-display pixel-24">{devices.length - activeDevices}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total synced</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="pixel-display pixel-24">{formatBytes(totalSynced)}</div>
          </CardContent>
        </Card>
      </div>

      {/* Error State */}
      {error && (
        <Card className="border-status-failed bg-status-failed-tint">
          <CardContent className="flex items-center gap-2 py-4">
            <FailedIcon className="h-5 w-5 text-status-failed-text" />
            <span className="text-status-failed-text">{error}</span>
          </CardContent>
        </Card>
      )}

      {/* Empty State */}
      {!loading && devices.length === 0 && (
        <Card className="border-dashed">
          <CardContent className="flex flex-col items-center justify-center py-12">
            <PhoneIcon className="h-12 w-12 text-muted-foreground mb-4" />
            <h3 className="text-lg font-[560] mb-2">No devices connected</h3>
            <p className="text-muted-foreground text-center mb-4 max-w-md">
              Connect your Windows, Linux, or mobile device to sync files with your nOS server.
            </p>
            <div className="flex gap-2">
              <Button onClick={() => setShowAddDialog(true)}>
                <AddIcon className="h-4 w-4 mr-2" />
                Add device
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Devices Grid */}
      {devices.length > 0 && (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {devices.map((device) => (
            <DeviceCard
              key={device.id}
              device={device}
              onEdit={(d) => {
                setEditDevice(d);
                setEditName(d.device_name);
              }}
              onRevoke={setRevokeDeviceTarget}
            />
          ))}
        </div>
      )}

      {/* Edit Dialog */}
      <Dialog open={!!editDevice} onOpenChange={() => setEditDevice(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Rename device</DialogTitle>
            <DialogDescription>
              Give this device a recognizable name.
            </DialogDescription>
          </DialogHeader>
          <div className="py-4">
            <Label htmlFor="device-name">Device name</Label>
            <Input
              id="device-name"
              value={editName}
              onChange={(e) => setEditName(e.target.value)}
              placeholder="My laptop"
              className="mt-2"
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditDevice(null)}>
              Cancel
            </Button>
            <Button onClick={handleEditSave} disabled={!editName.trim()}>
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Revoke Confirmation */}
      <AlertDialog open={!!revokeDeviceTarget} onOpenChange={() => setRevokeDeviceTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Revoke device access?</AlertDialogTitle>
            <AlertDialogDescription>
              This will immediately disconnect "{revokeDeviceTarget?.device_name}" and prevent it from syncing files.
              The device will need to be re-registered to sync again.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleRevoke} className={buttonVariants({ variant: 'destructive' })}>
              Revoke access
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Add Device Dialog */}
      <Dialog open={showAddDialog} onOpenChange={resetAddDialog}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Add sync device</DialogTitle>
            <DialogDescription>
              {!registeredDevice
                ? 'Register a new device to sync files with nOS.'
                : 'Copy the token below and enter it in your sync client.'}
            </DialogDescription>
          </DialogHeader>

          {!registeredDevice ? (
            <>
              <div className="space-y-4 py-4">
                <div>
                  <Label htmlFor="new-device-name">Device name</Label>
                  <Input
                    id="new-device-name"
                    value={newDeviceName}
                    onChange={(e) => setNewDeviceName(e.target.value)}
                    placeholder="e.g., Work Laptop, iPhone"
                    className="mt-2"
                  />
                </div>
                <div>
                  <Label>Device type</Label>
                  <div className="grid grid-cols-5 gap-2 mt-2">
                    {(['windows', 'linux', 'macos', 'android', 'ios'] as DeviceType[]).map((type) => (
                      <button
                        key={type}
                        onClick={() => setNewDeviceType(type)}
                        className={`flex flex-col items-center p-3 rounded-lg border-2 transition-colors ${
                          newDeviceType === type
                            ? 'border-ink '
                            : 'border-hairline border-hairline'
                        }`}
                      >
                        <DeviceIcon type={type} className="h-6 w-6 mb-1" />
                        <span className="text-xs">{formatDeviceType(type)}</span>
                      </button>
                    ))}
                  </div>
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={resetAddDialog}>
                  Cancel
                </Button>
                <Button onClick={handleRegister} disabled={!newDeviceName.trim()}>
                  Register device
                </Button>
              </DialogFooter>
            </>
          ) : (
            <>
              <div className="space-y-4 py-4">
                <div className="p-4 bg-status-ok-tint border border-status-ok rounded-lg">
                  <div className="flex items-center gap-2 text-status-ok-text mb-2">
                    <ChosenIcon className="h-5 w-5" />
                    <span className="font-medium">Device registered!</span>
                  </div>
                  <p className="text-sm text-status-ok-text">
                    {(newDeviceType === 'ios' || newDeviceType === 'android')
                      ? 'Scan the QR code with the NithronSync app, or copy the pairing link.'
                      : 'Copy the pairing link and paste it in NithronSync (or run nithron-sync-daemon --pair <link>).'}
                  </p>
                </div>

                {/* Show QR code for mobile devices */}
                {(newDeviceType === 'ios' || newDeviceType === 'android') && (
                  <div className="flex justify-center py-4">
                    <TokenQRCode link={registeredLink} size={180} />
                  </div>
                )}
                
                <div>
                  <Label>Pairing link</Label>
                  <div className="flex gap-2 mt-2">
                    <Input
                      value={registeredLink}
                      readOnly
                      className="font-mono text-xs"
                      data-testid="pairing-link"
                    />
                    <Button variant="outline" size="icon" onClick={copyToken} aria-label="Copy pairing link">
                      {copiedToken ? <ChosenIcon className="h-4 w-4" /> : <CopyIcon className="h-4 w-4" />}
                    </Button>
                  </div>
                  <p className="text-xs text-muted-foreground mt-1">
                    It holds this device's tokens and is shown only once: keep it private.
                    {registeredDevice.server_key_sha256 &&
                      " It also holds this server's key, so the client trusts this server's own certificate and no other."}
                  </p>
                </div>

                <div className="p-3 bg-status-warning-tint border border-status-warning rounded-lg">
                  <p className="text-sm text-status-warning-text">
                    The device token lasts 90 days; the client renews it with the refresh
                    token in the link.
                  </p>
                </div>
              </div>
              <DialogFooter>
                <Button onClick={resetAddDialog}>Done</Button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>

    </div>
  );
}

