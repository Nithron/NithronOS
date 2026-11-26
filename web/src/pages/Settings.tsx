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
  Globe,
  Server,
  Shield,
  Database,
  Volume2,
  Eye,
  Palette,
  Terminal,
  FileText,
  Activity,
  Zap,
  Lock,
  Cloud,
  RefreshCw,
  AlertCircle,
} from 'lucide-react'
import { PageHeader } from '@/components/ui/page-header'
import { Card } from '@/components/ui/card-enhanced'
import { Button } from '@/components/ui/button'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { cn } from '@/lib/utils'
import { toast } from '@/components/ui/toast'
import { useSystemInfo } from '@/hooks/use-api'

// Settings sections - removed duplicates
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

// General Settings
function GeneralSettings() {
  const [settings, setSettings] = useState({
    hostname: 'nos',
    timezone: 'America/New_York',
    language: 'en-US',
    dateFormat: 'MM/DD/YYYY',
    timeFormat: '12h',
    temperatureUnit: 'celsius',
    firstDayOfWeek: 'sunday',
  })

  return (
    <div className="space-y-6">
      <Card title="System Configuration">
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-2">Hostname</label>
            <input
              type="text"
              value={settings.hostname}
              onChange={(e) => setSettings({ ...settings, hostname: e.target.value })}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
              placeholder="Enter system hostname"
            />
            <p className="text-xs text-muted-foreground mt-1">
              The name used to identify this system on the network
            </p>
          </div>
          
          <div>
            <label className="block text-sm font-medium mb-2">Timezone</label>
            <select
              value={settings.timezone}
              onChange={(e) => setSettings({ ...settings, timezone: e.target.value })}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            >
              <option value="America/New_York">America/New York (EST)</option>
              <option value="America/Chicago">America/Chicago (CST)</option>
              <option value="America/Denver">America/Denver (MST)</option>
              <option value="America/Los_Angeles">America/Los Angeles (PST)</option>
              <option value="Europe/London">Europe/London (GMT)</option>
              <option value="Europe/Paris">Europe/Paris (CET)</option>
              <option value="Asia/Tokyo">Asia/Tokyo (JST)</option>
              <option value="Australia/Sydney">Australia/Sydney (AEDT)</option>
            </select>
          </div>
        </div>
      </Card>

      <Card title="Regional Settings">
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium mb-2">Language</label>
            <select
              value={settings.language}
              onChange={(e) => setSettings({ ...settings, language: e.target.value })}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            >
              <option value="en-US">English (US)</option>
              <option value="en-GB">English (UK)</option>
              <option value="es">Español</option>
              <option value="fr">Français</option>
              <option value="de">Deutsch</option>
              <option value="it">Italiano</option>
              <option value="pt">Português</option>
              <option value="ja">日本語</option>
              <option value="zh">中文</option>
            </select>
          </div>
          
          <div>
            <label className="block text-sm font-medium mb-2">Date Format</label>
            <select
              value={settings.dateFormat}
              onChange={(e) => setSettings({ ...settings, dateFormat: e.target.value })}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            >
              <option value="MM/DD/YYYY">MM/DD/YYYY</option>
              <option value="DD/MM/YYYY">DD/MM/YYYY</option>
              <option value="YYYY-MM-DD">YYYY-MM-DD</option>
            </select>
          </div>
          
          <div>
            <label className="block text-sm font-medium mb-2">Time Format</label>
            <select
              value={settings.timeFormat}
              onChange={(e) => setSettings({ ...settings, timeFormat: e.target.value })}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            >
              <option value="12h">12-hour (AM/PM)</option>
              <option value="24h">24-hour</option>
            </select>
          </div>
          
          <div>
            <label className="block text-sm font-medium mb-2">Temperature Unit</label>
            <select
              value={settings.temperatureUnit}
              onChange={(e) => setSettings({ ...settings, temperatureUnit: e.target.value })}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            >
              <option value="celsius">Celsius (°C)</option>
              <option value="fahrenheit">Fahrenheit (°F)</option>
            </select>
          </div>
        </div>
      </Card>

      <div className="flex justify-end">
        <Button onClick={() => toast.success('Settings saved successfully')}>
          <Save className="h-4 w-4 mr-2" />
          Save Changes
        </Button>
      </div>
    </div>
  )
}

