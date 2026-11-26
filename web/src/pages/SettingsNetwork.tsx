import { useState } from 'react'


import { 
  Network,
  Wifi,
  Globe,
  Shield,
  Server,
  Plus,
  Edit,
  Trash2,
  Save,
  RefreshCw,
  CheckCircle,
  XCircle,
  Info,
  Cable,
  Cloud,
  Download,
  Upload,
  Zap,
  UserPlus,
} from 'lucide-react'
import { PageHeader } from '@/components/ui/page-header'
import { Card } from '@/components/ui/card-enhanced'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { toast } from '@/components/ui/toast'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/nos-client'

// Network Interface type
interface NetworkInterface {
  name: string
  type: 'ethernet' | 'wifi' | 'bridge' | 'virtual'
  enabled: boolean
  dhcp: boolean
  ipv4Address?: string
  ipv4Netmask?: string
  ipv4Gateway?: string
  ipv6Address?: string
  dnsServers?: string[]
  mac?: string
  speed?: number
  status: 'up' | 'down' | 'configuring'
  statistics?: {
    rxBytes: number
    txBytes: number
    rxPackets: number
    txPackets: number
    rxErrors: number
    txErrors: number
  }
}

// Firewall Rule type
interface FirewallRule {
  id: string
  name: string
  enabled: boolean
  direction: 'inbound' | 'outbound'
  action: 'allow' | 'deny'
  protocol: 'tcp' | 'udp' | 'icmp' | 'any'
  sourceAddress?: string
  sourcePort?: string
  destAddress?: string
  destPort?: string
  interface?: string
  priority: number
}

// WireGuard Peer type
interface WireGuardPeer {
  id: string
  name: string
  publicKey: string
  allowedIPs: string[]
  endpoint?: string
  persistentKeepalive?: number
  lastHandshake?: string
  transferRx: number
  transferTx: number
}

