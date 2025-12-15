import { useState, useEffect } from 'react';
import {
  Cloud,
  FolderSync,
  Shield,
  Download,
  Upload,
  Settings,
  Info,
  ExternalLink,
  ChevronRight,
  Check,
  AlertTriangle,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Switch } from '@/components/ui/switch';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Separator } from '@/components/ui/separator';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { toast } from '@/components/ui/toast';
import {
  listSyncShares,
  getSyncConfig,
  updateSyncConfig,
  formatBytes,
  type SyncShare,
  type SyncConfig,
} from '@/api/sync';
import { Link } from 'react-router-dom';

export default function SettingsSync() {
  const [shares, setShares] = useState<SyncShare[]>([]);
  const [_config, setConfig] = useState<SyncConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  // Local form state
  const [selectedShares, setSelectedShares] = useState<string[]>([]);
  const [bandwidthLimit, setBandwidthLimit] = useState<number>(0);
  const [pauseSync, setPauseSync] = useState(false);

  // Load data
  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    try {
      setLoading(true);
      const [sharesRes, configRes] = await Promise.all([
        listSyncShares(),
        getSyncConfig(),
      ]);
      setShares(sharesRes.shares || []);
      setConfig(configRes);
      setSelectedShares(configRes.sync_shares || []);
      setBandwidthLimit(configRes.bandwidth_limit_kbps || 0);
      setPauseSync(configRes.pause_sync || false);
    } catch (err) {
      console.error('Failed to load sync settings:', err);
      toast.error('Failed to load sync settings');
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async () => {
    try {
      setSaving(true);
      await updateSyncConfig({
        sync_shares: selectedShares,
        bandwidth_limit_kbps: bandwidthLimit,
        pause_sync: pauseSync,
      });
      toast.success('Sync settings saved');
    } catch (err) {
      toast.error('Failed to save settings');
    } finally {
      setSaving(false);
    }
  };

  const toggleShare = (shareId: string) => {
    setSelectedShares((prev) =>
      prev.includes(shareId)
        ? prev.filter((id) => id !== shareId)
        : [...prev, shareId]
    );
  };

  const totalSize = shares
    .filter((s) => selectedShares.includes(s.share_id))
    .reduce((acc, s) => acc + s.total_size, 0);

  const totalFiles = shares
    .filter((s) => selectedShares.includes(s.share_id))
    .reduce((acc, s) => acc + s.file_count, 0);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight flex items-center gap-2">
            <Cloud className="h-6 w-6 text-blue-500" />
            NithronSync Settings
          </h1>
          <p className="text-muted-foreground">
            Configure file synchronization between your devices
          </p>
        </div>
        <div className="flex gap-2">
          <Link to="/settings/sync/devices">
            <Button variant="outline">
              Manage Devices
              <ChevronRight className="h-4 w-4 ml-1" />
            </Button>
          </Link>
          <Button onClick={handleSave} disabled={saving}>
            {saving ? 'Saving...' : 'Save Changes'}
          </Button>
        </div>
      </div>

      {/* Status Card */}
      <Card>
        <CardContent className="pt-6">
          <div className="grid gap-4 md:grid-cols-4">
            <div className="flex items-center gap-3">
              <div className="p-2 bg-blue-100 rounded-lg">
                <FolderSync className="h-5 w-5 text-blue-600" />
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Selected Shares</p>
                <p className="text-xl font-bold">{selectedShares.length}</p>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <div className="p-2 bg-green-100 rounded-lg">
                <Download className="h-5 w-5 text-green-600" />
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Total Files</p>
                <p className="text-xl font-bold">{totalFiles.toLocaleString()}</p>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <div className="p-2 bg-purple-100 rounded-lg">
                <Upload className="h-5 w-5 text-purple-600" />
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Total Size</p>
                <p className="text-xl font-bold">{formatBytes(totalSize)}</p>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <div className={`p-2 rounded-lg ${pauseSync ? 'bg-yellow-100' : 'bg-green-100'}`}>
                {pauseSync ? (
                  <AlertTriangle className="h-5 w-5 text-yellow-600" />
                ) : (
                  <Check className="h-5 w-5 text-green-600" />
                )}
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Status</p>
                <p className="text-xl font-bold">{pauseSync ? 'Paused' : 'Active'}</p>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <Tabs defaultValue="shares" className="space-y-4">
        <TabsList>
          <TabsTrigger value="shares">Shares</TabsTrigger>
          <TabsTrigger value="settings">Settings</TabsTrigger>
          <TabsTrigger value="downloads">Downloads</TabsTrigger>
        </TabsList>

        {/* Shares Tab */}
        <TabsContent value="shares" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Sync-Enabled Shares</CardTitle>
              <CardDescription>
                Select which shares should be available for sync on your devices
              </CardDescription>
            </CardHeader>
            <CardContent>
              {loading ? (
                <div className="py-8 text-center text-muted-foreground">
                  Loading shares...
                </div>
              ) : shares.length === 0 ? (
                <div className="py-8 text-center">
                  <FolderSync className="h-12 w-12 mx-auto text-muted-foreground mb-4" />
                  <h3 className="font-medium mb-2">No shares available</h3>
                  <p className="text-sm text-muted-foreground mb-4">
                    Create network shares to enable file synchronization
                  </p>
                  <Link to="/shares">
                    <Button variant="outline">Manage Shares</Button>
                  </Link>
                </div>
              ) : (
                <div className="space-y-3">
                  {shares.map((share) => (
                    <div
                      key={share.share_id}
                      className={`flex items-center justify-between p-4 rounded-lg border transition-colors cursor-pointer ${
                        selectedShares.includes(share.share_id)
                          ? 'border-blue-200 bg-blue-50'
                          : 'border-gray-200 hover:border-gray-300'
                      }`}
                      onClick={() => toggleShare(share.share_id)}
                    >
                      <div className="flex items-center gap-4">
                        <div className={`w-10 h-10 rounded-lg flex items-center justify-center ${
                          selectedShares.includes(share.share_id)
                            ? 'bg-blue-100 text-blue-600'
                            : 'bg-gray-100 text-gray-500'
                        }`}>
                          <FolderSync className="h-5 w-5" />
                        </div>
                        <div>
                          <h4 className="font-medium">{share.share_name}</h4>
                          <p className="text-sm text-muted-foreground">
                            {share.file_count.toLocaleString()} files • {formatBytes(share.total_size)}
                          </p>
                        </div>
                      </div>
                      <div className="flex items-center gap-3">
                        {share.sync_enabled && (
                          <Badge variant="secondary">Sync Enabled</Badge>
                        )}
                        <Switch
                          checked={selectedShares.includes(share.share_id)}
                          onCheckedChange={() => toggleShare(share.share_id)}
                        />
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        {/* Settings Tab */}
        <TabsContent value="settings" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Sync Control</CardTitle>
              <CardDescription>
                Control sync behavior across all your devices
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="flex items-center justify-between">
                <div className="space-y-0.5">
                  <Label className="text-base">Pause Sync</Label>
                  <p className="text-sm text-muted-foreground">
                    Temporarily stop all sync operations
                  </p>
                </div>
                <Switch
                  checked={pauseSync}
                  onCheckedChange={setPauseSync}
                />
              </div>
              
              <Separator />
              
              <div className="space-y-3">
                <Label className="text-base">Bandwidth Limit</Label>
                <p className="text-sm text-muted-foreground">
                  Limit upload/download speed (0 = unlimited)
                </p>
                <div className="flex items-center gap-2">
                  <Input
                    type="number"
                    value={bandwidthLimit}
                    onChange={(e) => setBandwidthLimit(parseInt(e.target.value) || 0)}
                    className="w-32"
                    min={0}
                  />
                  <span className="text-sm text-muted-foreground">KB/s</span>
                </div>
                {bandwidthLimit > 0 && (
                  <p className="text-xs text-muted-foreground">
                    ≈ {formatBytes(bandwidthLimit * 1024)}/s
                  </p>
                )}
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Security</CardTitle>
              <CardDescription>
                Sync security settings
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-center gap-3 p-4 bg-green-50 rounded-lg">
                <Shield className="h-5 w-5 text-green-600" />
                <div>
                  <p className="font-medium text-green-800">End-to-End Encryption</p>
                  <p className="text-sm text-green-600">
                    All sync traffic is encrypted using TLS 1.3
                  </p>
                </div>
              </div>
              
              <div className="flex items-center justify-between">
                <div className="space-y-0.5">
                  <Label className="text-base">Device Token Rotation</Label>
                  <p className="text-sm text-muted-foreground">
                    Tokens automatically rotate every 90 days
                  </p>
                </div>
                <Badge>Automatic</Badge>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        {/* Downloads Tab */}
        <TabsContent value="downloads" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Download NithronSync</CardTitle>
              <CardDescription>
                Get the sync client for your devices
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="grid gap-4 md:grid-cols-2">
                {/* Windows */}
                <div className="flex items-center gap-4 p-4 border rounded-lg">
                  <div className="p-3 bg-blue-100 rounded-lg">
                    <Settings className="h-6 w-6 text-blue-600" />
                  </div>
                  <div className="flex-1">
                    <h4 className="font-medium">Windows</h4>
                    <p className="text-sm text-muted-foreground">Windows 10/11</p>
                  </div>
                  <Button variant="outline" size="sm" disabled>
                    <Download className="h-4 w-4 mr-2" />
                    Coming Soon
                  </Button>
                </div>

                {/* Linux */}
                <div className="flex items-center gap-4 p-4 border rounded-lg">
                  <div className="p-3 bg-orange-100 rounded-lg">
                    <Settings className="h-6 w-6 text-orange-600" />
                  </div>
                  <div className="flex-1">
                    <h4 className="font-medium">Linux</h4>
                    <p className="text-sm text-muted-foreground">Debian/Ubuntu, Fedora</p>
                  </div>
                  <Button variant="outline" size="sm" disabled>
                    <Download className="h-4 w-4 mr-2" />
                    Coming Soon
                  </Button>
                </div>

                {/* macOS */}
                <div className="flex items-center gap-4 p-4 border rounded-lg">
                  <div className="p-3 bg-gray-100 rounded-lg">
                    <Settings className="h-6 w-6 text-gray-600" />
                  </div>
                  <div className="flex-1">
                    <h4 className="font-medium">macOS</h4>
                    <p className="text-sm text-muted-foreground">macOS 12+</p>
                  </div>
                  <Button variant="outline" size="sm" disabled>
                    <Download className="h-4 w-4 mr-2" />
                    Coming Soon
                  </Button>
                </div>

                {/* Mobile */}
                <div className="flex items-center gap-4 p-4 border rounded-lg">
                  <div className="p-3 bg-green-100 rounded-lg">
                    <Settings className="h-6 w-6 text-green-600" />
                  </div>
                  <div className="flex-1">
                    <h4 className="font-medium">Mobile</h4>
                    <p className="text-sm text-muted-foreground">iOS & Android</p>
                  </div>
                  <Button variant="outline" size="sm" disabled>
                    <Download className="h-4 w-4 mr-2" />
                    Coming Soon
                  </Button>
                </div>
              </div>

              <div className="mt-6 p-4 bg-blue-50 rounded-lg">
                <div className="flex items-start gap-3">
                  <Info className="h-5 w-5 text-blue-600 mt-0.5" />
                  <div>
                    <h4 className="font-medium text-blue-800">WebDAV Access</h4>
                    <p className="text-sm text-blue-700 mt-1">
                      You can also access your files using any WebDAV-compatible client.
                      Connect to: <code className="bg-blue-100 px-1 rounded">/dav/{"<share_id>"}</code>
                    </p>
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Documentation</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-3">
                <a
                  href="#"
                  className="flex items-center justify-between p-3 rounded-lg border hover:bg-gray-50 transition-colors"
                >
                  <div className="flex items-center gap-3">
                    <Info className="h-5 w-5 text-muted-foreground" />
                    <span>Getting Started Guide</span>
                  </div>
                  <ExternalLink className="h-4 w-4 text-muted-foreground" />
                </a>
                <a
                  href="#"
                  className="flex items-center justify-between p-3 rounded-lg border hover:bg-gray-50 transition-colors"
                >
                  <div className="flex items-center gap-3">
                    <Shield className="h-5 w-5 text-muted-foreground" />
                    <span>Security Best Practices</span>
                  </div>
                  <ExternalLink className="h-4 w-4 text-muted-foreground" />
                </a>
                <a
                  href="#"
                  className="flex items-center justify-between p-3 rounded-lg border hover:bg-gray-50 transition-colors"
                >
                  <div className="flex items-center gap-3">
                    <Settings className="h-5 w-5 text-muted-foreground" />
                    <span>API Documentation</span>
                  </div>
                  <ExternalLink className="h-4 w-4 text-muted-foreground" />
                </a>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}

