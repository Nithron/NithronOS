import { useState } from 'react'
import { motion } from 'framer-motion'
import { useNavigate } from 'react-router-dom'
import { 
  Settings as SettingsIcon,
  Network,
  Users,
  Calendar,
  Download,
  Info,
  ChevronRight,
  Save,
  Bell,
  Moon,
  Sun,
  Monitor,
  Shield,
  Palette,
  Terminal,
  Lock,
  RefreshCw,
  AlertCircle,
} from 'lucide-react'
import { PageHeader } from '@/components/ui/page-header'
import { Card } from '@/components/ui/card-enhanced'
import { Button } from '@/components/ui/button'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { cn } from '@/lib/utils'
import { toast } from '@/components/ui/toast'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/nos-client'

// Settings sections
const settingsSections = [
  { id: 'general', label: 'General', icon: SettingsIcon },
  { id: 'appearance', label: 'Appearance', icon: Palette },
  { id: 'notifications', label: 'Notifications', icon: Bell },
  { id: 'privacy', label: 'Privacy & Security', icon: Shield },
  { id: 'advanced', label: 'Advanced', icon: Terminal },
  { id: 'about', label: 'About', icon: Info },
]

// Quick links to other settings pages
const quickLinks = [
  { label: 'Network Settings', path: '/settings/network', icon: Network, description: 'Configure network and connectivity' },
  { label: 'User Management', path: '/settings/users', icon: Users, description: 'Manage user accounts and permissions' },
  { label: 'Schedules', path: '/settings/schedules', icon: Calendar, description: 'Configure automated tasks' },
  { label: 'System Updates', path: '/settings/updates', icon: Download, description: 'Manage system updates' },
  { label: '2FA Settings', path: '/settings/2fa', icon: Lock, description: 'Two-factor authentication' },
]

