import { useState, useEffect, useRef } from 'react';
import QRCode from 'qrcode';
import {
  Smartphone,
  Monitor,
  Laptop,
  Terminal,
  Trash2,
  Edit2,
  RefreshCw,
  Plus,
  QrCode,
  Copy,
  Check,
  AlertCircle,
  Cloud,
  CloudOff,
  Download,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
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
  token, 
  serverUrl,
  deviceName,
  size = 200 
}: { 
  token: string; 
  serverUrl: string;
  deviceName: string;
  size?: number;
}) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!canvasRef.current || !token) return;

    // Create a sync URL that the mobile app can parse
    // Format: nithronos://sync?server=<url>&token=<token>&name=<name>
    const syncUrl = `nithronos://sync?server=${encodeURIComponent(serverUrl)}&token=${encodeURIComponent(token)}&name=${encodeURIComponent(deviceName)}`;

    QRCode.toCanvas(canvasRef.current, syncUrl, {
      width: size,
      margin: 2,
      color: {
        dark: '#1f2937',
        light: '#ffffff',
      },
      errorCorrectionLevel: 'M',
    }, (err) => {
      if (err) {
        console.error('QR code generation error:', err);
        setError('Failed to generate QR code');
      }
    });
  }, [token, serverUrl, deviceName, size]);

  if (error) {
    return (
      <div className="flex items-center justify-center p-4 bg-red-50 border border-red-200 rounded-lg">
        <AlertCircle className="h-5 w-5 text-red-500 mr-2" />
        <span className="text-sm text-red-700">{error}</span>
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center">
      <canvas 
        ref={canvasRef} 
        data-testid="qr-code"
        className="rounded-lg border border-gray-200"
      />
      <p className="text-xs text-muted-foreground mt-2 text-center max-w-[200px]">
        Scan with NithronSync app to connect this device
      </p>
    </div>
  );
}