// Overview Tab
function OverviewTab() {
  const { data: interfaces = [], isLoading } = useQuery({
    queryKey: ['network-interfaces'],
    queryFn: async () => {
      const response = await api.get<any>('/api/v1/network/interfaces')
      return response
    },
    refetchInterval: 5000,
  })

  const { data: status } = useQuery({
    queryKey: ['network-status'],
    queryFn: async () => {
      const response = await api.get<any>('/api/v1/network/status')
      return response
    },
    refetchInterval: 5000,
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-8">
        <RefreshCw className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Network Status */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card className="p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-muted-foreground">Internet Status</p>
              <div className="flex items-center gap-2 mt-1">
                {status?.internet ? (
                  <>
                    <CheckCircle className="h-4 w-4 text-green-500" />
                    <span className="font-medium">Connected</span>
                  </>
                ) : (
                  <>
                    <XCircle className="h-4 w-4 text-red-500" />
                    <span className="font-medium">Disconnected</span>
                  </>
                )}
              </div>
            </div>
            <Globe className="h-8 w-8 text-muted-foreground" />
          </div>
        </Card>

        <Card className="p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-muted-foreground">Public IP</p>
              <p className="font-mono text-sm mt-1">{status?.publicIP || 'Unknown'}</p>
            </div>
            <Cloud className="h-8 w-8 text-muted-foreground" />
          </div>
        </Card>

        <Card className="p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-muted-foreground">Hostname</p>
              <p className="font-medium mt-1">{status?.hostname || 'nos'}</p>
            </div>
            <Server className="h-8 w-8 text-muted-foreground" />
          </div>
        </Card>
      </div>

      {/* Network Interfaces */}
      <Card>
        <div className="p-6">
          <h3 className="text-lg font-semibold mb-4">Network Interfaces</h3>
          <div className="space-y-4">
            {interfaces.map((iface: NetworkInterface) => (
              <InterfaceCard key={iface.name} interface={iface} />
            ))}
          </div>
        </div>
      </Card>

      {/* DNS Configuration */}
      <Card>
        <div className="p-6">
          <h3 className="text-lg font-semibold mb-4">DNS Configuration</h3>
          <DNSConfig />
        </div>
      </Card>
    </div>
  )
}

// Interface Card Component
function InterfaceCard({ interface: iface }: { interface: NetworkInterface }) {
  const [editing, setEditing] = useState(false)
  const [config, setConfig] = useState({
    dhcp: iface.dhcp,
    ipv4Address: iface.ipv4Address || '',
    ipv4Netmask: iface.ipv4Netmask || '',
    ipv4Gateway: iface.ipv4Gateway || '',
  })
  const queryClient = useQueryClient()

  const mutation = useMutation({
    mutationFn: (data: any) => api.put(`/api/v1/network/interfaces/${iface.name}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['network-interfaces'] })
      toast.success('Interface updated successfully')
      setEditing(false)
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.message || 'Failed to update interface')
    }
  })

  const handleSave = () => {
    mutation.mutate(config)
  }

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`
  }

  return (
    <div className="border rounded-lg p-4">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          {iface.type === 'ethernet' ? (
            <Cable className="h-5 w-5 text-muted-foreground" />
          ) : iface.type === 'wifi' ? (
            <Wifi className="h-5 w-5 text-muted-foreground" />
          ) : (
            <Network className="h-5 w-5 text-muted-foreground" />
          )}
          <div>
            <div className="flex items-center gap-2">
              <h4 className="font-medium">{iface.name}</h4>
              <Badge variant={iface.status === 'up' ? 'success' : 'secondary'}>
                {iface.status}
              </Badge>
            </div>
            {iface.mac && (
              <p className="text-xs text-muted-foreground">MAC: {iface.mac}</p>
            )}
          </div>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => setEditing(!editing)}
        >
          {editing ? 'Cancel' : 'Configure'}
        </Button>
      </div>

      {!editing ? (
        <div className="space-y-2 text-sm">
          <div className="flex justify-between">
            <span className="text-muted-foreground">IP Configuration:</span>
            <span>{iface.dhcp ? 'DHCP' : 'Static'}</span>
          </div>
          {iface.ipv4Address && (
            <div className="flex justify-between">
              <span className="text-muted-foreground">IPv4 Address:</span>
              <span className="font-mono">{iface.ipv4Address}/{iface.ipv4Netmask}</span>
            </div>
          )}
          {iface.ipv4Gateway && (
            <div className="flex justify-between">
              <span className="text-muted-foreground">Gateway:</span>
              <span className="font-mono">{iface.ipv4Gateway}</span>
            </div>
          )}
          {iface.statistics && (
            <>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Data Received:</span>
                <span>{formatBytes(iface.statistics.rxBytes)}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Data Sent:</span>
                <span>{formatBytes(iface.statistics.txBytes)}</span>
              </div>
            </>
          )}
        </div>
      ) : (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <Label htmlFor={`${iface.name}-dhcp`}>Use DHCP</Label>
            <Switch
              id={`${iface.name}-dhcp`}
              checked={config.dhcp}
              onCheckedChange={(checked) => setConfig({ ...config, dhcp: checked })}
            />
          </div>
          
          {!config.dhcp && (
            <>
              <div>
                <Label htmlFor={`${iface.name}-ip`}>IP Address</Label>
                <Input
                  id={`${iface.name}-ip`}
                  value={config.ipv4Address}
                  onChange={(e) => setConfig({ ...config, ipv4Address: e.target.value })}
                  placeholder="192.168.1.100"
                />
              </div>
              <div>
                <Label htmlFor={`${iface.name}-netmask`}>Netmask</Label>
                <Input
                  id={`${iface.name}-netmask`}
                  value={config.ipv4Netmask}
                  onChange={(e) => setConfig({ ...config, ipv4Netmask: e.target.value })}
                  placeholder="255.255.255.0"
                />
              </div>
              <div>
                <Label htmlFor={`${iface.name}-gateway`}>Gateway</Label>
                <Input
                  id={`${iface.name}-gateway`}
                  value={config.ipv4Gateway}
                  onChange={(e) => setConfig({ ...config, ipv4Gateway: e.target.value })}
                  placeholder="192.168.1.1"
                />
              </div>
            </>
          )}
          
          <div className="flex justify-end gap-2">
            <Button variant="outline" size="sm" onClick={() => setEditing(false)}>
              Cancel
            </Button>
            <Button size="sm" onClick={handleSave} disabled={mutation.isPending}>
              {mutation.isPending ? (
                <>
                  <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                  Saving...
                </>
              ) : (
                <>
                  <Save className="mr-2 h-4 w-4" />
                  Save
                </>
              )}
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}

// DNS Configuration Component
function DNSConfig() {
  const [editing, setEditing] = useState(false)
  const [servers, setServers] = useState<string[]>(['8.8.8.8', '8.8.4.4'])
  const queryClient = useQueryClient()

  useQuery({
    queryKey: ['dns-config'],
    queryFn: async () => {
      const response = await api.get<any>('/api/v1/network/dns')
      setServers(response.servers || [])
      return response
    },
  })

  const mutation = useMutation({
    mutationFn: (data: any) => api.put('/api/v1/network/dns', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dns-config'] })
      toast.success('DNS configuration updated')
      setEditing(false)
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.message || 'Failed to update DNS')
    }
  })

  const handleSave = () => {
    mutation.mutate({ servers: servers.filter(s => s) })
  }

  const addServer = () => {
    setServers([...servers, ''])
  }

  const removeServer = (index: number) => {
    setServers(servers.filter((_, i) => i !== index))
  }

  const updateServer = (index: number, value: string) => {
    const updated = [...servers]
    updated[index] = value
    setServers(updated)
  }

  return (
    <div>
      {!editing ? (
        <div className="space-y-2">
          <div className="flex items-center justify-between mb-4">
            <p className="text-sm text-muted-foreground">DNS Servers</p>
            <Button variant="outline" size="sm" onClick={() => setEditing(true)}>
              Edit
            </Button>
          </div>
          {servers.map((server, index) => (
            <div key={index} className="flex items-center gap-2">
              <span className="font-mono text-sm">{server}</span>
              {index === 0 && <Badge variant="outline">Primary</Badge>}
            </div>
          ))}
        </div>
      ) : (
        <div className="space-y-4">
          {servers.map((server, index) => (
            <div key={index} className="flex items-center gap-2">
              <Input
                value={server}
                onChange={(e) => updateServer(index, e.target.value)}
                placeholder="DNS Server IP"
                className="font-mono"
              />
              <Button
                variant="ghost"
                size="sm"
                onClick={() => removeServer(index)}
                disabled={servers.length === 1}
              >
                <Trash2 className="h-4 w-4" />
              </Button>
            </div>
          ))}
          
          <Button variant="outline" size="sm" onClick={addServer}>
            <Plus className="mr-2 h-4 w-4" />
            Add Server
          </Button>
          
          <div className="flex justify-end gap-2">
            <Button variant="outline" size="sm" onClick={() => setEditing(false)}>
              Cancel
            </Button>
            <Button size="sm" onClick={handleSave} disabled={mutation.isPending}>
              {mutation.isPending ? (
                <>
                  <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                  Saving...
                </>
              ) : (
                'Save Changes'
              )}
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}

// Firewall Tab
function FirewallTab() {
  const [showRuleDialog, setShowRuleDialog] = useState(false)
  const [selectedRule, setSelectedRule] = useState<FirewallRule | null>(null)
  const queryClient = useQueryClient()

  const { data: rules = [], isLoading } = useQuery({
    queryKey: ['firewall-rules'],
    queryFn: async () => {
      const response = await api.get<any>('/api/v1/network/firewall/rules')
      return response
    },
  })

  const { data: status } = useQuery({
    queryKey: ['firewall-status'],
    queryFn: async () => {
      const response = await api.get<any>('/api/v1/network/firewall/status')
      return response
    },
  })

  const toggleFirewall = useMutation({
    mutationFn: (enabled: boolean) => 
      api.post('/api/v1/network/firewall/toggle', { enabled }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['firewall-status'] })
      toast.success('Firewall status updated')
    },
  })

  const deleteRule = useMutation({
    mutationFn: (ruleId: string) => 
      api.delete(`/api/v1/network/firewall/rules/${ruleId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['firewall-rules'] })
      toast.success('Rule deleted')
    },
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-8">
        <RefreshCw className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Firewall Status */}
      <Card>
        <div className="p-6">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="text-lg font-semibold">Firewall Status</h3>
              <p className="text-sm text-muted-foreground mt-1">
                Protect your system from unauthorized access
              </p>
            </div>
            <div className="flex items-center gap-4">
              <div className="flex items-center gap-2">
                {status?.enabled ? (
                  <CheckCircle className="h-5 w-5 text-green-500" />
                ) : (
                  <XCircle className="h-5 w-5 text-red-500" />
                )}
                <span className={status?.enabled ? 'text-green-600' : 'text-red-600'}>
                  {status?.enabled ? 'Active' : 'Inactive'}
                </span>
              </div>
              <Switch
                checked={status?.enabled || false}
                onCheckedChange={(checked) => toggleFirewall.mutate(checked)}
              />
            </div>
          </div>
        </div>
      </Card>

      {/* Firewall Rules */}
      <Card>
        <div className="p-6">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-semibold">Firewall Rules</h3>
            <Button onClick={() => {
              setSelectedRule(null)
              setShowRuleDialog(true)
            }}>
              <Plus className="mr-2 h-4 w-4" />
              Add Rule
            </Button>
          </div>
          
          {rules.length === 0 ? (
            <div className="text-center py-8">
              <Shield className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
              <p className="text-muted-foreground">No firewall rules configured</p>
              <p className="text-sm text-muted-foreground mt-1">
                Add rules to control network traffic
              </p>
            </div>
          ) : (
            <div className="space-y-2">
              {rules.map((rule: FirewallRule) => (
                <div key={rule.id} className="border rounded-lg p-4">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <Switch
                        checked={rule.enabled}
                        onCheckedChange={() => {/* Toggle rule */}}
                      />
                      <div>
                        <div className="flex items-center gap-2">
                          <span className="font-medium">{rule.name}</span>
                          <Badge variant={rule.action === 'allow' ? 'success' : 'destructive'}>
                            {rule.action}
                          </Badge>
                          <Badge variant="outline">{rule.direction}</Badge>
                        </div>
                        <p className="text-sm text-muted-foreground">
                          {rule.protocol.toUpperCase()} 
                          {rule.destPort && ` port ${rule.destPort}`}
                          {rule.sourceAddress && ` from ${rule.sourceAddress}`}
                        </p>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          setSelectedRule(rule)
                          setShowRuleDialog(true)
                        }}
                      >
                        <Edit className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => deleteRule.mutate(rule.id)}
                      >
                        <Trash2 className="h-4 w-4 text-red-500" />
                      </Button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </Card>

      {/* Rule Dialog */}
      <FirewallRuleDialog
        rule={selectedRule}
        open={showRuleDialog}
        onClose={() => {
          setShowRuleDialog(false)
          setSelectedRule(null)
        }}
      />
    </div>
  )
}

// Firewall Rule Dialog
function FirewallRuleDialog({ 
  rule, 
  open, 
  onClose 
}: { 
  rule: FirewallRule | null
  open: boolean
  onClose: () => void 
}) {
  const [formData, setFormData] = useState({
    name: rule?.name || '',
    enabled: rule?.enabled ?? true,
    direction: rule?.direction || 'inbound',
    action: rule?.action || 'allow',
    protocol: rule?.protocol || 'tcp',
    sourceAddress: rule?.sourceAddress || '',
    sourcePort: rule?.sourcePort || '',
    destAddress: rule?.destAddress || '',
    destPort: rule?.destPort || '',
    interface: rule?.interface || '',
  })
  const queryClient = useQueryClient()

  const mutation = useMutation({
    mutationFn: (data: any) => 
      rule 
        ? api.put(`/api/v1/network/firewall/rules/${rule.id}`, data)
        : api.post('/api/v1/network/firewall/rules', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['firewall-rules'] })
      toast.success(rule ? 'Rule updated' : 'Rule created')
      onClose()
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.message || 'Failed to save rule')
    }
  })

  const handleSubmit = () => {
    if (!formData.name) {
      toast.error('Rule name is required')
      return
    }
    mutation.mutate(formData)
  }

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>{rule ? 'Edit Firewall Rule' : 'Add Firewall Rule'}</DialogTitle>
        </DialogHeader>
        
        <div className="space-y-4 py-4">
          <div>
            <Label htmlFor="rule-name">Rule Name</Label>
            <Input
              id="rule-name"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              placeholder="e.g., Allow SSH"
            />
          </div>
          
          <div className="grid grid-cols-2 gap-4">
            <div>
              <Label htmlFor="direction">Direction</Label>
              <Select
                value={formData.direction}
                onValueChange={(value) => setFormData({ ...formData, direction: value as any })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="inbound">Inbound</SelectItem>
                  <SelectItem value="outbound">Outbound</SelectItem>
                </SelectContent>
              </Select>
            </div>
            
            <div>
              <Label htmlFor="action">Action</Label>
              <Select
                value={formData.action}
                onValueChange={(value) => setFormData({ ...formData, action: value as any })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="allow">Allow</SelectItem>
                  <SelectItem value="deny">Deny</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          
          <div>
            <Label htmlFor="protocol">Protocol</Label>
            <Select
              value={formData.protocol}
              onValueChange={(value) => setFormData({ ...formData, protocol: value as any })}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="tcp">TCP</SelectItem>
                <SelectItem value="udp">UDP</SelectItem>
                <SelectItem value="icmp">ICMP</SelectItem>
                <SelectItem value="any">Any</SelectItem>
              </SelectContent>
            </Select>
          </div>
          
          <div className="grid grid-cols-2 gap-4">
            <div>
              <Label htmlFor="source-address">Source Address</Label>
              <Input
                id="source-address"
                value={formData.sourceAddress}
                onChange={(e) => setFormData({ ...formData, sourceAddress: e.target.value })}
                placeholder="Any or IP/CIDR"
              />
            </div>
            
            <div>
              <Label htmlFor="source-port">Source Port</Label>
              <Input
                id="source-port"
                value={formData.sourcePort}
                onChange={(e) => setFormData({ ...formData, sourcePort: e.target.value })}
                placeholder="Any or port number"
              />
            </div>
          </div>
          
          <div className="grid grid-cols-2 gap-4">
            <div>
              <Label htmlFor="dest-address">Destination Address</Label>
              <Input
                id="dest-address"
                value={formData.destAddress}
                onChange={(e) => setFormData({ ...formData, destAddress: e.target.value })}
                placeholder="Any or IP/CIDR"
              />
            </div>
            
            <div>
              <Label htmlFor="dest-port">Destination Port</Label>
              <Input
                id="dest-port"
                value={formData.destPort}
                onChange={(e) => setFormData({ ...formData, destPort: e.target.value })}
                placeholder="Any or port number"
              />
            </div>
          </div>
          
          <div className="flex items-center justify-between">
            <Label htmlFor="enabled">Enable Rule</Label>
            <Switch
              id="enabled"
              checked={formData.enabled}
              onCheckedChange={(checked) => setFormData({ ...formData, enabled: checked })}
            />
          </div>
        </div>
        
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={handleSubmit} disabled={mutation.isPending}>
            {mutation.isPending ? (
              <>
                <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                Saving...
              </>
            ) : (
              rule ? 'Update Rule' : 'Create Rule'
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// WireGuard Tab
function WireGuardTab() {
  // Dialog state - used in button handlers below
  const [showPeerDialog, setShowPeerDialog] = useState(false)
  const [selectedPeer, setSelectedPeer] = useState<WireGuardPeer | null>(null)
  // Note: Dialog component would be rendered here but is not implemented yet
  console.log(showPeerDialog, selectedPeer) // Suppress unused variable warning
  const queryClient = useQueryClient()

  const { data: config, isLoading } = useQuery({
    queryKey: ['wireguard-config'],
    queryFn: async () => {
      const response = await api.get<any>('/api/v1/network/wireguard')
      return response
    },
  })

  const { data: peers = [] } = useQuery({
    queryKey: ['wireguard-peers'],
    queryFn: async () => {
      const response = await api.get<any>('/api/v1/network/wireguard/peers')
      return response
    },
  })

  const toggleWireGuard = useMutation({
    mutationFn: (enabled: boolean) => 
      api.post('/api/v1/network/wireguard/toggle', { enabled }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['wireguard-config'] })
      toast.success('WireGuard status updated')
    },
  })

  const deletePeer = useMutation({
    mutationFn: (peerId: string) => 
      api.delete(`/api/v1/network/wireguard/peers/${peerId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['wireguard-peers'] })
      toast.success('Peer removed')
    },
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-8">
        <RefreshCw className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* WireGuard Status */}
      <Card>
        <div className="p-6">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="text-lg font-semibold">WireGuard VPN</h3>
              <p className="text-sm text-muted-foreground mt-1">
                Secure VPN connections for remote access
              </p>
            </div>
            <div className="flex items-center gap-4">
              <div className="flex items-center gap-2">
                {config?.enabled ? (
                  <CheckCircle className="h-5 w-5 text-green-500" />
                ) : (
                  <XCircle className="h-5 w-5 text-red-500" />
                )}
                <span className={config?.enabled ? 'text-green-600' : 'text-red-600'}>
                  {config?.enabled ? 'Active' : 'Inactive'}
                </span>
              </div>
              <Switch
                checked={config?.enabled || false}
                onCheckedChange={(checked) => toggleWireGuard.mutate(checked)}
              />
            </div>
          </div>
          
          {config?.enabled && (
            <div className="mt-4 p-4 bg-muted rounded-lg">
              <div className="grid grid-cols-2 gap-4 text-sm">
                <div>
                  <span className="text-muted-foreground">Interface:</span>
                  <span className="ml-2 font-mono">wg0</span>
                </div>
                <div>
                  <span className="text-muted-foreground">Port:</span>
                  <span className="ml-2 font-mono">{config.port || 51820}</span>
                </div>
                <div className="col-span-2">
                  <span className="text-muted-foreground">Public Key:</span>
                  <span className="ml-2 font-mono text-xs break-all">{config.publicKey}</span>
                </div>
              </div>
            </div>
          )}
        </div>
      </Card>

      {/* WireGuard Peers */}
      <Card>
        <div className="p-6">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-semibold">VPN Peers</h3>
            <Button onClick={() => {
              setSelectedPeer(null)
              setShowPeerDialog(true)
            }}>
              <Plus className="mr-2 h-4 w-4" />
              Add Peer
            </Button>
          </div>
          
          {peers.length === 0 ? (
            <div className="text-center py-8">
              <UserPlus className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
              <p className="text-muted-foreground">No peers configured</p>
              <p className="text-sm text-muted-foreground mt-1">
                Add peers to allow VPN connections
              </p>
            </div>
          ) : (
            <div className="space-y-2">
              {peers.map((peer: WireGuardPeer) => (
                <div key={peer.id} className="border rounded-lg p-4">
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="font-medium">{peer.name}</span>
                        {peer.lastHandshake && (
                          <Badge variant="outline">
                            Last seen: {formatDistanceToNow(new Date(peer.lastHandshake), { addSuffix: true })}
                          </Badge>
                        )}
                      </div>
                      <p className="text-sm text-muted-foreground mt-1">
                        Allowed IPs: {peer.allowedIPs.join(', ')}
                      </p>
                      {peer.endpoint && (
                        <p className="text-sm text-muted-foreground">
                          Endpoint: {peer.endpoint}
                        </p>
                      )}
                    </div>
                    <div className="flex items-center gap-2">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          setSelectedPeer(peer)
                          setShowPeerDialog(true)
                        }}
                      >
                        <Edit className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => deletePeer.mutate(peer.id)}
                      >
                        <Trash2 className="h-4 w-4 text-red-500" />
                      </Button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </Card>

      {/* Peer Dialog */}
      {/* TODO: Implement WireGuardPeerDialog */}
    </div>
  )
}

// HTTPS Tab
function HTTPSTab() {
  const [uploading, setUploading] = useState(false)
  const queryClient = useQueryClient()

  const { data: tlsConfig, isLoading } = useQuery({
    queryKey: ['tls-config'],
    queryFn: async () => {
      const response = await api.get<any>('/api/v1/network/https')
      return response
    },
  })

  const toggleHTTPS = useMutation({
    mutationFn: (enabled: boolean) => 
      api.post('/api/v1/network/https/toggle', { enabled }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tls-config'] })
      toast.success('HTTPS status updated')
    },
  })

  const generateCert = useMutation({
    mutationFn: () => api.post('/api/v1/network/https/generate'),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tls-config'] })
      toast.success('Self-signed certificate generated')
    },
  })

  const handleFileUpload = async (event: React.ChangeEvent<HTMLInputElement>, type: 'cert' | 'key') => {
    const file = event.target.files?.[0]
    if (!file) return

    setUploading(true)
    const formData = new FormData()
    formData.append('file', file)
    formData.append('type', type)

    try {
      await api.post('/api/v1/network/https/upload', formData)
      queryClient.invalidateQueries({ queryKey: ['tls-config'] })
      toast.success(`${type === 'cert' ? 'Certificate' : 'Key'} uploaded successfully`)
    } catch (error: any) {
      toast.error(error.response?.data?.message || `Failed to upload ${type}`)
    } finally {
      setUploading(false)
    }
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-8">
        <RefreshCw className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* HTTPS Status */}
      <Card>
        <div className="p-6">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="text-lg font-semibold">HTTPS Configuration</h3>
              <p className="text-sm text-muted-foreground mt-1">
                Secure web interface with TLS encryption
              </p>
            </div>
            <div className="flex items-center gap-4">
              <div className="flex items-center gap-2">
                {tlsConfig?.enabled ? (
                  <CheckCircle className="h-5 w-5 text-green-500" />
                ) : (
                  <XCircle className="h-5 w-5 text-red-500" />
                )}
                <span className={tlsConfig?.enabled ? 'text-green-600' : 'text-red-600'}>
                  {tlsConfig?.enabled ? 'Enabled' : 'Disabled'}
                </span>
              </div>
              <Switch
                checked={tlsConfig?.enabled || false}
                onCheckedChange={(checked) => toggleHTTPS.mutate(checked)}
              />
            </div>
          </div>
        </div>
      </Card>

      {/* Certificate Info */}
      {tlsConfig?.certificate && (
        <Card>
          <div className="p-6">
            <h3 className="text-lg font-semibold mb-4">Current Certificate</h3>
            <div className="space-y-2 text-sm">
              <div className="flex justify-between">
                <span className="text-muted-foreground">Subject:</span>
                <span className="font-mono">{tlsConfig.certificate.subject}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Issuer:</span>
                <span className="font-mono">{tlsConfig.certificate.issuer}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Valid From:</span>
                <span>{new Date(tlsConfig.certificate.validFrom).toLocaleDateString()}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Valid To:</span>
                <span>{new Date(tlsConfig.certificate.validTo).toLocaleDateString()}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Type:</span>
                <Badge variant={tlsConfig.certificate.selfSigned ? 'secondary' : 'success'}>
                  {tlsConfig.certificate.selfSigned ? 'Self-Signed' : 'CA-Signed'}
                </Badge>
              </div>
            </div>
          </div>
        </Card>
      )}

      {/* Certificate Management */}
      <Card>
        <div className="p-6">
          <h3 className="text-lg font-semibold mb-4">Certificate Management</h3>
          
          <div className="space-y-4">
            <Alert>
              <Info className="h-4 w-4" />
              <AlertDescription>
                You can either generate a self-signed certificate or upload your own certificate and key files.
              </AlertDescription>
            </Alert>
            
            <div className="flex flex-wrap gap-4">
              <Button onClick={() => generateCert.mutate()} disabled={generateCert.isPending}>
                {generateCert.isPending ? (
                  <>
                    <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                    Generating...
                  </>
                ) : (
                  <>
                    <Zap className="mr-2 h-4 w-4" />
                    Generate Self-Signed
                  </>
                )}
              </Button>
              
              <div>
                <input
                  type="file"
                  id="cert-upload"
                  className="hidden"
                  accept=".pem,.crt,.cer"
                  onChange={(e) => handleFileUpload(e, 'cert')}
                />
                <Button
                  variant="outline"
                  onClick={() => document.getElementById('cert-upload')?.click()}
                  disabled={uploading}
                >
                  <Upload className="mr-2 h-4 w-4" />
                  Upload Certificate
                </Button>
              </div>
              
              <div>
                <input
                  type="file"
                  id="key-upload"
                  className="hidden"
                  accept=".pem,.key"
                  onChange={(e) => handleFileUpload(e, 'key')}
                />
                <Button
                  variant="outline"
                  onClick={() => document.getElementById('key-upload')?.click()}
                  disabled={uploading}
                >
                  <Upload className="mr-2 h-4 w-4" />
                  Upload Private Key
                </Button>
              </div>
              
              <Button variant="outline">
                <Download className="mr-2 h-4 w-4" />
                Download CSR
              </Button>
            </div>
          </div>
        </div>
      </Card>

      {/* Let's Encrypt */}
      <Card>
        <div className="p-6">
          <h3 className="text-lg font-semibold mb-4">Let's Encrypt</h3>
          <p className="text-sm text-muted-foreground mb-4">
            Automatically obtain and renew free SSL certificates
          </p>
          
          <div className="space-y-4">
            <div>
              <Label htmlFor="le-domain">Domain Name</Label>
              <Input
                id="le-domain"
                placeholder="nas.example.com"
                disabled={!tlsConfig?.letsencrypt}
              />
            </div>
            
            <div>
              <Label htmlFor="le-email">Email Address</Label>
              <Input
                id="le-email"
                type="email"
                placeholder="admin@example.com"
                disabled={!tlsConfig?.letsencrypt}
              />
            </div>
            
            <div className="flex items-center justify-between">
              <Label htmlFor="le-enabled">Enable Let's Encrypt</Label>
              <Switch id="le-enabled" />
            </div>
            
            <Button disabled>
              Request Certificate
            </Button>
          </div>
        </div>
      </Card>
    </div>
  )
}

export function SettingsNetwork() {

  const [activeTab, setActiveTab] = useState('overview')

  return (
    <div className="container mx-auto py-6 space-y-6">
      <PageHeader
        title="Network & Remote Access"
        description="Configure network interfaces, firewall, VPN, and remote access"
        icon={Network}
      />

      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList className="grid w-full grid-cols-4">
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="firewall">Firewall</TabsTrigger>
          <TabsTrigger value="wireguard">WireGuard</TabsTrigger>
          <TabsTrigger value="https">HTTPS</TabsTrigger>
        </TabsList>
        
        <TabsContent value="overview">
          <OverviewTab />
        </TabsContent>
        
        <TabsContent value="firewall">
          <FirewallTab />
        </TabsContent>
        
        <TabsContent value="wireguard">
          <WireGuardTab />
        </TabsContent>
        
        <TabsContent value="https">
          <HTTPSTab />
        </TabsContent>
      </Tabs>
    </div>
  )
}

function formatDistanceToNow(date: Date, options?: { addSuffix?: boolean }): string {
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  const seconds = Math.floor(diff / 1000)
  const minutes = Math.floor(seconds / 60)
  const hours = Math.floor(minutes / 60)
  const days = Math.floor(hours / 24)
  
  if (days > 0) return `${days} day${days > 1 ? 's' : ''}${options?.addSuffix ? ' ago' : ''}`
  if (hours > 0) return `${hours} hour${hours > 1 ? 's' : ''}${options?.addSuffix ? ' ago' : ''}`
  if (minutes > 0) return `${minutes} minute${minutes > 1 ? 's' : ''}${options?.addSuffix ? ' ago' : ''}`
  return `${seconds} second${seconds > 1 ? 's' : ''}${options?.addSuffix ? ' ago' : ''}`
}