// General Settings Component
function GeneralSettings() {
  const queryClient = useQueryClient()
  const [settings, setSettings] = useState({
    hostname: '',
    timezone: '',
    language: 'en-US',
    dateFormat: 'MM/DD/YYYY',
    timeFormat: '12h',
    temperatureUnit: 'celsius',
    firstDayOfWeek: 'sunday',
  })

  // Fetch system settings
  const { data: systemConfig } = useQuery({
    queryKey: ['system-config'],
    queryFn: async () => {
      const response = await api.get<any>('/api/v1/system/config')
      return response
    }
  })
  
  // Update settings when data loads
  useState(() => {
    if (systemConfig) {
      const data = systemConfig
      setSettings(prev => ({
        ...prev,
        hostname: data.hostname || '',
        timezone: data.timezone || '',
      }))
    }
  })

  // Update system settings mutation
  const updateSystemMutation = useMutation({
    mutationFn: async (data: any) => {
      await api.put('/api/v1/system/config', data)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['system-config'] })
      toast.success('System settings updated')
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.message || 'Failed to update settings')
    }
  })

  const handleSave = () => {
    updateSystemMutation.mutate({
      hostname: settings.hostname,
      timezone: settings.timezone,
    })
  }

  return (
    <div className="space-y-6">
      <Card title="System Configuration">
        <div className="space-y-4">
          <div>
            <Label htmlFor="hostname">Hostname</Label>
            <Input
              id="hostname"
              value={settings.hostname}
              onChange={(e) => setSettings({ ...settings, hostname: e.target.value })}
              placeholder="Enter system hostname"
            />
            <p className="text-xs text-muted-foreground mt-1">
              The name used to identify this system on the network
            </p>
          </div>
          
          <div>
            <Label htmlFor="timezone">Timezone</Label>
            <Select
              value={settings.timezone}
              onValueChange={(value) => setSettings({ ...settings, timezone: value })}
            >
              <SelectTrigger>
                <SelectValue placeholder="Select timezone" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="America/New_York">America/New York (EST)</SelectItem>
                <SelectItem value="America/Chicago">America/Chicago (CST)</SelectItem>
                <SelectItem value="America/Denver">America/Denver (MST)</SelectItem>
                <SelectItem value="America/Los_Angeles">America/Los Angeles (PST)</SelectItem>
                <SelectItem value="Europe/London">Europe/London (GMT)</SelectItem>
                <SelectItem value="Europe/Paris">Europe/Paris (CET)</SelectItem>
                <SelectItem value="Asia/Tokyo">Asia/Tokyo (JST)</SelectItem>
                <SelectItem value="Australia/Sydney">Australia/Sydney (AEDT)</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      </Card>

      <Card title="Regional Settings">
        <div className="grid grid-cols-2 gap-4">
          <div>
            <Label htmlFor="language">Language</Label>
            <Select
              value={settings.language}
              onValueChange={(value) => setSettings({ ...settings, language: value })}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="en-US">English (US)</SelectItem>
                <SelectItem value="en-GB">English (UK)</SelectItem>
                <SelectItem value="es">Español</SelectItem>
                <SelectItem value="fr">Français</SelectItem>
                <SelectItem value="de">Deutsch</SelectItem>
              </SelectContent>
            </Select>
          </div>
          
          <div>
            <Label htmlFor="dateFormat">Date Format</Label>
            <Select
              value={settings.dateFormat}
              onValueChange={(value) => setSettings({ ...settings, dateFormat: value })}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="MM/DD/YYYY">MM/DD/YYYY</SelectItem>
                <SelectItem value="DD/MM/YYYY">DD/MM/YYYY</SelectItem>
                <SelectItem value="YYYY-MM-DD">YYYY-MM-DD</SelectItem>
              </SelectContent>
            </Select>
          </div>
          
          <div>
            <Label htmlFor="timeFormat">Time Format</Label>
            <Select
              value={settings.timeFormat}
              onValueChange={(value) => setSettings({ ...settings, timeFormat: value })}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="12h">12-hour (AM/PM)</SelectItem>
                <SelectItem value="24h">24-hour</SelectItem>
              </SelectContent>
            </Select>
          </div>
          
          <div>
            <Label htmlFor="temperatureUnit">Temperature Unit</Label>
            <Select
              value={settings.temperatureUnit}
              onValueChange={(value) => setSettings({ ...settings, temperatureUnit: value })}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="celsius">Celsius (°C)</SelectItem>
                <SelectItem value="fahrenheit">Fahrenheit (°F)</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      </Card>

      <div className="flex justify-end">
        <Button 
          onClick={handleSave}
          disabled={updateSystemMutation.isPending}
        >
          {updateSystemMutation.isPending ? (
            <>
              <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
              Saving...
            </>
          ) : (
            <>
              <Save className="h-4 w-4 mr-2" />
              Save Changes
            </>
          )}
        </Button>
      </div>
    </div>
  )
}

