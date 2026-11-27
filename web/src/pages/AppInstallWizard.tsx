import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery, useMutation } from '@tanstack/react-query';
import { AppIcon, ChevronLeftIcon, ChevronRightIcon, ChosenIcon, FailedIcon, HideIcon, NoticeIcon, SecurityIcon, SettingsIcon, ShowIcon, StorageIcon, WarningIcon, WorkingIcon } from '@/components/icons'
import { appsApi } from '../api/apps';
import type { CatalogEntry, JsonSchemaProperty, PortMapping, VolumeMount } from '../api/apps.types';
import { cn } from '../lib/utils';
import { StatusMark } from '@/components/ui/pixel';
import { toast } from '@/components/ui/toast';

interface FormData {
  [key: string]: any;
}

interface FormErrors {
  [key: string]: string;
}

export function AppInstallWizard() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [currentStep, setCurrentStep] = useState(0);
  const [formData, setFormData] = useState<FormData>({});
  const [formErrors, setFormErrors] = useState<FormErrors>({});
  const [showPasswords, setShowPasswords] = useState<Record<string, boolean>>({});

  // Fetch catalog
  const { data: catalog } = useQuery({
    queryKey: ['apps', 'catalog'],
    queryFn: appsApi.getCatalog,
  });

  const app = catalog?.entries.find((entry: CatalogEntry) => entry.id === id);

  // The app's install form (its template's schema.json).
  const { data: schema = null } = useQuery({
    queryKey: ['apps', 'schema', id],
    queryFn: () => appsApi.getSchema(id!),
    enabled: !!app?.id,
  });

  useEffect(() => {
    if (!app || !schema) return;
    // Defaults: the schema's, then the catalog entry's env.
    const defaults: FormData = {};
    Object.entries(schema.properties || {}).forEach(([key, prop]: [string, any]) => {
      if (prop.default !== undefined) {
        defaults[key] = prop.default;
      }
    });
    if (app.defaults?.env) {
      Object.entries(app.defaults.env).forEach(([key, value]) => {
        if (defaults[key] === undefined) {
          defaults[key] = value;
        }
      });
    }
    setFormData(defaults);
  }, [app, schema]);

  // Install mutation
  const installMutation = useMutation({
    mutationFn: (params: FormData) => 
      appsApi.installApp({
        id: id!,
        params
      }),
    onSuccess: () => {
      toast.success(`${app?.name} installed successfully!`);
      navigate(`/apps/${id}`);
    },
    onError: (error: any) => {
      toast.error(`Failed to install app: ${error.message}`);
    }
  });

  const steps = [
    { title: 'Overview', icon: NoticeIcon },
    { title: 'Configuration', icon: SettingsIcon },
    { title: 'Storage', icon: StorageIcon },
    { title: 'Review & install', icon: ChosenIcon }
  ];

  const validateForm = (): boolean => {
    const errors: FormErrors = {};
    
    if (schema?.properties) {
      Object.entries(schema.properties).forEach(([key, prop]: [string, any]) => {
        const value = formData[key];
        
        // Required check
        if (schema.required?.includes(key) && !value) {
          errors[key] = 'This field is required';
          return;
        }
        
        // Type validation
        if (value !== undefined && value !== '') {
          // Pattern validation
          if (prop.pattern) {
            const regex = new RegExp(prop.pattern);
            if (!regex.test(value)) {
              errors[key] = `Invalid format`;
            }
          }
          
          // Length validation
          if (prop.minLength && value.length < prop.minLength) {
            errors[key] = `Must be at least ${prop.minLength} characters`;
          }
          if (prop.maxLength && value.length > prop.maxLength) {
            errors[key] = `Must be at most ${prop.maxLength} characters`;
          }
        }
      });
    }
    
    setFormErrors(errors);
    return Object.keys(errors).length === 0;
  };

  const handleNext = () => {
    if (currentStep === 1) {
      // Validate configuration step
      if (!validateForm()) {
        toast.error('Please fix the form errors');
        return;
      }
    }
    
    if (currentStep < steps.length - 1) {
      setCurrentStep(currentStep + 1);
    }
  };

  const handleBack = () => {
    if (currentStep > 0) {
      setCurrentStep(currentStep - 1);
    }
  };

  const handleInstall = () => {
    if (!validateForm()) {
      toast.error('Please fix the form errors');
      return;
    }
    
    installMutation.mutate(formData);
  };

  const renderFormField = (key: string, prop: JsonSchemaProperty) => {
    const value = formData[key] || '';
    const error = formErrors[key];
    const isPassword = prop.format === 'password';
    const showPassword = showPasswords[key];
    
    return (
      <div key={key} className="space-y-2">
        <label className="block">
          <span className="text-sm font-medium">{prop.title || key}</span>
          {schema?.required?.includes(key) && (
            <span className="text-status-failed-text ml-1">*</span>
          )}
        </label>
        
        {prop.enum ? (
          <select
            value={value}
            onChange={(e) => setFormData({ ...formData, [key]: e.target.value })}
            className={cn('input w-full', error && 'border-status-failed')}
          >
            <option value="">Select...</option>
            {prop.enum.map(opt => (
              <option key={opt} value={opt}>{opt}</option>
            ))}
          </select>
        ) : prop.type === 'boolean' ? (
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={value === true || value === 'true'}
              onChange={(e) => setFormData({ ...formData, [key]: e.target.checked })}
              className="checkbox"
            />
            <span className="text-sm text-ink-2">{prop.description}</span>
          </label>
        ) : (
          <div className="relative">
            <input
              type={isPassword && !showPassword ? 'password' : 'text'}
              value={value}
              onChange={(e) => setFormData({ ...formData, [key]: e.target.value })}
              placeholder={prop.examples?.[0] || ''}
              className={cn('input w-full', error && 'border-status-failed', isPassword && 'pr-10')}
            />
            {isPassword && (
              <button
                type="button"
                onClick={() => setShowPasswords({ ...showPasswords, [key]: !showPassword })}
                className="absolute right-2 top-1/2 -translate-y-1/2 text-ink-2 hover:text-ink"
              >
                {showPassword ? <HideIcon className="w-4 h-4" /> : <ShowIcon className="w-4 h-4" />}
              </button>
            )}
          </div>
        )}
        
        {prop.description && !error && (
          <p className="text-xs text-ink-2">{prop.description}</p>
        )}
        {error && (
          <p className="text-xs text-status-failed-text">{error}</p>
        )}
      </div>
    );
  };

  if (!app) {
    return (
      <div className="container mx-auto py-6">
        <div className="text-center py-12">
          <FailedIcon className="w-12 h-12 text-status-failed-text mx-auto mb-4" />
          <p className="text-lg">App not found</p>
          <button
            onClick={() => navigate('/apps')}
            className="btn btn-primary mt-4"
          >
            Back to catalog
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="container mx-auto py-6 max-w-4xl">
      {/* Header */}
      <div className="flex items-center gap-4 mb-6">
        <button
          onClick={() => navigate('/apps')}
          className="btn btn-secondary btn-sm"
        >
          <ChevronLeftIcon className="w-4 h-4" />
        </button>
        <AppIcon className="w-8 h-8 text-ink" />
        <div>
          <h1 className="screen-title">Install {app.name}</h1>
          <p className="text-sm text-ink-2">{app.description}</p>
        </div>
      </div>

      {/* Steps */}
      <div className="flex items-center justify-between mb-8">
        {steps.map((step, index) => (
          <div
            key={index}
            className="flex items-center"
          >
            <StatusMark
              state={index < currentStep ? 'ok' : index === currentStep ? 'running' : 'off'}
              className={cn('text-sm', index === currentStep ? 'font-[560] text-ink' : 'text-ink-2')}
            >
              {step.title}
            </StatusMark>
          </div>
        ))}
      </div>

      {/* Content */}
      <div className="border border-hairline p-6 min-h-[400px]">
        {currentStep === 0 && (
          <div className="space-y-6">
            <div>
              <h2 className="text-xl font-[560] mb-4">About {app.name}</h2>
              <p className="text-ink mb-4">{app.description}</p>
              {app.notes && (
                <div className="border border-hairline p-4">
                  <p className="text-sm text-ink">{app.notes}</p>
                </div>
              )}
            </div>
            
            <div>
              <h3 className="text-lg font-[560] mb-3">Requirements</h3>
              <div className="space-y-2">
                {app.defaults?.resources?.cpu_limit && (
                  <div className="flex items-center gap-2">
                    <SecurityIcon className="w-4 h-4 text-ink-2" />
                    <span className="text-sm">CPU: up to {app.defaults.resources.cpu_limit} cores</span>
                  </div>
                )}
                {app.defaults?.resources?.memory_limit && (
                  <div className="flex items-center gap-2">
                    <SecurityIcon className="w-4 h-4 text-ink-2" />
                    <span className="text-sm">Memory: up to {app.defaults.resources.memory_limit}</span>
                  </div>
                )}
                {app.defaults?.ports?.length > 0 && (
                  <div className="flex items-center gap-2">
                    <SecurityIcon className="w-4 h-4 text-ink-2" />
                    <span className="text-sm">
                      Ports: {app.defaults.ports.map((p: PortMapping) => p.host).join(', ')}
                    </span>
                  </div>
                )}
                {app.needs_privileged && (
                  <div className="flex items-center gap-2">
                    <WarningIcon className="w-4 h-4 text-status-warning-text" />
                    <span className="text-sm text-status-warning-text">
                      Requires privileged access
                    </span>
                  </div>
                )}
              </div>
            </div>
          </div>
        )}

        {currentStep === 1 && (
          <div className="space-y-6">
            <h2 className="text-xl font-[560]">Configuration</h2>
            {schema?.properties && Object.keys(schema.properties).length > 0 ? (
              <div className="space-y-4">
                {Object.entries(schema.properties).map(([key, prop]: [string, any]) =>
                  renderFormField(key, prop)
                )}
              </div>
            ) : (
              <p className="text-ink-2">No configuration required</p>
            )}
          </div>
        )}

        {currentStep === 2 && (
          <div className="space-y-6">
            <h2 className="text-xl font-[560]">Storage configuration</h2>
            
            <div className="border border-hairline p-4">
              <h3 className="font-medium mb-2">Data directory</h3>
              <code className="text-sm text-ink">/srv/apps/{app.id}/data</code>
              <p className="text-sm text-ink-2 mt-2">
                App data will be stored here. This directory will be automatically created
                and backed by Btrfs snapshots if available.
              </p>
            </div>
            
            {app.defaults?.volumes?.length > 0 && (
              <div>
                <h3 className="font-medium mb-3">Volume mounts</h3>
                <div className="space-y-2">
                  {app.defaults.volumes.map((vol: VolumeMount, idx: number) => (
                    <div key={idx} className="border border-hairline p-3">
                      <div className="flex items-center justify-between">
                        <span className="text-sm font-mono">{vol.container}</span>
                        <span className="text-xs text-ink-2">
                          {vol.read_only ? 'Read-only' : 'Read-write'}
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
            
            <div className="border border-hairline p-4">
              <div className="flex items-start gap-3">
                <NoticeIcon className="w-5 h-5 text-ink flex-shrink-0 mt-0.5" />
                <div className="text-sm">
                  <p className="text-ink font-medium mb-1">Automatic snapshots</p>
                  <p className="text-ink-2">
                    Snapshots will be created before any app updates or configuration changes,
                    allowing easy rollback if needed.
                  </p>
                </div>
              </div>
            </div>
          </div>
        )}

        {currentStep === 3 && (
          <div className="space-y-6">
            <h2 className="text-xl font-[560]">Review & install</h2>
            
            <div className="space-y-4">
              <div className="border border-hairline p-4">
                <h3 className="font-medium mb-3">Configuration summary</h3>
                <div className="space-y-2">
                  {Object.entries(formData).map(([key, value]) => (
                    <div key={key} className="flex items-center justify-between text-sm">
                      <span className="text-ink-2">{key}:</span>
                      <span className="font-mono">
                        {typeof value === 'string' && value.length > 20
                          ? value.substring(0, 20) + '...'
                          : String(value)}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
              
              <div className="border border-status-warning p-4">
                <div className="flex items-start gap-3">
                  <NoticeIcon className="w-5 h-5 text-status-warning-text flex-shrink-0 mt-0.5" />
                  <div className="text-sm">
                    <p className="text-status-warning-text font-medium mb-1">Ready to install</p>
                    <p className="text-ink-2">
                      The app will be installed with the configuration shown above.
                      Docker images will be downloaded and the app will be started automatically.
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Actions */}
      <div className="flex items-center justify-between mt-6">
        <button
          onClick={handleBack}
          disabled={currentStep === 0}
          className="btn btn-secondary"
        >
          <ChevronLeftIcon className="w-4 h-4 mr-2" />
          Back
        </button>
        
        {currentStep < steps.length - 1 ? (
          <button
            onClick={handleNext}
            className="btn btn-primary"
          >
            Next
            <ChevronRightIcon className="w-4 h-4 ml-2" />
          </button>
        ) : (
          <button
            onClick={handleInstall}
            disabled={installMutation.isPending}
            className="btn btn-primary"
          >
            {installMutation.isPending ? (
              <>
                <WorkingIcon className="w-4 h-4 mr-2 animate-spin" />
                Installing...
              </>
            ) : (
              <>
                <ChosenIcon className="w-4 h-4 mr-2" />
                Install app
              </>
            )}
          </button>
        )}
      </div>
    </div>
  );
}
