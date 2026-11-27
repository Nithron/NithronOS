import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Progress } from '@/components/ui/progress';
import { ChevronLeftIcon, ChevronRightIcon, ConnectionIcon, InternetIcon, KeyIcon, LockIcon, OkIcon, RouterIcon, SecurityIcon, WarningIcon, WorkingIcon } from '@/components/icons'
import {
  useStartWizard,
  useWizardState,
  useWizardNext,
  useCompleteWizard,
} from '@/api/net';
import type { AccessMode, HTTPSMode } from '@/api/net.types';

const WIZARD_STEPS = [
  { id: 1, title: 'Access mode', icon: SecurityIcon },
  { id: 2, title: 'Configuration', icon: RouterIcon },
  { id: 3, title: 'Security', icon: LockIcon },
  { id: 4, title: 'Review', icon: OkIcon },
];

export function RemoteAccessWizard() {
  const navigate = useNavigate();
  const [currentStep, setCurrentStep] = useState(1);
  const [formData, setFormData] = useState<any>({
    access_mode: 'lan_only' as AccessMode,
    wireguard: {
      cidr: '10.8.0.0/24',
      listen_port: 51820,
      endpoint_hostname: '',
      dns: ['1.1.1.1', '1.0.0.1'],
    },
    https: {
      // The HTTPS step is only for public HTTPS: Let's Encrypt, HTTP challenge.
      mode: 'http_01' as HTTPSMode,
      domain: '',
      email: '',
      dns_provider: '',
      dns_api_key: '',
    },
  });

  const startWizard = useStartWizard();
  const wizardState = useWizardState();
  const wizardNext = useWizardNext();
  const completeWizard = useCompleteWizard();

  useEffect(() => {
    // Start wizard session
    if (!wizardState.data && !startWizard.isPending) {
      startWizard.mutate();
    }
  }, []);

  useEffect(() => {
    // Sync state with backend
    if (wizardState.data) {
      setCurrentStep(wizardState.data.step);
    }
  }, [wizardState.data]);

  const handleNext = async () => {
    const stepData = {
      access_mode: formData.access_mode,
      ...(formData.access_mode === 'wireguard' ? formData.wireguard : 
          formData.access_mode === 'public_https' ? formData.https : {}),
    };

    try {
      await wizardNext.mutateAsync(stepData);
      if (currentStep < 4) {
        setCurrentStep(currentStep + 1);
      }
    } catch (error) {
      console.error('Failed to proceed:', error);
    }
  };

  const handleBack = () => {
    if (currentStep > 1) {
      setCurrentStep(currentStep - 1);
    }
  };

  const handleComplete = async () => {
    try {
      await completeWizard.mutateAsync();
      navigate('/settings/network');
    } catch (error) {
      console.error('Failed to complete wizard:', error);
    }
  };

  const renderAccessModeStep = () => (
    <div className="space-y-6">
      <RadioGroup
        value={formData.access_mode}
        onValueChange={(value: string) => setFormData({ ...formData, access_mode: value as AccessMode })}
      >
        <div className="space-y-4">
          <Card className={formData.access_mode === 'lan_only' ? 'border-primary' : ''}>
            <CardHeader>
              <div className="flex items-center gap-2">
                <RadioGroupItem value="lan_only" id="lan_only" />
                <Label htmlFor="lan_only" className="flex-1 cursor-pointer">
                  <div className="flex items-center gap-2">
                    <ConnectionIcon className="h-5 w-5" />
                    <span className="font-semibold">LAN only (recommended)</span>
                  </div>
                  <p className="text-sm text-muted-foreground mt-1">
                    Access nOS only from your local network. Most secure option.
                  </p>
                </Label>
              </div>
            </CardHeader>
          </Card>

          <Card className={formData.access_mode === 'wireguard' ? 'border-primary' : ''}>
            <CardHeader>
              <div className="flex items-center gap-2">
                <RadioGroupItem value="wireguard" id="wireguard" />
                <Label htmlFor="wireguard" className="flex-1 cursor-pointer">
                  <div className="flex items-center gap-2">
                    <KeyIcon className="h-5 w-5" />
                    <span className="font-semibold">WireGuard VPN</span>
                  </div>
                  <p className="text-sm text-muted-foreground mt-1">
                    Secure remote access through encrypted VPN tunnel.
                  </p>
                </Label>
              </div>
            </CardHeader>
          </Card>

          <Card className={formData.access_mode === 'public_https' ? 'border-primary' : ''}>
            <CardHeader>
              <div className="flex items-center gap-2">
                <RadioGroupItem value="public_https" id="public_https" />
                <Label htmlFor="public_https" className="flex-1 cursor-pointer">
                  <div className="flex items-center gap-2">
                    <InternetIcon className="h-5 w-5" />
                    <span className="font-semibold">Public HTTPS</span>
                    <Badge variant="secondary">Advanced</Badge>
                  </div>
                  <p className="text-sm text-muted-foreground mt-1">
                    Expose nOS to the internet with HTTPS and domain.
                  </p>
                </Label>
              </div>
            </CardHeader>
          </Card>
        </div>
      </RadioGroup>

      {formData.access_mode === 'public_https' && (
        <Alert>
          <WarningIcon className="h-4 w-4" />
          <AlertDescription>
            Public HTTPS requires a domain name pointing to this server and open ports 80/443.
            2FA will be enforced for all non-LAN connections.
          </AlertDescription>
        </Alert>
      )}
    </div>
  );

  const renderConfigurationStep = () => {
    if (formData.access_mode === 'wireguard') {
      return (
        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>WireGuard configuration</CardTitle>
              <CardDescription>Configure your VPN server settings</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <Label htmlFor="wg-cidr">VPN subnet</Label>
                <Input
                  id="wg-cidr"
                  value={formData.wireguard.cidr}
                  onChange={(e) => setFormData({
                    ...formData,
                    wireguard: { ...formData.wireguard, cidr: e.target.value }
                  })}
                  placeholder="10.8.0.0/24"
                />
                <p className="text-sm text-muted-foreground mt-1">
                  IP range for VPN clients
                </p>
              </div>

              <div>
                <Label htmlFor="wg-port">Listen port</Label>
                <Input
                  id="wg-port"
                  type="number"
                  value={formData.wireguard.listen_port}
                  onChange={(e) => setFormData({
                    ...formData,
                    wireguard: { ...formData.wireguard, listen_port: parseInt(e.target.value) }
                  })}
                  placeholder="51820"
                />
                <p className="text-sm text-muted-foreground mt-1">
                  UDP port for WireGuard (must be open in router)
                </p>
              </div>

              <div>
                <Label htmlFor="wg-endpoint">Public endpoint (optional)</Label>
                <Input
                  id="wg-endpoint"
                  value={formData.wireguard.endpoint_hostname}
                  onChange={(e) => setFormData({
                    ...formData,
                    wireguard: { ...formData.wireguard, endpoint_hostname: e.target.value }
                  })}
                  placeholder="vpn.example.com or 203.0.113.1"
                />
                <p className="text-sm text-muted-foreground mt-1">
                  Public IP or hostname for clients to connect
                </p>
              </div>

              <div>
                <Label htmlFor="wg-dns">DNS servers</Label>
                <Input
                  id="wg-dns"
                  value={formData.wireguard.dns.join(', ')}
                  onChange={(e) => setFormData({
                    ...formData,
                    wireguard: { ...formData.wireguard, dns: e.target.value.split(',').map(s => s.trim()) }
                  })}
                  placeholder="1.1.1.1, 1.0.0.1"
                />
                <p className="text-sm text-muted-foreground mt-1">
                  DNS servers for VPN clients
                </p>
              </div>
            </CardContent>
          </Card>
        </div>
      );
    }

    if (formData.access_mode === 'public_https') {
      return (
        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>HTTPS configuration</CardTitle>
              <CardDescription>Configure domain and SSL certificate</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <Label htmlFor="https-domain">Domain name</Label>
                <Input
                  id="https-domain"
                  value={formData.https.domain}
                  onChange={(e) => setFormData({
                    ...formData,
                    https: { ...formData.https, domain: e.target.value }
                  })}
                  placeholder="nas.example.com"
                  required
                />
                <p className="text-sm text-muted-foreground mt-1">
                  Must point to this server's IP address
                </p>
              </div>

              <div>
                <Label htmlFor="https-email">Email address</Label>
                <Input
                  id="https-email"
                  type="email"
                  value={formData.https.email}
                  onChange={(e) => setFormData({
                    ...formData,
                    https: { ...formData.https, email: e.target.value }
                  })}
                  placeholder="admin@example.com"
                  required
                />
                <p className="text-sm text-muted-foreground mt-1">
                  For Let's Encrypt notifications
                </p>
              </div>

              <div>
                <Label>Certificate method</Label>
                <p className="text-sm text-muted-foreground">
                  A free Let's Encrypt certificate, obtained with the HTTP challenge: ports 80 and 443 must be
                  reachable from the internet, and the domain must point to this system. (The DNS challenge
                  needs Caddy plugins Debian's Caddy doesn't include.)
                </p>
              </div>

              
            </CardContent>
          </Card>
        </div>
      );
    }

    return (
      <div className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>LAN only configuration</CardTitle>
            <CardDescription>
              Your nOS instance will only be accessible from your local network
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Alert>
              <SecurityIcon className="h-4 w-4" />
              <AlertDescription>
                No additional configuration needed. The firewall will block all external access.
              </AlertDescription>
            </Alert>
          </CardContent>
        </Card>
      </div>
    );
  };

  const renderSecurityStep = () => (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Security settings</CardTitle>
          <CardDescription>Review security implications of your configuration</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {formData.access_mode === 'public_https' && (
            <Alert>
              <LockIcon className="h-4 w-4" />
              <AlertDescription>
                <strong>2FA enforcement:</strong> All non-LAN admin sessions will require two-factor authentication.
                Make sure to set up TOTP after completing this wizard.
              </AlertDescription>
            </Alert>
          )}

          <div className="space-y-2">
            <h4 className="font-medium">Firewall rules preview</h4>
            <div className="rounded-lg bg-muted p-4 font-mono text-sm">
              <div className="space-y-1">
                <div className="text-green-600">+ Allow LAN to UI/API (192.168.0.0/16, 10.0.0.0/8)</div>
                {formData.access_mode === 'lan_only' && (
                  <div className="text-red-600">+ Block WAN to ports 80, 443</div>
                )}
                {formData.access_mode === 'wireguard' && (
                  <>
                    <div className="text-green-600">+ Allow UDP port {formData.wireguard.listen_port} (WireGuard)</div>
                    <div className="text-red-600">+ Block WAN to UI/API</div>
                  </>
                )}
                {formData.access_mode === 'public_https' && (
                  <>
                    <div className="text-green-600">+ Allow TCP ports 80, 443 (HTTPS)</div>
                    <div className="text-yellow-600">+ Require 2FA for non-LAN</div>
                  </>
                )}
              </div>
            </div>
          </div>

          <Alert>
            <WarningIcon className="h-4 w-4" />
            <AlertDescription>
              A 60-second rollback timer will activate after applying firewall changes.
              You must confirm the changes work or they will automatically revert.
            </AlertDescription>
          </Alert>
        </CardContent>
      </Card>
    </div>
  );

  const renderReviewStep = () => (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Review configuration</CardTitle>
          <CardDescription>Confirm your remote access settings before applying</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div>
            <h4 className="font-medium mb-2">Access mode</h4>
            <Badge variant="outline" className="text-base">
              {formData.access_mode === 'lan_only' && 'LAN Only'}
              {formData.access_mode === 'wireguard' && 'WireGuard VPN'}
              {formData.access_mode === 'public_https' && 'Public HTTPS'}
            </Badge>
          </div>

          {formData.access_mode === 'wireguard' && (
            <div className="space-y-2">
              <h4 className="font-medium">WireGuard settings</h4>
              <dl className="grid grid-cols-2 gap-2 text-sm">
                <dt className="text-muted-foreground">Subnet:</dt>
                <dd>{formData.wireguard.cidr}</dd>
                <dt className="text-muted-foreground">Port:</dt>
                <dd>{formData.wireguard.listen_port}</dd>
                <dt className="text-muted-foreground">Endpoint:</dt>
                <dd>{formData.wireguard.endpoint_hostname || 'Auto-detect'}</dd>
              </dl>
            </div>
          )}

          {formData.access_mode === 'public_https' && (
            <div className="space-y-2">
              <h4 className="font-medium">HTTPS settings</h4>
              <dl className="grid grid-cols-2 gap-2 text-sm">
                <dt className="text-muted-foreground">Domain:</dt>
                <dd>{formData.https.domain}</dd>
                <dt className="text-muted-foreground">Email:</dt>
                <dd>{formData.https.email}</dd>
                <dt className="text-muted-foreground">Method:</dt>
                <dd>Let's Encrypt (HTTP challenge)</dd>
              </dl>
            </div>
          )}

          <Alert>
            <OkIcon className="h-4 w-4" />
            <AlertDescription>
              Ready to apply configuration. The system will configure firewall rules,
              {formData.access_mode === 'wireguard' && ' enable WireGuard VPN,'}
              {formData.access_mode === 'public_https' && ' configure HTTPS with Let\'s Encrypt,'}
              {' '}and restart required services.
            </AlertDescription>
          </Alert>
        </CardContent>
      </Card>
    </div>
  );

  const renderStepContent = () => {
    switch (currentStep) {
      case 1:
        return renderAccessModeStep();
      case 2:
        return renderConfigurationStep();
      case 3:
        return renderSecurityStep();
      case 4:
        return renderReviewStep();
      default:
        return null;
    }
  };

  return (
    <div className="container mx-auto p-6 max-w-4xl">
      <div className="mb-8">
        <h1 className="screen-title">Remote access wizard</h1>
        <p className="text-muted-foreground mt-2">
          Configure how you access nOS from outside your local network
        </p>
      </div>

      {/* Progress indicator */}
      <div className="mb-8">
        <Progress value={(currentStep / 4) * 100} className="mb-4" />
        <div className="flex justify-between">
          {WIZARD_STEPS.map((step) => {
            const Icon = step.icon;
            return (
              <div
                key={step.id}
                className={`flex flex-col items-center ${
                  step.id <= currentStep ? 'text-primary' : 'text-muted-foreground'
                }`}
              >
                <div
                  className={`rounded-full p-2 ${
                    step.id <= currentStep ? 'bg-primary/10' : 'bg-muted'
                  }`}
                >
                  <Icon className="h-5 w-5" />
                </div>
                <span className="text-xs mt-1">{step.title}</span>
              </div>
            );
          })}
        </div>
      </div>

      {/* Step content */}
      <div className="mb-8">
        {wizardState.data?.error && (
          <Alert variant="destructive" className="mb-4">
            <WarningIcon className="h-4 w-4" />
            <AlertDescription>{wizardState.data.error}</AlertDescription>
          </Alert>
        )}
        
        {renderStepContent()}
      </div>

      {/* Navigation buttons */}
      <div className="flex justify-between">
        <Button
          variant="outline"
          onClick={handleBack}
          disabled={currentStep === 1}
        >
          <ChevronLeftIcon className="h-4 w-4 mr-2" />
          Back
        </Button>

        {currentStep < 4 ? (
          <Button
            onClick={handleNext}
            disabled={wizardNext.isPending}
          >
            {wizardNext.isPending && <WorkingIcon className="h-4 w-4 mr-2 animate-spin" />}
            Next
            <ChevronRightIcon className="h-4 w-4 ml-2" />
          </Button>
        ) : (
          <Button
            onClick={handleComplete}
            disabled={completeWizard.isPending}
          >
            {completeWizard.isPending && <WorkingIcon className="h-4 w-4 mr-2 animate-spin" />}
            Apply Configuration
          </Button>
        )}
      </div>
    </div>
  );
}
