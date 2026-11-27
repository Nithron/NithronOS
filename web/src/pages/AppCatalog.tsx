import { useState, useMemo } from 'react';
import { useQuery, useQueryClient, useMutation } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { motion, AnimatePresence } from 'framer-motion';
import { AppIcon, ChevronRightIcon, DownloadIcon, FailedIcon, NoticeIcon, OkIcon, OpenExternalIcon, RestartIcon, SearchIcon, SecurityIcon, StartIcon, StopIcon, WorkingIcon } from '@/components/icons'
import { appsApi } from '../api/apps';
import type { CatalogEntry, InstalledApp } from '../api/apps.types';
import { cn } from '../lib/utils';
import { StatusMark } from '@/components/ui/pixel';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card-enhanced';
import { toast } from '@/components/ui/toast';
import { Badge } from '@/components/ui/badge';

// Animation variants
const containerVariants: any = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0.05
    }
  }
};

const itemVariants: any = {
  hidden: { opacity: 0, y: 20 },
  visible: {
    opacity: 1,
    y: 0,
    transition: {
      type: "spring",
      stiffness: 100
    }
  }
};

export function AppCatalog() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [activeTab, setActiveTab] = useState<'catalog' | 'installed'>('catalog');
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string | null>(null);
  const [officialOnly, setOfficialOnly] = useState(false);
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid');
  const [isRefreshing, setIsRefreshing] = useState(false);

  // Fetch catalog
  const { data: catalog, isLoading: catalogLoading } = useQuery({
    queryKey: ['apps', 'catalog'],
    queryFn: appsApi.getCatalog,
    refetchInterval: 60000, // Refresh every minute
  });

  // Fetch installed apps
  const { data: installedData, isLoading: installedLoading } = useQuery({
    queryKey: ['apps', 'installed'],
    queryFn: appsApi.getInstalledApps,
    refetchInterval: 10000, // Refresh every 10 seconds
  });

  // Sync catalogs mutation
  const syncCatalogsMutation = useMutation({
    mutationFn: async () => {
      setIsRefreshing(true);
      // Call sync API
      await appsApi.syncCatalogs();
      // Wait a bit for sync to complete
      await new Promise(resolve => setTimeout(resolve, 2000));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['apps', 'catalog'] });
      toast.success('Catalogs synced successfully');
      setIsRefreshing(false);
    },
    onError: (error) => {
      toast.error('Failed to sync catalogs');
      console.error('Sync error:', error);
      setIsRefreshing(false);
    }
  });

  const installedApps = installedData?.items || [];
  const installedAppIds = new Set(installedApps.map((app: InstalledApp) => app.id));

  // Get unique categories
  const categories = useMemo(() => {
    if (!catalog) return [];
    const cats = new Set<string>();
    catalog.entries.forEach((app: CatalogEntry) => {
      app.categories?.forEach((cat: string) => cats.add(cat));
    });
    return Array.from(cats).sort();
  }, [catalog]);

  // Filter catalog entries
  const filteredEntries = useMemo(() => {
    if (!catalog) return [];
    
    return catalog.entries.filter((app: CatalogEntry) => {
      // Search filter
      if (searchQuery) {
        const query = searchQuery.toLowerCase();
        if (
          !app.name.toLowerCase().includes(query) &&
          !app.description.toLowerCase().includes(query) &&
          !app.id.toLowerCase().includes(query)
        ) {
          return false;
        }
      }

      // Category filter
      if (selectedCategory && !app.categories?.includes(selectedCategory)) {
        return false;
      }

      // Official filter
      if (officialOnly && !(app as any).verified) {
        return false;
      }

      return true;
    });
  }, [catalog, searchQuery, selectedCategory, officialOnly]);

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'running':
        return 'text-status-ok-text';
      case 'stopped':
        return 'text-ink-2';
      case 'error':
      case 'unhealthy':
        return 'text-status-failed-text';
      case 'starting':
      case 'stopping':
      case 'upgrading':
        return 'text-status-warning-text';
      default:
        return 'text-ink-2';
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'running':
        return <OkIcon className="w-4 h-4" />;
      case 'stopped':
        return <FailedIcon className="w-4 h-4" />;
      case 'error':
      case 'unhealthy':
        return <FailedIcon className="w-4 h-4" />;
      case 'starting':
      case 'stopping':
      case 'upgrading':
        return <WorkingIcon className="w-4 h-4 animate-spin" />;
      default:
        return null;
    }
  };

  const handleAppAction = async (appId: string, action: 'start' | 'stop' | 'restart') => {
    try {
      switch (action) {
        case 'start':
          await appsApi.startApp(appId);
          break;
        case 'stop':
          await appsApi.stopApp(appId);
          break;
        case 'restart':
          await appsApi.restartApp(appId);
          break;
      }
      await queryClient.invalidateQueries({ queryKey: ['apps', 'installed'] });
      toast.success(`App ${action}ed successfully`);
    } catch (error) {
      console.error(`Failed to ${action} app:`, error);
      toast.error(`Failed to ${action} app`);
    }
  };

  const handleInstall = (appId: string) => {
    navigate(`/apps/install/${appId}`);
  };

  const AppCard = ({ app, isInstalled }: { app: CatalogEntry; isInstalled: boolean }) => {
    
    return (
      <motion.div variants={itemVariants}>
        <Card className="h-full overflow-hidden">
          <div className="relative flex h-full flex-col">
            {/* The app's name first; its states in words under it. */}
            {((app as any).verified || isInstalled) && (
              <div className="flex items-center gap-3 px-4 pt-4 text-sm">
                {isInstalled && <StatusMark state="ok">Installed</StatusMark>}
                {(app as any).verified && <span className="text-ink-2">Verified</span>}
              </div>
            )}

            {/* App Info; the version and the button stand at the bottom of the cell. */}
            <div className="flex flex-1 flex-col gap-3 p-4">
              <div>
                <h3 className="font-[560] text-lg line-clamp-1">{app.name}</h3>
                <p className="text-sm text-muted-foreground line-clamp-2 mt-1">
                  {app.description}
                </p>
              </div>
              
              {/* Categories */}
              <div className="flex flex-wrap gap-1">
                {app.categories?.map((cat) => (
                  <Badge key={cat} variant="secondary" className="text-xs">
                    {cat}
                  </Badge>
                ))}
              </div>
              
              {/* Version and Size */}
              <div className="mt-auto flex items-center justify-between font-mono text-xs text-ink-2">
                <span>{/^\d/.test(app.version) ? `v${app.version}` : app.version}</span>
                {(app as any).repository && (
                  <a
                    href={(app as any).repository}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-center underline decoration-ink-2 underline-offset-4 hover:text-ink"
                  >
                    <OpenExternalIcon className="w-3 h-3 mr-1" />
                    Source
                  </a>
                )}
              </div>
              
              {/* Action Button */}
              <Button
                className="w-full"
                variant="outline"
                onClick={() => isInstalled ? navigate(`/apps/${app.id}`) : handleInstall(app.id)}
              >
                {isInstalled ? (
                  <>
                    <NoticeIcon className="w-4 h-4 mr-2" />
                    Manage
                  </>
                ) : (
                  <>
                    <DownloadIcon className="w-4 h-4 mr-2" />
                    Install
                  </>
                )}
              </Button>
            </div>
          </div>
        </Card>
      </motion.div>
    );
  };

  const InstalledAppCard = ({ app }: { app: InstalledApp }) => {
    
    return (
      <motion.div variants={itemVariants}>
        <Card className="h-full overflow-hidden">
          <div className="relative">
            {/* Status Bar */}
            <div className="border-b border-hairline px-4 py-3 text-sm">
              <StatusMark
                state={app.status === 'running' ? 'ok' : app.status === 'error' ? 'failed' : app.status === 'starting' || app.status === 'stopping' ? 'running' : 'off'}
              >
                {app.status}
              </StatusMark>
            </div>
            
            {/* App Content */}
            <div className="p-4 space-y-3">
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-3">
                  <div>
                    <h3 className="font-[560] text-lg">{app.name}</h3>
                    <div className={cn("flex items-center gap-1 text-sm", getStatusColor(app.status))}>
                      {getStatusIcon(app.status)}
                      <span className="capitalize">{app.status}</span>
                    </div>
                  </div>
                </div>
              </div>
              
              {/* App Info */}
              <div className="space-y-2 text-sm text-muted-foreground">
                <div className="flex items-center justify-between">
                  <span>Version</span>
                  <span className="font-medium text-foreground">{app.version}</span>
                </div>
                {app.ports && app.ports.length > 0 && (
                  <div className="flex items-center justify-between">
                    <span>Ports</span>
                    <span className="font-medium text-foreground">
                      {app.ports.join(', ')}
                    </span>
                  </div>
                )}
              </div>
              
              {/* Action Buttons */}
              <div className="flex gap-2">
                {app.status === 'stopped' ? (
                  <Button
                    size="sm"
                    variant="outline"
                    className="flex-1"
                    onClick={() => handleAppAction(app.id, 'start')}
                  >
                    <StartIcon className="w-4 h-4 mr-1" />
                    Start
                  </Button>
                ) : app.status === 'running' ? (
                  <>
                    <Button
                      size="sm"
                      variant="outline"
                      className="flex-1"
                      onClick={() => handleAppAction(app.id, 'stop')}
                    >
                      <StopIcon className="w-4 h-4 mr-1" />
                      Stop
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      className="flex-1"
                      onClick={() => handleAppAction(app.id, 'restart')}
                    >
                      <RestartIcon className="w-4 h-4 mr-1" />
                      Restart
                    </Button>
                  </>
                ) : null}
                <Button
                  size="sm"
                  variant="default"
                  onClick={() => navigate(`/apps/${app.id}`)}
                >
                  <ChevronRightIcon className="w-4 h-4" />
                </Button>
              </div>
            </div>
          </div>
        </Card>
      </motion.div>
    );
  };

  return (
    <div className="container mx-auto py-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div>
            <h1 className="screen-title">App catalog</h1>
          </div>
        </div>
        <Button
          onClick={() => syncCatalogsMutation.mutate()}
          disabled={isRefreshing || syncCatalogsMutation.isPending}
          variant="outline"
        >
          <RestartIcon className={cn(
            "w-4 h-4 mr-2",
            (isRefreshing || syncCatalogsMutation.isPending) && "animate-spin"
          )} />
          Sync catalogs
        </Button>
      </div>

      {/* Catalog or installed: the shared row of words. The view is a
          radio frame of two words. */}
      <div className="flex items-end justify-between gap-4">
        <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v as typeof activeTab)} className="w-auto flex-1">
          <TabsList>
            <TabsTrigger value="catalog">Catalog</TabsTrigger>
            <TabsTrigger value="installed">
              Installed{installedApps.length > 0 && <span className="ml-2 font-mono text-ink-2">{installedApps.length}</span>}
            </TabsTrigger>
          </TabsList>
        </Tabs>
        <div role="radiogroup" aria-label="View" className="inline-flex divide-x divide-rule overflow-hidden rounded-lg border border-rule text-sm">
          {(['grid', 'list'] as const).map((m) => (
            <button
              key={m}
              type="button"
              role="radio"
              aria-checked={viewMode === m}
              onClick={() => setViewMode(m)}
              className={cn('min-h-8 px-3', viewMode === m ? 'bg-ink text-page' : 'hover:bg-ink/[.09]')}
            >
              {m === 'grid' ? 'Grid' : 'List'}
            </button>
          ))}
        </div>
      </div>

      {/* Filters */}
      {activeTab === 'catalog' && (
        <div className="space-y-4">
          {/* Search and toggles */}
          <div className="flex gap-4 items-center">
            <div className="relative flex-1 max-w-md">
              <SearchIcon className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Search apps..."
                className="w-full pl-10 pr-4 py-2 bg-background border rounded-lg focus:outline-none focus:ring-2 focus:ring-primary"
              />
            </div>
            
            <Button
              variant={officialOnly ? "default" : "outline"}
              size="sm"
              onClick={() => setOfficialOnly(!officialOnly)}
            >
              <SecurityIcon className="w-4 h-4 mr-2" />
              Verified only
            </Button>
          </div>

          {/* Category Pills */}
          {categories.length > 0 && (
            <div className="flex flex-wrap gap-2">
              <button
                onClick={() => setSelectedCategory(null)}
                aria-pressed={selectedCategory === null}
                className={cn(
                  'min-h-8 border px-3 text-sm',
                  selectedCategory === null ? 'border-ink bg-ink text-page' : 'border-rule hover:border-ink'
                )}
              >
                All categories
              </button>
              {categories.map((cat) => {
                return (
                  <button
                    key={cat}
                    onClick={() => setSelectedCategory(cat)}
                    aria-pressed={selectedCategory === cat}
                    className={cn(
                      'flex min-h-8 items-center gap-2 border px-3 text-sm',
                      selectedCategory === cat ? 'border-ink bg-ink text-page' : 'border-rule hover:border-ink'
                    )}
                  >
                    {cat}
                  </button>
                );
              })}
            </div>
          )}
        </div>
      )}

      {/* Content */}
      <AnimatePresence mode="wait">
        {activeTab === 'catalog' ? (
          <motion.div
            key="catalog"
            initial={false}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
          >
            {catalogLoading ? (
              <div className="flex items-center justify-center py-20">
                <WorkingIcon className="w-8 h-8 animate-spin text-muted-foreground" />
              </div>
            ) : filteredEntries.length === 0 ? (
              <Card className="p-12 text-center">
                <AppIcon className="w-16 h-16 mx-auto text-muted-foreground mb-4" />
                <h3 className="text-lg font-[560] mb-2">No apps found</h3>
                <p className="text-muted-foreground">
                  {searchQuery || selectedCategory
                    ? 'Try adjusting your filters'
                    : 'No apps available in the catalog'}
                </p>
              </Card>
            ) : (
              <motion.div
                variants={containerVariants}
                initial={false}
                animate="visible"
                className={cn(
                  viewMode === 'grid'
                    ? "grid border-l border-t border-hairline md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 [&>*]:border-b [&>*]:border-r [&>*]:border-hairline [&>*>*]:h-full [&>*>*]:border-0"
                    : "space-y-4"
                )}
              >
                {filteredEntries.map((app) => (
                  <AppCard
                    key={app.id}
                    app={app}
                    isInstalled={installedAppIds.has(app.id)}
                  />
                ))}
              </motion.div>
            )}
          </motion.div>
        ) : (
          <motion.div
            key="installed"
            initial={false}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
          >
            {installedLoading ? (
              <div className="flex items-center justify-center py-20">
                <WorkingIcon className="w-8 h-8 animate-spin text-muted-foreground" />
              </div>
            ) : installedApps.length === 0 ? (
              <Card className="p-12 text-center">
                <AppIcon className="w-16 h-16 mx-auto text-muted-foreground mb-4" />
                <h3 className="text-lg font-[560] mb-2">No apps installed</h3>
                <p className="text-muted-foreground mb-6">
                  Install apps from the catalog to get started
                </p>
                <Button onClick={() => setActiveTab('catalog')}>
                  Browse catalog
                </Button>
              </Card>
            ) : (
              <motion.div
                variants={containerVariants}
                initial={false}
                animate="visible"
                className={cn(
                  viewMode === 'grid'
                    ? "grid border-l border-t border-hairline md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 [&>*]:border-b [&>*]:border-r [&>*]:border-hairline [&>*>*]:h-full [&>*>*]:border-0"
                    : "space-y-4"
                )}
              >
                {installedApps.map((app) => (
                  <InstalledAppCard key={app.id} app={app} />
                ))}
              </motion.div>
            )}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}