// Appearance Settings Component
function AppearanceSettings() {
  const queryClient = useQueryClient()
  const [settings, setSettings] = useState({
    theme: 'system',
    accentColor: '#3b82f6',
    fontSize: 'medium',
    animations: true,
    compactMode: false,
  })

  // Fetch appearance settings
  const { data: appearanceConfig } = useQuery({
    queryKey: ['appearance-config'],
    queryFn: async () => {
      const response = await api.get<any>('/api/v1/settings/appearance')
      return response
    }
  })
  
  // Update appearance when data loads
  useState(() => {
    if (appearanceConfig) {
      const data = appearanceConfig
      setSettings(data)
    }
  })

  // Update appearance mutation
  const updateAppearanceMutation = useMutation({
    mutationFn: async (data: any) => {
      await api.put('/api/v1/settings/appearance', data)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['appearance-config'] })
      toast.success('Appearance settings updated')
      
      // Apply theme immediately
      if (settings.theme === 'dark') {
        document.documentElement.classList.add('dark')
      } else if (settings.theme === 'light') {
        document.documentElement.classList.remove('dark')
      } else {
        // System preference
        const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
        if (prefersDark) {
          document.documentElement.classList.add('dark')
        } else {
          document.documentElement.classList.remove('dark')
        }
      }
      
      // Apply accent color
      document.documentElement.style.setProperty('--accent-color', settings.accentColor)
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.message || 'Failed to update appearance')
    }
  })

  const handleThemeChange = (theme: string) => {
    setSettings({ ...settings, theme })
  }

  const handleSave = () => {
    updateAppearanceMutation.mutate(settings)
  }

  return (
    <div className="space-y-6">
      <Card title="Theme">
        <div className="space-y-4">
          <div className="grid grid-cols-3 gap-2">
            {[
              { value: 'light', label: 'Light', icon: Sun },
              { value: 'dark', label: 'Dark', icon: Moon },
              { value: 'system', label: 'System', icon: Monitor },
            ].map(option => (
              <button
                key={option.value}
                onClick={() => handleThemeChange(option.value)}
                className={cn(
                  "p-4 rounded-lg border-2 transition-all",
                  settings.theme === option.value 
                    ? "border-primary bg-primary/5" 
                    : "border-border hover:border-primary/50"
                )}
              >
                <option.icon className="h-6 w-6 mx-auto mb-2" />
                <p className="text-sm font-medium">{option.label}</p>
              </button>
            ))}
          </div>
        </div>
      </Card>

      <Card title="Accent Color">
        <div className="space-y-4">
          <div className="flex items-center gap-2">
            {[
              '#3b82f6', // Blue
              '#10b981', // Green
              '#f59e0b', // Amber
              '#ef4444', // Red
              '#8b5cf6', // Purple
              '#ec4899', // Pink
              '#06b6d4', // Cyan
            ].map(color => (
              <button
                key={color}
                onClick={() => setSettings({ ...settings, accentColor: color })}
                className={cn(
                  "w-10 h-10 rounded-full border-2 transition-all",
                  settings.accentColor === color 
                    ? "border-gray-900 dark:border-white scale-110" 
                    : "border-transparent"
                )}
                style={{ backgroundColor: color }}
              />
            ))}
            <input
              type="color"
              value={settings.accentColor}
              onChange={(e) => setSettings({ ...settings, accentColor: e.target.value })}
              className="w-10 h-10 rounded-full cursor-pointer"
            />
          </div>
        </div>
      </Card>

      <Card title="Display Options">
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="font-medium">Animations</p>
              <p className="text-sm text-muted-foreground">Enable smooth transitions</p>
            </div>
            <Switch
              checked={settings.animations}
              onCheckedChange={(checked) => setSettings({ ...settings, animations: checked })}
            />
          </div>
          
          <div className="flex items-center justify-between">
            <div>
              <p className="font-medium">Compact Mode</p>
              <p className="text-sm text-muted-foreground">Reduce spacing between elements</p>
            </div>
            <Switch
              checked={settings.compactMode}
              onCheckedChange={(checked) => setSettings({ ...settings, compactMode: checked })}
            />
          </div>
        </div>
      </Card>

      <div className="flex justify-end">
        <Button 
          onClick={handleSave}
          disabled={updateAppearanceMutation.isPending}
        >
          {updateAppearanceMutation.isPending ? (
            <>
              <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
              Saving...
            </>
          ) : (
            <>
              <Save className="h-4 w-4 mr-2" />
              Save Changes
            </>
          )}
        </Button>
      </div>
    </div>
  )
}