// Appearance Settings
function AppearanceSettings() {
  const [theme, setTheme] = useState('system')
  const [accentColor, setAccentColor] = useState('#3b82f6')
  const [fontSize, setFontSize] = useState('medium')

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
                onClick={() => setTheme(option.value)}
                className={cn(
                  "p-4 rounded-lg border-2 transition-all",
                  theme === option.value
                    ? "border-primary bg-primary/10"
                    : "border-border hover:bg-muted/50"
                )}
              >
                <option.icon className="h-6 w-6 mx-auto mb-2" />
                <div className="text-sm font-medium">{option.label}</div>
              </button>
            ))}
          </div>
        </div>
      </Card>

      <Card title="Customization">
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-2">Accent Color</label>
            <div className="flex gap-2">
              {['#3b82f6', '#8b5cf6', '#ec4899', '#10b981', '#f59e0b', '#ef4444'].map(color => (
                <button
                  key={color}
                  onClick={() => setAccentColor(color)}
                  className={cn(
                    "w-10 h-10 rounded-lg border-2",
                    accentColor === color ? "border-foreground" : "border-transparent"
                  )}
                  style={{ backgroundColor: color }}
                />
              ))}
              <input
                type="color"
                value={accentColor}
                onChange={(e) => setAccentColor(e.target.value)}
                className="w-10 h-10 rounded-lg border cursor-pointer"
              />
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium mb-2">Font Size</label>
            <select
              value={fontSize}
              onChange={(e) => setFontSize(e.target.value)}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            >
              <option value="small">Small</option>
              <option value="medium">Medium (Default)</option>
              <option value="large">Large</option>
              <option value="extra-large">Extra Large</option>
            </select>
          </div>

          <label className="flex items-center gap-2">
            <input type="checkbox" className="rounded" />
            <span className="text-sm">Enable animations</span>
          </label>

          <label className="flex items-center gap-2">
            <input type="checkbox" defaultChecked className="rounded" />
            <span className="text-sm">Show tooltips</span>
          </label>
        </div>
      </Card>
    </div>
  )
}

// Notifications Settings
function NotificationsSettings() {
  return (
    <div className="space-y-6">
      <Card title="Notification Preferences">
        <div className="space-y-3">
          <label className="flex items-center justify-between p-3 rounded-lg hover:bg-muted/50">
            <div className="flex items-center gap-3">
              <Bell className="h-5 w-5 text-muted-foreground" />
              <div>
                <div className="font-medium">System Alerts</div>
                <div className="text-sm text-muted-foreground">Critical system notifications</div>
              </div>
            </div>
            <input type="checkbox" defaultChecked className="rounded" />
          </label>

          <label className="flex items-center justify-between p-3 rounded-lg hover:bg-muted/50">
            <div className="flex items-center gap-3">
              <Activity className="h-5 w-5 text-muted-foreground" />
              <div>
                <div className="font-medium">Job Notifications</div>
                <div className="text-sm text-muted-foreground">Backup, scrub, and sync job status</div>
              </div>
            </div>
            <input type="checkbox" defaultChecked className="rounded" />
          </label>

          <label className="flex items-center justify-between p-3 rounded-lg hover:bg-muted/50">
            <div className="flex items-center gap-3">
              <Download className="h-5 w-5 text-muted-foreground" />
              <div>
                <div className="font-medium">Update Notifications</div>
                <div className="text-sm text-muted-foreground">Available system updates</div>
              </div>
            </div>
            <input type="checkbox" defaultChecked className="rounded" />
          </label>

          <label className="flex items-center justify-between p-3 rounded-lg hover:bg-muted/50">
            <div className="flex items-center gap-3">
              <Volume2 className="h-5 w-5 text-muted-foreground" />
              <div>
                <div className="font-medium">Sound Notifications</div>
                <div className="text-sm text-muted-foreground">Play sounds for notifications</div>
              </div>
            </div>
            <input type="checkbox" className="rounded" />
          </label>
        </div>
      </Card>

      <Card title="Email Notifications">
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-2">Email Address</label>
            <input
              type="email"
              placeholder="admin@example.com"
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            />
          </div>

          <label className="flex items-center gap-2">
            <input type="checkbox" className="rounded" />
            <span className="text-sm">Send daily summary</span>
          </label>

          <label className="flex items-center gap-2">
            <input type="checkbox" defaultChecked className="rounded" />
            <span className="text-sm">Send critical alerts immediately</span>
          </label>
        </div>
      </Card>
    </div>
  )
}

