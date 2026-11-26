package server

import (
	"net/http"
	"os"
	"path/filepath"

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

	// Initialize backup handler (using existing implementation)
	// The existing backup handler requires scheduler, replicator, and restorer
	// For now, we'll skip initializing it as it needs more complex setup
	var backupHandler *BackupHandler

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