// About Section Component
function AboutSection() {
  const { data: systemInfo, isLoading } = useQuery({
    queryKey: ['system-info'],
    queryFn: async () => {
      const response = await api.get<any>('/api/v1/about')
      return response
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
      <Card title="System Information">
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <p className="text-muted-foreground">Version</p>
              <p className="font-medium">{systemInfo?.version || 'Unknown'}</p>
            </div>
            <div>
              <p className="text-muted-foreground">Build</p>
              <p className="font-medium">{systemInfo?.build || 'Unknown'}</p>
            </div>
            <div>
              <p className="text-muted-foreground">Kernel</p>
              <p className="font-medium">{systemInfo?.kernel || 'Unknown'}</p>
            </div>
            <div>
              <p className="text-muted-foreground">Architecture</p>
              <p className="font-medium">{systemInfo?.arch || 'Unknown'}</p>
            </div>
            <div>
              <p className="text-muted-foreground">Hostname</p>
              <p className="font-medium">{systemInfo?.hostname || 'Unknown'}</p>
            </div>
            <div>
              <p className="text-muted-foreground">Uptime</p>
              <p className="font-medium">{systemInfo?.uptime || 'Unknown'}</p>
            </div>
          </div>
        </div>
      </Card>

      <Card title="Hardware">
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <p className="text-muted-foreground">CPU</p>
              <p className="font-medium">{systemInfo?.hardware?.cpu || 'Unknown'}</p>
            </div>
            <div>
              <p className="text-muted-foreground">Cores</p>
              <p className="font-medium">{systemInfo?.hardware?.cores || 'Unknown'}</p>
            </div>
            <div>
              <p className="text-muted-foreground">Memory</p>
              <p className="font-medium">{systemInfo?.hardware?.memory || 'Unknown'}</p>
            </div>
            <div>
              <p className="text-muted-foreground">Storage</p>
              <p className="font-medium">{systemInfo?.hardware?.storage || 'Unknown'}</p>
            </div>
          </div>
        </div>
      </Card>

      <Card title="Network">
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <p className="text-muted-foreground">Primary Interface</p>
              <p className="font-medium">{systemInfo?.network?.interface || 'Unknown'}</p>
            </div>
            <div>
              <p className="text-muted-foreground">IP Address</p>
              <p className="font-medium">{systemInfo?.network?.ipAddress || 'Unknown'}</p>
            </div>
            <div>
              <p className="text-muted-foreground">MAC Address</p>
              <p className="font-medium">{systemInfo?.network?.macAddress || 'Unknown'}</p>
            </div>
            <div>
              <p className="text-muted-foreground">Gateway</p>
              <p className="font-medium">{systemInfo?.network?.gateway || 'Unknown'}</p>
            </div>
          </div>
        </div>
      </Card>

      <Card title="License">
        <div className="space-y-2 text-sm">
          <p>nOS is licensed under the MIT License.</p>
          <p className="text-muted-foreground">
            Copyright © 2024 nOS Team. All rights reserved.
          </p>
        </div>
      </Card>
    </div>
  )
}

// Advanced Settings Component
function AdvancedSettings() {
  const queryClient = useQueryClient()
  const [settings, setSettings] = useState({
    enableSSH: false,
    sshPort: 22,
    enableTelemetry: false,
    debugMode: false,
    apiRateLimit: 100,
    sessionTimeout: 30,
  })

  // Mutation for updating advanced settings
  const updateAdvancedMutation = useMutation({
    mutationFn: async (data: any) => {
      await api.put('/api/v1/settings/advanced', data)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['advanced-settings'] })
      toast.success('Advanced settings updated')
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.message || 'Failed to update settings')
    }
  })

  const handleSave = () => {
    updateAdvancedMutation.mutate(settings)
  }

  return (
    <div className="space-y-6">
      <Alert>
        <AlertCircle className="h-4 w-4" />
        <AlertDescription>
          These settings can affect system stability. Change with caution.
        </AlertDescription>
      </Alert>

      <Card title="Remote Access">
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="font-medium">Enable SSH</p>
              <p className="text-sm text-muted-foreground">Allow secure shell access</p>
            </div>
            <Switch
              checked={settings.enableSSH}
              onCheckedChange={(checked) => setSettings({ ...settings, enableSSH: checked })}
            />
          </div>
          
          {settings.enableSSH && (
            <div>
              <Label htmlFor="ssh-port">SSH Port</Label>
              <Input
                id="ssh-port"
                type="number"
                value={settings.sshPort}
                onChange={(e) => setSettings({ ...settings, sshPort: parseInt(e.target.value) })}
                min="1"
                max="65535"
              />
            </div>
          )}
        </div>
      </Card>

      <Card title="System">
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="font-medium">Debug Mode</p>
              <p className="text-sm text-muted-foreground">Enable verbose logging</p>
            </div>
            <Switch
              checked={settings.debugMode}
              onCheckedChange={(checked) => setSettings({ ...settings, debugMode: checked })}
            />
          </div>
          
          <div className="flex items-center justify-between">
            <div>
              <p className="font-medium">Telemetry</p>
              <p className="text-sm text-muted-foreground">Send anonymous usage data</p>
            </div>
            <Switch
              checked={settings.enableTelemetry}
              onCheckedChange={(checked) => setSettings({ ...settings, enableTelemetry: checked })}
            />
          </div>
        </div>
      </Card>

      <Card title="Performance">
        <div className="space-y-4">
          <div>
            <Label htmlFor="rate-limit">API Rate Limit</Label>
            <Input
              id="rate-limit"
              type="number"
              value={settings.apiRateLimit}
              onChange={(e) => setSettings({ ...settings, apiRateLimit: parseInt(e.target.value) })}
              min="10"
              max="1000"
            />
            <p className="text-xs text-muted-foreground mt-1">
              Maximum API requests per minute
            </p>
          </div>
          
          <div>
            <Label htmlFor="session-timeout">Session Timeout (minutes)</Label>
            <Input
              id="session-timeout"
              type="number"
              value={settings.sessionTimeout}
              onChange={(e) => setSettings({ ...settings, sessionTimeout: parseInt(e.target.value) })}
              min="5"
              max="1440"
            />
            <p className="text-xs text-muted-foreground mt-1">
              Automatically log out after inactivity
            </p>
          </div>
        </div>
      </Card>

      <div className="flex justify-end gap-4">
        <Button variant="outline">
          Reset to Defaults
        </Button>
        <Button 
          onClick={handleSave}
          disabled={updateAdvancedMutation.isPending}
        >
          {updateAdvancedMutation.isPending ? (
            <>
              <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
              Saving...
            </>
          ) : (
            <>
              <Save className="h-4 w-4 mr-2" />
              Save Changes
            </>
          )}
        </Button>
      </div>
    </div>
  )
}

