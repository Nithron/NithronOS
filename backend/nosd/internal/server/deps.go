package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"

	"nos/backend/nosd/internal/apps"
	"nos/backend/nosd/internal/auth/session"
	userstore "nos/backend/nosd/internal/auth/store"
	"nos/backend/nosd/internal/config"
	"nos/backend/nosd/internal/groups"
	"nos/backend/nosd/internal/notifications"
	"nos/backend/nosd/internal/policy"
	"nos/backend/nosd/internal/ratelimit"
	"nos/backend/nosd/internal/setup/firstboot"
	"nos/backend/nosd/internal/shares"
	"nos/backend/nosd/internal/updates"
	"nos/backend/nosd/pkg/agentclient"
	"nos/backend/nosd/pkg/alerts"
	"nos/backend/nosd/pkg/backup"
	"nos/backend/nosd/pkg/monitor"
)

// routeDeps holds what the route files share. It is built once per router.
// A nil service means it failed to start; its routes answer 503.
type routeDeps struct {
	cfg                 config.Config
	users               *userstore.Store
	agentClient         *agentclient.Client
	sharesStore         *shares.Store
	sharesHandler       *SharesHandler
	groupsHandler       *GroupsHandler
	policyHandler       *PolicyHandler
	accounts            accounts
	groupsStore         *groups.Store
	sharesApplier       *sharesApplier
	updates             *updates.Service
	monitor             *MonitorHandler
	backupStart         func(ctx context.Context)
	monitorStart        func(ctx context.Context)
	backupHandler       *BackupHandler
	notificationManager *notifications.Manager
	appsManager         *apps.Manager
	rlStore             *ratelimit.Store
	mgr                 *session.Manager
	fb                  *firstboot.Manager
}

func newRouteDeps(cfg config.Config) *routeDeps {
	// Init stores
	users, _ := userstore.New(cfg.UsersPath)
	InitJobsStore(cfg)

	agentClient := agentclient.New(cfg.AgentSocket())
	etcDir := filepath.Dir(cfg.UsersPath)
	// One shares store for the shares API, NithronSync and WebDAV; groups and
	// the policy next to it. A store that can't be read leaves its API
	// answering 503 (nothing is overwritten).
	sharesStore, err := shares.NewStore(filepath.Join(etcDir, "shares.json"))
	if err != nil {
		log.Error().Err(err).Msg("Failed to load the shares store")
		sharesStore = nil
	}
	groupsStore, err := groups.NewStore(filepath.Join(etcDir, "groups.json"))
	if err != nil {
		log.Error().Err(err).Msg("Failed to load the groups store")
		groupsStore = nil
	}
	policyStore, err := policy.NewStore(filepath.Join(etcDir, "policy.json"))
	if err != nil {
		log.Error().Err(err).Msg("Failed to load the policy")
		policyStore = nil
	}
	acc := accounts{users: users, groups: groupsStore}
	monitorHandler, monitorStart := newMonitor(cfg, etcDir)
	updatesSvc, err := updates.New(agentClient,
		filepath.Join(filepath.Dir(cfg.SessionsPath), "updates"),
		filepath.Join(etcDir, "update", "channel"))
	if err != nil {
		log.Error().Err(err).Msg("Failed to load the update state")
		updatesSvc = nil
	}
	var (
		applier       *sharesApplier
		sharesHandler *SharesHandler
		groupsHandler *GroupsHandler
		policyHandler *PolicyHandler
	)
	if policyStore != nil {
		policyHandler = &PolicyHandler{store: policyStore}
	}
	if sharesStore != nil && groupsStore != nil && policyStore != nil {
		applier = &sharesApplier{shares: sharesStore, groups: groupsStore, users: acc, agent: agentClient}
		sharesHandler = &SharesHandler{store: sharesStore, groups: groupsStore, users: acc, policy: policyStore, applier: applier}
		groupsHandler = &GroupsHandler{store: groupsStore, shares: sharesStore, users: acc, applier: applier}
	}

	// Backups: scheduled snapshots, replication and restore, all through
	// nos-agent. (The handler used to be left nil, so none of the backup
	// API existed.)
	backupHandler, backupStart := newBackup(cfg, etcDir, agentClient)

	// Initialize notifications manager
	notificationsPath := filepath.Join(filepath.Dir(cfg.UsersPath), "notifications")
	notificationManager, err := notifications.NewManager(notificationsPath)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize notifications manager")
	}

	// Initialize apps manager
	appManagerConfig := &apps.Config{
		AppsRoot:      "/srv/apps",
		StateFile:     filepath.Join(filepath.Dir(cfg.UsersPath), "apps.json"),
		CatalogPath:   "/usr/share/nos/apps",
		CachePath:     "/var/lib/nos/apps/catalog.cache.json",
		SourcesPath:   "/etc/nos/apps/catalogs.d",
		TemplatesPath: "/usr/share/nos/apps",
		AgentPath:     cfg.AgentSocket(),
		CaddyPath:     "/etc/caddy/Caddyfile.d",
	}
	if v := os.Getenv("NOS_APPS_STATE"); v != "" {
		appManagerConfig.StateFile = v
	}
	appsManager, err := apps.NewManager(appManagerConfig)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize the app manager; /api/v1/apps answers 503")
		appsManager = nil
	}
	// Disk-backed session and ratelimit stores
	rlStore := ratelimit.New(cfg.RateLimitPath)
	mgr := session.New(cfg.SessionsPath)

	// The first-boot setup code is owned by one manager per state file; main
	// runs its rotation loop, these handlers use it on demand.
	fb := FirstBootManager(cfg)

	return &routeDeps{
		cfg: cfg, users: users, agentClient: agentClient,
		sharesStore: sharesStore, sharesHandler: sharesHandler, backupHandler: backupHandler,
		groupsHandler: groupsHandler, policyHandler: policyHandler, accounts: acc,
		groupsStore: groupsStore, sharesApplier: applier, updates: updatesSvc,
		monitor: monitorHandler, monitorStart: monitorStart, backupStart: backupStart,
		notificationManager: notificationManager, appsManager: appsManager,
		rlStore: rlStore, mgr: mgr, fb: fb,
	}
}

