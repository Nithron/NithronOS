import { useState } from 'react';
import { formatDate, formatDateTime, formatTime } from '@/lib/datetime'
import { confirmAction } from '@/components/ui/confirm'
import { useNavigate } from 'react-router-dom';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { AddIcon, ConnectionIcon, CopyIcon, DeleteIcon, DownloadIcon, FailedIcon, FirewallIcon, InternetIcon, KeyIcon, OkIcon, OpenExternalIcon, SettingsIcon, TimeIcon, WarningIcon, WorkingIcon } from '@/components/icons'
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
    if (!(await confirmAction({ title: 'Remove this WireGuard peer?', description: 'The device can no longer connect until it is added again.', action: 'Remove peer' }))) return;

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
          <CardTitle>Network status</CardTitle>
          <CardDescription>Current network configuration and access settings</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <Label className="text-muted-foreground">Access mode</Label>
              <div className="flex items-center gap-2 mt-1">
                {networkStatus.data?.access_mode === 'lan_only' && (
                  <>
                    <ConnectionIcon className="h-4 w-4" />
                    <span>LAN only</span>
                  </>
                )}
                {networkStatus.data?.access_mode === 'wireguard' && (
                  <>
                    <KeyIcon className="h-4 w-4" />
                    <span>WireGuard VPN</span>
                  </>
                )}
                {networkStatus.data?.access_mode === 'public_https' && (
                  <>
                    <InternetIcon className="h-4 w-4" />
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
              <Label className="text-muted-foreground">LAN access</Label>
              <div className="mt-1">
                {networkStatus.data?.lan_access ? (
                  <Badge variant="default">
                    <OkIcon className="h-3 w-3 mr-1" />
                    Enabled
                  </Badge>
                ) : (
                  <Badge variant="secondary">
                    <FailedIcon className="h-3 w-3 mr-1" />
                    Disabled
                  </Badge>
                )}
              </div>
            </div>

            <div>
              <Label className="text-muted-foreground">WAN blocked</Label>
              <div className="mt-1">
                {networkStatus.data?.wan_blocked ? (
                  <Badge variant="default">
                    <FirewallIcon className="h-3 w-3 mr-1" />
                    Yes
                  </Badge>
                ) : (
                  <Badge variant="secondary">
                    <WarningIcon className="h-3 w-3 mr-1" />
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
            <Label className="text-muted-foreground">Open ports</Label>
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
          <CardTitle>Quick actions</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-2">
          <Button
            variant="outline"
            onClick={() => navigate('/settings/network/wizard')}
          >
            <SettingsIcon className="h-4 w-4 mr-2" />
            Run setup wizard
          </Button>
          
          {networkStatus.data?.access_mode === 'public_https' && (
            <Button
              variant="outline"
              onClick={handleTestHTTPS}
              disabled={testHTTPS.isPending}
            >
              {testHTTPS.isPending ? (
                <WorkingIcon className="h-4 w-4 mr-2 animate-spin" />
              ) : (
                <OkIcon className="h-4 w-4 mr-2" />
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
          <TimeIcon className="h-4 w-4" />
          <AlertDescription className="flex items-center justify-between">
            <span>
              Firewall changes pending confirmation. 
              {firewallState.data.rollback_at && (
                <> Auto-rollback at {formatTime(firewallState.data.rollback_at)}</>
              )}
            </span>
            <div className="flex gap-2">
              <Button size="sm" onClick={handleConfirmFirewall}>
                Confirm
              </Button>
              <Button size="sm" variant="outline" onClick={handleRollbackFirewall}>
                Rollback now
              </Button>
            </div>
          </AlertDescription>
        </Alert>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Firewall status</CardTitle>
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
              <Label className="text-muted-foreground">Last applied</Label>
              <div className="mt-1 text-sm">
                {firewallState.data?.last_applied
                  ? formatDateTime(firewallState.data.last_applied)
                  : 'Never'}
              </div>
            </div>
          </div>

          <div>
            <Label className="text-muted-foreground mb-2">Active rules</Label>
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
                        <OkIcon className="h-4 w-4 text-status-ok-text" />
                      ) : (
                        <FailedIcon className="h-4 w-4 text-status-failed-text" />
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
              <WarningIcon className="h-4 w-4" />
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
                  <Label className="text-muted-foreground">Listen port</Label>
                  <div className="mt-1">{wgState.data.listen_port}</div>
                </div>
                <div>
                  <Label className="text-muted-foreground">Server subnet</Label>
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
                    <AddIcon className="h-4 w-4 mr-1" />
                    Add peer
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
                            Last seen: {formatDateTime(peer.last_handshake)}
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
                          <DeleteIcon className="h-4 w-4" />
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
          <CardTitle>HTTPS configuration</CardTitle>
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
                  <OpenExternalIcon className="h-4 w-4" />
                </Button>
              </div>
            </div>
          )}

          {httpsConfig.data?.expiry && (
            <div>
              <Label className="text-muted-foreground">Certificate expiry</Label>
              <div className="mt-1">
                {formatDate(httpsConfig.data.expiry)}
                {httpsConfig.data.next_renewal && (
                  <span className="text-sm text-muted-foreground ml-2">
                    (Renewal: {formatDate(httpsConfig.data.next_renewal)})
                  </span>
                )}
              </div>
            </div>
          )}

          {httpsConfig.data?.error_message && (
            <Alert variant="destructive">
              <WarningIcon className="h-4 w-4" />
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
        <h1 className="screen-title">Network & remote access</h1>
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
            <DialogTitle>Add WireGuard peer</DialogTitle>
            <DialogDescription>
              Create a new VPN client configuration
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label htmlFor="peer-name">Peer name</Label>
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
              {addPeer.isPending && <WorkingIcon className="h-4 w-4 mr-2 animate-spin" />}
              Add Peer
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Peer Config Dialog */}
      <Dialog open={showPeerConfigDialog} onOpenChange={setShowPeerConfigDialog}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>WireGuard client configuration</DialogTitle>
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
                <Label>Configuration file</Label>
                <pre className="mt-2 p-3 bg-muted rounded-lg text-xs overflow-x-auto">
                  {selectedPeerConfig.config}
                </pre>
                <div className="flex gap-2 mt-2">
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => copyToClipboard(selectedPeerConfig.config)}
                  >
                    <CopyIcon className="h-4 w-4 mr-1" />
                    Copy
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => downloadConfig(selectedPeerConfig.config, 'wireguard.conf')}
                  >
                    <DownloadIcon className="h-4 w-4 mr-1" />
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