// Privacy & Security Settings
function PrivacySettings() {
  return (
    <div className="space-y-6">
      <Card title="Privacy">
        <div className="space-y-3">
          <label className="flex items-center justify-between p-3 rounded-lg hover:bg-muted/50">
            <div className="flex items-center gap-3">
              <Eye className="h-5 w-5 text-muted-foreground" />
              <div>
                <div className="font-medium">Telemetry</div>
                <div className="text-sm text-muted-foreground">Share anonymous usage data</div>
              </div>
            </div>
            <input type="checkbox" className="rounded" />
          </label>

          <label className="flex items-center justify-between p-3 rounded-lg hover:bg-muted/50">
            <div className="flex items-center gap-3">
              <Cloud className="h-5 w-5 text-muted-foreground" />
              <div>
                <div className="font-medium">Crash Reports</div>
                <div className="text-sm text-muted-foreground">Automatically send crash reports</div>
              </div>
            </div>
            <input type="checkbox" className="rounded" />
          </label>
        </div>
      </Card>

      <Card title="Security">
        <div className="space-y-3">
          <label className="flex items-center justify-between p-3 rounded-lg hover:bg-muted/50">
            <div className="flex items-center gap-3">
              <Lock className="h-5 w-5 text-muted-foreground" />
              <div>
                <div className="font-medium">Session Timeout</div>
                <div className="text-sm text-muted-foreground">Auto-logout after inactivity</div>
              </div>
            </div>
            <select className="rounded-md border border-input bg-background px-3 py-1 text-sm">
              <option>15 minutes</option>
              <option>30 minutes</option>
              <option>1 hour</option>
              <option>Never</option>
            </select>
          </label>

          <label className="flex items-center justify-between p-3 rounded-lg hover:bg-muted/50">
            <div className="flex items-center gap-3">
              <Shield className="h-5 w-5 text-muted-foreground" />
              <div>
                <div className="font-medium">Login Notifications</div>
                <div className="text-sm text-muted-foreground">Alert on new device login</div>
              </div>
            </div>
            <input type="checkbox" defaultChecked className="rounded" />
          </label>
        </div>
      </Card>
    </div>
  )
}