// Server URL QR Code for initial setup
function ServerQRCode({ size = 200 }: { size?: number }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [error, setError] = useState<string | null>(null);
  const serverUrl = window.location.origin;

  useEffect(() => {
    if (!canvasRef.current) return;

    // Create a discovery URL for the mobile app
    const discoveryUrl = `nithronos://discover?server=${encodeURIComponent(serverUrl)}`;

    QRCode.toCanvas(canvasRef.current, discoveryUrl, {
      width: size,
      margin: 2,
      color: {
        dark: '#1f2937',
        light: '#ffffff',
      },
      errorCorrectionLevel: 'M',
    }, (err) => {
      if (err) {
        console.error('QR code generation error:', err);
        setError('Failed to generate QR code');
      }
    });
  }, [serverUrl, size]);

  if (error) {
    return (
      <div className="flex items-center justify-center p-4 bg-red-50 border border-red-200 rounded-lg">
        <AlertCircle className="h-5 w-5 text-red-500 mr-2" />
        <span className="text-sm text-red-700">{error}</span>
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center">
      <canvas 
        ref={canvasRef}
        data-testid="qr-code" 
        className="rounded-lg border border-gray-200"
      />
      <p className="text-xs text-muted-foreground mt-2 text-center">
        Server: {serverUrl}
      </p>
    </div>
  );
}

// Device icon component
function DeviceIcon({ type, className = 'h-5 w-5' }: { type: DeviceType; className?: string }) {
  switch (type) {
    case 'windows':
      return <Monitor className={className} />;
    case 'linux':
      return <Terminal className={className} />;
    case 'macos':
      return <Laptop className={className} />;
    case 'android':
    case 'ios':
      return <Smartphone className={className} />;
    default:
      return <Monitor className={className} />;
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
      {/* Status indicator */}
      <div
        className={`absolute top-0 left-0 w-1 h-full ${
          isActive ? 'bg-green-500' : 'bg-gray-400'
        }`}
      />
      
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-3">
            <div className={`p-2 rounded-lg ${isActive ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'}`}>
              <DeviceIcon type={device.device_type} className="h-6 w-6" />
            </div>
            <div>
              <CardTitle className="text-lg font-semibold">{device.device_name}</CardTitle>
              <CardDescription className="flex items-center gap-2">
                <span>{formatDeviceType(device.device_type)}</span>
                {device.os_version && (
                  <>
                    <span className="text-gray-300">•</span>
                    <span>{device.os_version}</span>
                  </>
                )}
              </CardDescription>
            </div>
          </div>
          <Badge variant={isActive ? 'default' : 'secondary'} className="shrink-0">
            {isActive ? 'Active' : 'Offline'}
          </Badge>
        </div>
      </CardHeader>
      
      <CardContent>
        <div className="grid grid-cols-2 gap-4 text-sm mb-4">
          <div>
            <span className="text-muted-foreground">Last Sync</span>
            <p className="font-medium">{formatRelativeTime(device.last_sync_at)}</p>
          </div>
          <div>
            <span className="text-muted-foreground">Last Seen</span>
            <p className="font-medium">{formatRelativeTime(device.last_seen_at)}</p>
          </div>
          <div>
            <span className="text-muted-foreground">Total Synced</span>
            <p className="font-medium">{formatBytes(device.bytes_synced)}</p>
          </div>
          <div>
            <span className="text-muted-foreground">Sync Count</span>
            <p className="font-medium">{device.sync_count.toLocaleString()}</p>
          </div>
        </div>
        
        {device.last_ip && (
          <div className="text-sm text-muted-foreground mb-4">
            Last IP: <code className="text-xs bg-gray-100 px-1 py-0.5 rounded">{device.last_ip}</code>
          </div>
        )}
        
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={() => onEdit(device)}>
            <Edit2 className="h-4 w-4 mr-1" />
            Rename
          </Button>
          <Button variant="destructive" size="sm" onClick={() => onRevoke(device)}>
            <Trash2 className="h-4 w-4 mr-1" />
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
  const [showQRDialog, setShowQRDialog] = useState(false);
  
  // New device form
  const [newDeviceName, setNewDeviceName] = useState('');
  const [newDeviceType, setNewDeviceType] = useState<DeviceType>('windows');
  const [registeredDevice, setRegisteredDevice] = useState<{
    device_id: string;
    device_token: string;
    refresh_token: string;
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
    } catch (err) {
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
    } catch (err) {
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

  // Copy token to clipboard
  const copyToken = () => {
    if (!registeredDevice) return;
    navigator.clipboard.writeText(registeredDevice.device_token);
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
          <h1 className="text-2xl font-bold tracking-tight">Sync Devices</h1>
          <p className="text-muted-foreground">
            Manage devices connected to NithronSync
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={loadDevices} disabled={loading}>
            <RefreshCw className={`h-4 w-4 mr-2 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
          <Button onClick={() => setShowAddDialog(true)}>
            <Plus className="h-4 w-4 mr-2" />
            Add Device
          </Button>
        </div>
      </div>

      {/* Stats Cards */}
      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Devices</CardTitle>
            <Smartphone className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{devices.length}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Active Now</CardTitle>
            <Cloud className="h-4 w-4 text-green-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600">{activeDevices}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Offline</CardTitle>
            <CloudOff className="h-4 w-4 text-gray-400" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-gray-500">{devices.length - activeDevices}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Synced</CardTitle>
            <Download className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{formatBytes(totalSynced)}</div>
          </CardContent>
        </Card>
      </div>

      {/* Error State */}
      {error && (
        <Card className="border-red-200 bg-red-50">
          <CardContent className="flex items-center gap-2 py-4">
            <AlertCircle className="h-5 w-5 text-red-500" />
            <span className="text-red-700">{error}</span>
          </CardContent>
        </Card>
      )}

      {/* Empty State */}
      {!loading && devices.length === 0 && (
        <Card className="border-dashed">
          <CardContent className="flex flex-col items-center justify-center py-12">
            <Smartphone className="h-12 w-12 text-muted-foreground mb-4" />
            <h3 className="text-lg font-semibold mb-2">No devices connected</h3>
            <p className="text-muted-foreground text-center mb-4 max-w-md">
              Connect your Windows, Linux, or mobile device to sync files with your NithronOS server.
            </p>
            <div className="flex gap-2">
              <Button onClick={() => setShowAddDialog(true)}>
                <Plus className="h-4 w-4 mr-2" />
                Add Device
              </Button>
              <Button variant="outline" onClick={() => setShowQRDialog(true)}>
                <QrCode className="h-4 w-4 mr-2" />
                Scan QR Code
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
            <DialogTitle>Rename Device</DialogTitle>
            <DialogDescription>
              Give this device a recognizable name.
            </DialogDescription>
          </DialogHeader>
          <div className="py-4">
            <Label htmlFor="device-name">Device Name</Label>
            <Input
              id="device-name"
              value={editName}
              onChange={(e) => setEditName(e.target.value)}
              placeholder="My Laptop"
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
            <AlertDialogTitle>Revoke Device Access?</AlertDialogTitle>
            <AlertDialogDescription>
              This will immediately disconnect "{revokeDeviceTarget?.device_name}" and prevent it from syncing files.
              The device will need to be re-registered to sync again.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleRevoke} className="bg-red-600 hover:bg-red-700">
              Revoke Access
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Add Device Dialog */}
      <Dialog open={showAddDialog} onOpenChange={resetAddDialog}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Add Sync Device</DialogTitle>
            <DialogDescription>
              {!registeredDevice
                ? 'Register a new device to sync files with NithronOS.'
                : 'Copy the token below and enter it in your sync client.'}
            </DialogDescription>
          </DialogHeader>

          {!registeredDevice ? (
            <>
              <div className="space-y-4 py-4">
                <div>
                  <Label htmlFor="new-device-name">Device Name</Label>
                  <Input
                    id="new-device-name"
                    value={newDeviceName}
                    onChange={(e) => setNewDeviceName(e.target.value)}
                    placeholder="e.g., Work Laptop, iPhone"
                    className="mt-2"
                  />
                </div>
                <div>
                  <Label>Device Type</Label>
                  <div className="grid grid-cols-5 gap-2 mt-2">
                    {(['windows', 'linux', 'macos', 'android', 'ios'] as DeviceType[]).map((type) => (
                      <button
                        key={type}
                        onClick={() => setNewDeviceType(type)}
                        className={`flex flex-col items-center p-3 rounded-lg border-2 transition-colors ${
                          newDeviceType === type
                            ? 'border-blue-500 bg-blue-50'
                            : 'border-gray-200 hover:border-gray-300'
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
                  Register Device
                </Button>
              </DialogFooter>
            </>
          ) : (
            <>
              <div className="space-y-4 py-4">
                <div className="p-4 bg-green-50 border border-green-200 rounded-lg">
                  <div className="flex items-center gap-2 text-green-700 mb-2">
                    <Check className="h-5 w-5" />
                    <span className="font-medium">Device Registered!</span>
                  </div>
                  <p className="text-sm text-green-600">
                    {(newDeviceType === 'ios' || newDeviceType === 'android')
                      ? 'Scan the QR code with the NithronSync app, or copy the token manually.'
                      : 'Copy the token below and paste it in your NithronSync client.'}
                  </p>
                </div>

                {/* Show QR code for mobile devices */}
                {(newDeviceType === 'ios' || newDeviceType === 'android') && (
                  <div className="flex justify-center py-4">
                    <TokenQRCode
                      token={registeredDevice.device_token}
                      serverUrl={window.location.origin}
                      deviceName={newDeviceName}
                      size={180}
                    />
                  </div>
                )}
                
                <div>
                  <Label>Device Token</Label>
                  <div className="flex gap-2 mt-2">
                    <Input
                      value={registeredDevice.device_token}
                      readOnly
                      className="font-mono text-xs"
                    />
                    <Button variant="outline" size="icon" onClick={copyToken}>
                      {copiedToken ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                    </Button>
                  </div>
                  <p className="text-xs text-muted-foreground mt-1">
                    This token will only be shown once. Store it securely.
                  </p>
                </div>

                <div className="p-3 bg-yellow-50 border border-yellow-200 rounded-lg">
                  <p className="text-sm text-yellow-700">
                    <strong>Important:</strong> The device token expires in 90 days.
                    The client will automatically refresh it before expiration.
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

      {/* QR Code Dialog for mobile app discovery */}
      <Dialog open={showQRDialog} onOpenChange={setShowQRDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Connect with QR Code</DialogTitle>
            <DialogDescription>
              Scan this QR code with the NithronSync mobile app to connect to this server.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col items-center py-6">
            <ServerQRCode size={220} />
            
            <div className="mt-6 w-full">
              <div className="relative">
                <div className="absolute inset-0 flex items-center">
                  <span className="w-full border-t" />
                </div>
                <div className="relative flex justify-center text-xs uppercase">
                  <span className="bg-white px-2 text-muted-foreground">Or enter manually</span>
                </div>
              </div>
              
              <div className="mt-4">
                <Label>Server URL</Label>
                <div className="flex gap-2 mt-2">
                  <Input
                    value={window.location.origin}
                    readOnly
                    className="font-mono text-sm"
                  />
                  <Button 
                    variant="outline" 
                    size="icon"
                    onClick={() => {
                      navigator.clipboard.writeText(window.location.origin);
                      toast.success('Server URL copied!');
                    }}
                  >
                    <Copy className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            </div>
            
            <div className="mt-6 p-4 bg-blue-50 border border-blue-200 rounded-lg w-full">
              <h4 className="font-medium text-blue-900 mb-2">Get the NithronSync App</h4>
              <div className="flex gap-3">
                <a 
                  href="#" 
                  className="flex-1 flex items-center justify-center gap-2 p-2 bg-black text-white rounded-lg text-sm hover:bg-gray-800 transition-colors"
                >
                  <Smartphone className="h-4 w-4" />
                  App Store
                </a>
                <a 
                  href="#" 
                  className="flex-1 flex items-center justify-center gap-2 p-2 bg-black text-white rounded-lg text-sm hover:bg-gray-800 transition-colors"
                >
                  <Smartphone className="h-4 w-4" />
                  Play Store
                </a>
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button onClick={() => setShowQRDialog(false)}>Close</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

