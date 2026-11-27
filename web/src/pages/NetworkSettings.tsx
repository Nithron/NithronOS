import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Globe, Shield, Wifi, Key, AlertTriangle,
  CheckCircle, XCircle, Settings, Plus, Copy, Download,
  Trash2, Loader2, ExternalLink, Clock
} from 'lucide-react';
import { format } from 'date-fns';
import {
  useNetworkStatus,
  useFirewallState,
  useWireGuardState,
  useHTTPSConfig,
  useConfirmFirewall,
  useRollbackFirewall,
  useAddWireGuardPeer,
  useRemoveWireGuardPeer,
  useTestHTTPS,
} from '@/api/net';
import type { WireGuardPeer, WireGuardPeerConfig } from '@/api/net.types';

export function NetworkSettings() {
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState('overview');
  const [showAddPeerDialog, setShowAddPeerDialog] = useState(false);
  const [showPeerConfigDialog, setShowPeerConfigDialog] = useState(false);
  const [selectedPeerConfig, setSelectedPeerConfig] = useState<WireGuardPeerConfig | null>(null);
  const [newPeerName, setNewPeerName] = useState('');

  // API hooks
  const networkStatus = useNetworkStatus();
  const firewallState = useFirewallState();
  const wgState = useWireGuardState();
  const httpsConfig = useHTTPSConfig();
  const confirmFirewall = useConfirmFirewall();
  const rollbackFirewall = useRollbackFirewall();
  const addPeer = useAddWireGuardPeer();
  const removePeer = useRemoveWireGuardPeer();
  const testHTTPS = useTestHTTPS();

  const handleConfirmFirewall = async () => {
    try {
      await confirmFirewall.mutateAsync();
    } catch (error) {
      console.error('Failed to confirm firewall:', error);
    }
  };

  const handleRollbackFirewall = async () => {
    try {
      await rollbackFirewall.mutateAsync();
    } catch (error) {
      console.error('Failed to rollback firewall:', error);
    }
  };

  const handleAddPeer = async () => {
    if (!newPeerName) return;

    try {
      const config = await addPeer.mutateAsync({ name: newPeerName });
      setSelectedPeerConfig(config);
      setShowAddPeerDialog(false);
      setShowPeerConfigDialog(true);
      setNewPeerName('');
    } catch (error) {
      console.error('Failed to add peer:', error);
    }
  };

  const handleRemovePeer = async (peerId: string) => {
    if (!confirm('Are you sure you want to remove this peer?')) return;

    try {
      await removePeer.mutateAsync(peerId);
    } catch (error) {
      console.error('Failed to remove peer:', error);
    }
  };

  const handleTestHTTPS = async () => {
    try {
      const result = await testHTTPS.mutateAsync();
      alert(result.message);
    } catch (error) {
      console.error('HTTPS test failed:', error);
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
  };

  const downloadConfig = (config: string, filename: string) => {
    const blob = new Blob([config], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    a.click();
    URL.revokeObjectURL(url);
  };

  const renderOverview = () => (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Network Status</CardTitle>
          <CardDescription>Current network configuration and access settings</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <Label className="text-muted-foreground">Access Mode</Label>
              <div className="flex items-center gap-2 mt-1">
                {networkStatus.data?.access_mode === 'lan_only' && (
                  <>
                    <Wifi className="h-4 w-4" />
                    <span>LAN Only</span>
                  </>
                )}
                {networkStatus.data?.access_mode === 'wireguard' && (
                  <>
                    <Key className="h-4 w-4" />
                    <span>WireGuard VPN</span>
                  </>
                )}
                {networkStatus.data?.access_mode === 'public_https' && (
                  <>
                    <Globe className="h-4 w-4" />
                    <span>Public HTTPS</span>
                  </>
                )}
              </div>
            </div>

            <div>
              <Label className="text-muted-foreground">External IP</Label>
              <div className="mt-1 font-mono text-sm">
                {networkStatus.data?.external_ip || 'Unknown'}
              </div>
            </div>

            <div>
              <Label className="text-muted-foreground">LAN Access</Label>
              <div className="mt-1">
                {networkStatus.data?.lan_access ? (
                  <Badge variant="default">
                    <CheckCircle className="h-3 w-3 mr-1" />
                    Enabled
                  </Badge>
                ) : (
                  <Badge variant="secondary">
                    <XCircle className="h-3 w-3 mr-1" />
                    Disabled
                  </Badge>
                )}
              </div>
            </div>

            <div>
              <Label className="text-muted-foreground">WAN Blocked</Label>
              <div className="mt-1">
                {networkStatus.data?.wan_blocked ? (
                  <Badge variant="default">
                    <Shield className="h-3 w-3 mr-1" />
                    Yes
                  </Badge>
                ) : (
                  <Badge variant="secondary">
                    <AlertTriangle className="h-3 w-3 mr-1" />
                    No
                  </Badge>
                )}
              </div>
            </div>
          </div>

          <div>
            <Label className="text-muted-foreground">Internal IPs</Label>
            <div className="mt-1 flex flex-wrap gap-2">
              {networkStatus.data?.internal_ips?.map((ip) => (
                <Badge key={ip} variant="outline">
                  {ip}
                </Badge>
              ))}
            </div>
          </div>

          <div>
            <Label className="text-muted-foreground">Open Ports</Label>
            <div className="mt-1 flex flex-wrap gap-2">
              {networkStatus.data?.open_ports?.map((port) => (
                <Badge key={port} variant="outline">
                  {port}
                </Badge>
              ))}
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Quick Actions</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-2">
          <Button
            variant="outline"
            onClick={() => navigate('/settings/network/wizard')}
          >
            <Settings className="h-4 w-4 mr-2" />
            Run Setup Wizard
          </Button>
          
          {networkStatus.data?.access_mode === 'public_https' && (
            <Button
              variant="outline"
              onClick={handleTestHTTPS}
              disabled={testHTTPS.isPending}
            >
              {testHTTPS.isPending ? (
                <Loader2 className="h-4 w-4 mr-2 animate-spin" />
              ) : (
                <CheckCircle className="h-4 w-4 mr-2" />
              )}
              Test HTTPS
            </Button>
          )}
        </CardContent>
      </Card>
    </div>
  );

  const renderFirewall = () => (
    <div className="space-y-6">
      {firewallState.data?.status === 'pending_confirm' && (
        <Alert variant="destructive">
          <Clock className="h-4 w-4" />
          <AlertDescription className="flex items-center justify-between">
            <span>
              Firewall changes pending confirmation. 
              {firewallState.data.rollback_at && (
                <> Auto-rollback at {format(new Date(firewallState.data.rollback_at), 'HH:mm:ss')}</>
              )}
            </span>
            <div className="flex gap-2">
              <Button size="sm" onClick={handleConfirmFirewall}>
                Confirm
              </Button>
              <Button size="sm" variant="outline" onClick={handleRollbackFirewall}>
                Rollback Now
              </Button>
            </div>
          </AlertDescription>
        </Alert>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Firewall Status</CardTitle>
          <CardDescription>
            Current firewall rules and configuration
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <Label className="text-muted-foreground">Status</Label>
              <div className="mt-1">
                <Badge variant={firewallState.data?.status === 'active' ? 'default' : 'secondary'}>
                  {firewallState.data?.status || 'Unknown'}
                </Badge>
              </div>
            </div>
            <div>
              <Label className="text-muted-foreground">Last Applied</Label>
              <div className="mt-1 text-sm">
                {firewallState.data?.last_applied
                  ? format(new Date(firewallState.data.last_applied), 'PPp')
                  : 'Never'}
              </div>
            </div>
          </div>

          <div>
            <Label className="text-muted-foreground mb-2">Active Rules</Label>
            <div className="space-y-1">
              {firewallState.data?.rules
                ?.filter((rule) => rule.enabled)
                .map((rule) => (
                  <div
                    key={rule.id}
                    className="flex items-center justify-between p-2 rounded-lg bg-muted/50"
                  >
                    <div className="flex items-center gap-2">
                      {rule.type === 'allow' ? (
                        <CheckCircle className="h-4 w-4 text-green-600" />
                      ) : (
                        <XCircle className="h-4 w-4 text-red-600" />
                      )}
                      <span className="text-sm">{rule.description}</span>
                    </div>
                    <div className="flex items-center gap-2 text-xs text-muted-foreground">
                      {rule.protocol && <Badge variant="outline">{rule.protocol}</Badge>}
                      {rule.dest_port && <Badge variant="outline">:{rule.dest_port}</Badge>}
                      {rule.source_cidr && <Badge variant="outline">{rule.source_cidr}</Badge>}
                    </div>
                  </div>
                ))}
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );

  const renderWireGuard = () => (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>WireGuard VPN</CardTitle>
          <CardDescription>
            Manage VPN server and client configurations
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {!wgState.data?.enabled ? (
            <Alert>
              <AlertTriangle className="h-4 w-4" />
              <AlertDescription>
                WireGuard is not enabled. Run the Remote Access Wizard to set it up.
              </AlertDescription>
            </Alert>
          ) : (
            <>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <Label className="text-muted-foreground">Interface</Label>
                  <div className="mt-1">{wgState.data.interface}</div>
                </div>
                <div>
                  <Label className="text-muted-foreground">Listen Port</Label>
                  <div className="mt-1">{wgState.data.listen_port}</div>
                </div>
                <div>
                  <Label className="text-muted-foreground">Server Subnet</Label>
                  <div className="mt-1 font-mono text-sm">{wgState.data.server_cidr}</div>
                </div>
                <div>
                  <Label className="text-muted-foreground">Endpoint</Label>
                  <div className="mt-1 font-mono text-sm">
                    {wgState.data.endpoint_hostname}:{wgState.data.listen_port}
                  </div>
                </div>
              </div>

              <div>
                <div className="flex items-center justify-between mb-2">
                  <Label>Peers ({wgState.data.peers?.length || 0})</Label>
                  <Button size="sm" onClick={() => setShowAddPeerDialog(true)}>
                    <Plus className="h-4 w-4 mr-1" />
                    Add Peer
                  </Button>
                </div>
                <div className="space-y-2">
                  {wgState.data.peers?.map((peer: WireGuardPeer) => (
                    <div
                      key={peer.id}
                      className="flex items-center justify-between p-3 rounded-lg border"
                    >
                      <div className="flex-1">
                        <div className="font-medium">{peer.name}</div>
                        <div className="text-sm text-muted-foreground">
                          {peer.allowed_ips.join(', ')}
                        </div>
                        {peer.last_handshake && (
                          <div className="text-xs text-muted-foreground mt-1">
                            Last seen: {format(new Date(peer.last_handshake), 'PPp')}
                          </div>
                        )}
                      </div>
                      <div className="flex items-center gap-2">
                        <Badge variant={peer.enabled ? 'default' : 'secondary'}>
                          {peer.enabled ? 'Active' : 'Disabled'}
                        </Badge>
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => handleRemovePeer(peer.id)}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );

  const renderHTTPS = () => (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>HTTPS Configuration</CardTitle>
          <CardDescription>
            SSL/TLS certificate and domain settings
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <Label className="text-muted-foreground">Mode</Label>
              <div className="mt-1">
                <Badge>
                  {httpsConfig.data?.mode === 'self_signed' && 'Self-Signed'}
                  {httpsConfig.data?.mode === 'http_01' && 'Let\'s Encrypt (HTTP-01)'}
                </Badge>
              </div>
            </div>
            <div>
              <Label className="text-muted-foreground">Status</Label>
              <div className="mt-1">
                <Badge
                  variant={
                    httpsConfig.data?.status === 'active'
                      ? 'default'
                      : httpsConfig.data?.status === 'renewing'
                      ? 'secondary'
                      : 'outline'
                  }
                >
                  {httpsConfig.data?.status || 'Unknown'}
                </Badge>
              </div>
            </div>
          </div>

          {httpsConfig.data?.domain && (
            <div>
              <Label className="text-muted-foreground">Domain</Label>
              <div className="mt-1 flex items-center gap-2">
                <span className="font-mono">{httpsConfig.data.domain}</span>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => window.open(`https://${httpsConfig.data.domain}`, '_blank')}
                >
                  <ExternalLink className="h-4 w-4" />
                </Button>
              </div>
            </div>
          )}

          {httpsConfig.data?.expiry && (
            <div>
              <Label className="text-muted-foreground">Certificate Expiry</Label>
              <div className="mt-1">
                {format(new Date(httpsConfig.data.expiry), 'PPP')}
                {httpsConfig.data.next_renewal && (
                  <span className="text-sm text-muted-foreground ml-2">
                    (Renewal: {format(new Date(httpsConfig.data.next_renewal), 'PPP')})
                  </span>
                )}
              </div>
            </div>
          )}

          {httpsConfig.data?.error_message && (
            <Alert variant="destructive">
              <AlertTriangle className="h-4 w-4" />
              <AlertDescription>{httpsConfig.data.error_message}</AlertDescription>
            </Alert>
          )}
        </CardContent>
      </Card>
    </div>
  );

  return (
    <div className="container mx-auto p-6">
      <div className="mb-6">
        <h1 className="text-2xl font-bold">Network & Remote Access</h1>
        <p className="text-muted-foreground">
          Configure network settings, firewall rules, and remote access options
        </p>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-4">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="firewall">Firewall</TabsTrigger>
          <TabsTrigger value="wireguard">WireGuard</TabsTrigger>
          <TabsTrigger value="https">HTTPS/TLS</TabsTrigger>
        </TabsList>

        <TabsContent value="overview">{renderOverview()}</TabsContent>
        <TabsContent value="firewall">{renderFirewall()}</TabsContent>
        <TabsContent value="wireguard">{renderWireGuard()}</TabsContent>
        <TabsContent value="https">{renderHTTPS()}</TabsContent>
      </Tabs>

      {/* Add Peer Dialog */}
      <Dialog open={showAddPeerDialog} onOpenChange={setShowAddPeerDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Add WireGuard Peer</DialogTitle>
            <DialogDescription>
              Create a new VPN client configuration
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label htmlFor="peer-name">Peer Name</Label>
              <Input
                id="peer-name"
                value={newPeerName}
                onChange={(e) => setNewPeerName(e.target.value)}
                placeholder="e.g., John's Laptop"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowAddPeerDialog(false)}>
              Cancel
            </Button>
            <Button onClick={handleAddPeer} disabled={!newPeerName || addPeer.isPending}>
              {addPeer.isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
              Add Peer
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Peer Config Dialog */}
      <Dialog open={showPeerConfigDialog} onOpenChange={setShowPeerConfigDialog}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>WireGuard Client Configuration</DialogTitle>
            <DialogDescription>
              Save this configuration or scan the QR code with your WireGuard app
            </DialogDescription>
          </DialogHeader>
          {selectedPeerConfig && (
            <div className="space-y-4">
              <div className="flex justify-center">
                <img src={selectedPeerConfig.qr_code} alt="WireGuard QR Code" className="w-64 h-64" />
              </div>
              <div>
                <Label>Configuration File</Label>
                <pre className="mt-2 p-3 bg-muted rounded-lg text-xs overflow-x-auto">
                  {selectedPeerConfig.config}
                </pre>
                <div className="flex gap-2 mt-2">
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => copyToClipboard(selectedPeerConfig.config)}
                  >
                    <Copy className="h-4 w-4 mr-1" />
                    Copy
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => downloadConfig(selectedPeerConfig.config, 'wireguard.conf')}
                  >
                    <Download className="h-4 w-4 mr-1" />
                    Download
                  </Button>
                </div>
              </div>
            </div>
          )}
          <DialogFooter>
            <Button onClick={() => setShowPeerConfigDialog(false)}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