// Advanced Settings
function AdvancedSettings() {
  return (
    <div className="space-y-6">
      <Alert>
        <AlertCircle className="h-4 w-4" />
        <AlertDescription>
          These settings are for advanced users. Incorrect configuration may affect system stability.
        </AlertDescription>
      </Alert>

      <Card title="Performance">
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-2">Cache Size (MB)</label>
            <input
              type="number"
              defaultValue="512"
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            />
          </div>

          <div>
            <label className="block text-sm font-medium mb-2">Worker Threads</label>
            <input
              type="number"
              defaultValue="4"
              min="1"
              max="16"
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            />
          </div>

          <label className="flex items-center gap-2">
            <input type="checkbox" defaultChecked className="rounded" />
            <span className="text-sm">Enable hardware acceleration</span>
          </label>
        </div>
      </Card>

      <Card title="Developer Options">
        <div className="space-y-3">
          <label className="flex items-center justify-between p-3 rounded-lg hover:bg-muted/50">
            <div className="flex items-center gap-3">
              <Terminal className="h-5 w-5 text-muted-foreground" />
              <div>
                <div className="font-medium">Debug Mode</div>
                <div className="text-sm text-muted-foreground">Enable verbose logging</div>
              </div>
            </div>
            <input type="checkbox" className="rounded" />
          </label>

          <label className="flex items-center justify-between p-3 rounded-lg hover:bg-muted/50">
            <div className="flex items-center gap-3">
              <FileText className="h-5 w-5 text-muted-foreground" />
              <div>
                <div className="font-medium">API Documentation</div>
                <div className="text-sm text-muted-foreground">Enable Swagger UI at /api/docs</div>
              </div>
            </div>
            <input type="checkbox" className="rounded" />
          </label>
        </div>
      </Card>

      <Card title="Maintenance">
        <div className="space-y-3">
          <Button variant="outline" className="w-full justify-start">
            <Database className="h-4 w-4 mr-2" />
            Clear Application Cache
          </Button>
          <Button variant="outline" className="w-full justify-start">
            <RefreshCw className="h-4 w-4 mr-2" />
            Reset to Default Settings
          </Button>
          <Button variant="outline" className="w-full justify-start text-destructive">
            <Zap className="h-4 w-4 mr-2" />
            Factory Reset
          </Button>
        </div>
      </Card>
    </div>
  )
}

// About Section
function AboutSection() {
  const { data: systemInfo } = useSystemInfo()
  
  return (
    <div className="space-y-6">
      <Card title="nOS">
        <div className="space-y-4">
          <div className="flex items-center gap-4">
            <div className="w-16 h-16 rounded-xl bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center">
              <Server className="h-8 w-8 text-white" />
            </div>
            <div>
              <h3 className="text-lg font-semibold">nOS</h3>
              <p className="text-sm text-muted-foreground">Open Source NAS & HomeLab Platform</p>
              <p className="text-xs text-muted-foreground mt-1">Version {systemInfo?.version || '2.1.0'}</p>
            </div>
          </div>
          
          <div className="pt-4 border-t space-y-2 text-sm">
            <div className="flex justify-between">
              <span className="text-muted-foreground">License</span>
              <span>Open Source (MIT)</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Build Date</span>
              <span>{new Date().toLocaleDateString()}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Kernel</span>
              <span>{systemInfo?.kernel || 'Linux 6.1.0'}</span>
            </div>
          </div>
        </div>
      </Card>

      <Card title="System Information">
        <div className="grid grid-cols-2 gap-4 text-sm">
          <div>
            <p className="text-muted-foreground">Hostname</p>
            <p className="font-medium">{systemInfo?.hostname || 'nos'}</p>
          </div>
          <div>
            <p className="text-muted-foreground">Architecture</p>
            <p className="font-medium">{systemInfo?.arch || 'x86_64'}</p>
          </div>
          <div>
            <p className="text-muted-foreground">CPU</p>
            <p className="font-medium">{(systemInfo as any)?.cpuModel || 'Intel Xeon'}</p>
          </div>
          <div>
            <p className="text-muted-foreground">Memory</p>
            <p className="font-medium">{systemInfo?.memoryTotal ? `${Math.round(systemInfo.memoryTotal / 1024 / 1024 / 1024)} GB` : '32 GB'}</p>
          </div>
          <div>
            <p className="text-muted-foreground">Uptime</p>
            <p className="font-medium">{systemInfo?.uptime ? `${Math.floor(systemInfo.uptime / 86400)} days` : '15 days'}</p>
          </div>
          <div>
            <p className="text-muted-foreground">Load Average</p>
            <p className="font-medium">0.5, 0.8, 0.6</p>
          </div>
        </div>
      </Card>

      <Card title="Resources">
        <div className="space-y-2">
          <a href="https://docs.nithron.com" target="_blank" rel="noopener noreferrer" 
             className="flex items-center justify-between p-3 rounded-lg hover:bg-muted/50">
            <div className="flex items-center gap-3">
              <FileText className="h-5 w-5 text-muted-foreground" />
              <div>
                <div className="font-medium">Documentation</div>
                <div className="text-sm text-muted-foreground">User guides and API docs</div>
              </div>
            </div>
            <ChevronRight className="h-4 w-4" />
          </a>
          
          <a href="https://github.com/Nithron/nOS" target="_blank" rel="noopener noreferrer"
             className="flex items-center justify-between p-3 rounded-lg hover:bg-muted/50">
            <div className="flex items-center gap-3">
              <Globe className="h-5 w-5 text-muted-foreground" />
              <div>
                <div className="font-medium">GitHub Repository</div>
                <div className="text-sm text-muted-foreground">Source code and issues</div>
              </div>
            </div>
            <ChevronRight className="h-4 w-4" />
          </a>
          
          <a href="https://community.nithron.com" target="_blank" rel="noopener noreferrer"
             className="flex items-center justify-between p-3 rounded-lg hover:bg-muted/50">
            <div className="flex items-center gap-3">
              <Users className="h-5 w-5 text-muted-foreground" />
              <div>
                <div className="font-medium">Community Forum</div>
                <div className="text-sm text-muted-foreground">Get help and share ideas</div>
              </div>
            </div>
            <ChevronRight className="h-4 w-4" />
          </a>
        </div>
      </Card>
    </div>
  )
}