// adminRequired lets the request through only for a signed-in administrator.
func (d *routeDeps) adminRequired(next http.Handler) http.Handler {
	// Skip admin check in tests
	if os.Getenv("NOS_TEST_SKIP_AUTH") == "1" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := signedInUID(r)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		u, err := d.users.FindByID(uid)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if !hasRole(u.Roles, "admin") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// newMonitor builds the metrics collector, its storage and the alert
// engine. start runs them (only the daemon does; tests and route-dump just
// build the router). A storage that can't be opened leaves /monitor at 503.
func newMonitor(cfg config.Config, etcDir string) (*MonitorHandler, func(context.Context)) {
	logger := *Logger(cfg)
	storage, err := monitor.NewTimeSeriesStorage(logger, filepath.Join(filepath.Dir(cfg.SessionsPath), "monitor"))
	if err != nil {
		log.Error().Err(err).Msg("Failed to open the metrics storage")
		return nil, func(context.Context) {}
	}
	collector := monitor.NewCollector(logger, storage, 30*time.Second)
	engine := alerts.NewEngine(logger, filepath.Join(etcDir, "alerts"), collector, storage)
	start := func(ctx context.Context) {
		if err := collector.Start(ctx); err != nil {
			log.Error().Err(err).Msg("metrics collector")
		}
		if err := engine.Start(ctx); err != nil {
			log.Error().Err(err).Msg("alerts engine")
		}
	}
	return NewMonitorHandler(logger, collector, storage, engine), start
}

// newBackup builds the backup scheduler, replicator and restorer; start runs
// the schedules (the daemon only).
func newBackup(cfg config.Config, etcDir string, agent AgentClient) (*BackupHandler, func(context.Context)) {
	logger := *Logger(cfg)
	stateDir := filepath.Join(filepath.Dir(cfg.SessionsPath), "backup")
	host := backupAgent{agent: agent}
	scheduler := backup.NewScheduler(logger, filepath.Join(stateDir, "schedules.json"), host)
	jobs := scheduler.GetJobManager() // one job list for snapshots, replication and restore
	replicator := backup.NewReplicator(logger, filepath.Join(stateDir, "destinations.json"), filepath.Join(etcDir, "backup", "keys"), jobs)
	replicator.Connect(host, scheduler.ListSnapshots)
	restorer := backup.NewRestorer(logger, host, jobs, scheduler, replicator)
	start := func(ctx context.Context) {
		for _, dir := range []string{stateDir, filepath.Join(etcDir, "backup", "keys")} {
			if err := os.MkdirAll(dir, 0o700); err != nil {
				log.Error().Err(err).Str("dir", dir).Msg("backup state folder")
			}
		}
		if err := scheduler.Start(ctx); err != nil {
			log.Error().Err(err).Msg("backup scheduler")
		}
		if err := replicator.Start(); err != nil {
			log.Error().Err(err).Msg("backup replicator")
		}
	}
	return NewBackupHandler(logger, scheduler, replicator, restorer), start
}