// Main Settings Component
export function SettingsConnected() {
  const navigate = useNavigate()
  const [activeSection, setActiveSection] = useState('general')

  const renderSection = () => {
    switch (activeSection) {
      case 'general':
        return <GeneralSettings />
      case 'appearance':
        return <AppearanceSettings />
      case 'about':
        return <AboutSection />
      case 'advanced':
        return <AdvancedSettings />
      default:
        return <GeneralSettings />
    }
  }

  return (
    <div className="container mx-auto py-6">
      <PageHeader
        title="Settings"
        description="Configure your nOS system"
        icon={SettingsIcon}
      />

      <div className="grid grid-cols-12 gap-6 mt-6">
        {/* Sidebar */}
        <div className="col-span-3">
          <Card className="p-4">
            <nav className="space-y-1">
              {settingsSections.map(section => (
                <button
                  key={section.id}
                  onClick={() => setActiveSection(section.id)}
                  className={cn(
                    "w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors",
                    activeSection === section.id
                      ? "bg-primary text-primary-foreground"
                      : "hover:bg-muted"
                  )}
                >
                  <section.icon className="h-4 w-4" />
                  {section.label}
                </button>
              ))}
            </nav>
            
            <div className="mt-6 pt-6 border-t">
              <p className="text-xs font-medium text-muted-foreground mb-3">Quick Links</p>
              <div className="space-y-1">
                {quickLinks.map(link => (
                  <button
                    key={link.path}
                    onClick={() => navigate(link.path)}
                    className="w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm hover:bg-muted transition-colors"
                  >
                    <link.icon className="h-4 w-4" />
                    <div className="text-left flex-1">
                      <p className="font-medium">{link.label}</p>
                      <p className="text-xs text-muted-foreground">{link.description}</p>
                    </div>
                    <ChevronRight className="h-4 w-4" />
                  </button>
                ))}
              </div>
            </div>
          </Card>
        </div>

        {/* Content */}
        <div className="col-span-9">
          <motion.div
            key={activeSection}
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.2 }}
          >
            {renderSection()}
          </motion.div>
        </div>
      </div>
    </div>
  )
}