export function Settings() {
  const [activeSection, setActiveSection] = useState('general')
  const navigate = useNavigate()

  const renderSection = () => {
    switch (activeSection) {
      case 'general': return <GeneralSettings />
      case 'appearance': return <AppearanceSettings />
      case 'notifications': return <NotificationsSettings />
      case 'privacy': return <PrivacySettings />
      case 'advanced': return <AdvancedSettings />
      case 'about': return <AboutSection />
      default: return <GeneralSettings />
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Settings"
        description="Configure system settings and preferences"
      />

      <div className="flex gap-6">
        {/* Sidebar */}
        <div className="w-64 shrink-0 space-y-4">
          <Card>
            <nav className="space-y-1">
              {settingsSections.map(section => (
                <button
                  key={section.id}
                  onClick={() => setActiveSection(section.id)}
                  className={cn(
                    "w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors text-left",
                    activeSection === section.id
                      ? "bg-primary text-primary-foreground"
                      : "hover:bg-muted/50"
                  )}
                >
                  <section.icon className="h-4 w-4" />
                  {section.label}
                </button>
              ))}
            </nav>
          </Card>

          {/* Quick Links */}
          <Card title="More Settings">
            <nav className="space-y-1">
              {quickLinks.map(link => (
                <button
                  key={link.path}
                  onClick={() => navigate(link.path)}
                  className="w-full flex items-center justify-between p-2 rounded-lg text-sm hover:bg-muted/50 text-left"
                >
                  <div className="flex items-center gap-2">
                    <link.icon className="h-4 w-4 text-muted-foreground" />
                    <span>{link.label}</span>
                  </div>
                  <ChevronRight className="h-4 w-4 text-muted-foreground" />
                </button>
              ))}
            </nav>
          </Card>
        </div>

        {/* Main content */}
        <motion.div 
          key={activeSection}
          initial={{ opacity: 0, x: 20 }}
          animate={{ opacity: 1, x: 0 }}
          transition={{ duration: 0.2 }}
          className="flex-1"
        >
          {renderSection()}
        </motion.div>
      </div>
    </div>
  )
}