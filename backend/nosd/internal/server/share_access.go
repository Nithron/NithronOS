package server

import (
	"context"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"nos/backend/nosd/internal/api"
	userstore "nos/backend/nosd/internal/auth/store"
	"nos/backend/nosd/internal/groups"
	"nos/backend/nosd/internal/shares"
)

// accounts answers questions about nOS users and their groups for shares,
// NithronSync and WebDAV.
type accounts struct {
	users  *userstore.Store
	groups *groups.Store
}

// UserExists reports whether a nOS user has the username.
func (a accounts) UserExists(username string) bool {
	_, err := a.users.FindByUsername(username)
	return err == nil
}

// Usernames lists every nOS user.
func (a accounts) Usernames() ([]string, error) {
	list, err := a.users.List()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(list))
	for _, u := range list {
		out = append(out, u.Username)
	}
	sort.Strings(out) // stable for the agent (and tests)
	return out, nil
}

// ShareAccess reports what the user with id userID may do in sh, by the same
// rule as Samba and the folder ACLs (shares.Share.Access).
func (a accounts) ShareAccess(sh shares.Share, userID string) (read, write bool) {
	u, err := a.users.FindByID(userID)
	if err != nil {
		return false, false
	}
	return sh.Access(u.Username, a.groups.Of(u.Username))
}

// forgetUser takes a deleted user out of every group and every share's
// owners and readers, then applies the result. Otherwise a new user who later
// got the same name would inherit the access.
func (d *routeDeps) forgetUser(ctx context.Context, username string) {
	if d.groupsStore == nil || d.sharesStore == nil {
		return
	}
	if err := d.groupsStore.RemoveMember(username); err != nil {
		log.Error().Err(err).Str("user", username).Msg("remove deleted user from groups")
	}
	principal := shares.UserPrefix + username
	for _, sh := range d.sharesStore.List() {
		if !slices.Contains(sh.Owners, principal) && !slices.Contains(sh.Readers, principal) {
			continue
		}
		_, err := d.sharesStore.Modify(sh.ID, func(s *shares.Share) error {
			s.Owners = slices.DeleteFunc(s.Owners, func(p string) bool { return p == principal })
			s.Readers = slices.DeleteFunc(s.Readers, func(p string) bool { return p == principal })
			return nil
		})
		if err != nil {
			log.Error().Err(err).Str("user", username).Str("share", sh.Name).Msg("remove deleted user from share")
		}
	}
	if d.sharesApplier != nil {
		d.sharesApplier.Apply(ctx)
	}
}

// dashboardSources feeds the dashboard's shares and apps widgets from the
// real stores (one row per protocol a share is exported over).
func (d *routeDeps) dashboardSources() api.DashboardSources {
	src := api.DashboardSources{Disks: d.smart.get}
	if d.sharesStore != nil {
		src.Shares = func() []api.ShareInfo {
			out := []api.ShareInfo{}
			for _, sh := range d.sharesStore.List() {
				if sh.SMB != nil {
					out = append(out, api.ShareInfo{Name: sh.Name, Proto: "SMB", Path: sh.Path, State: activeOrDisabled(sh.SMB.Enabled)})
				}
				if sh.NFS != nil {
					out = append(out, api.ShareInfo{Name: sh.Name, Proto: "NFS", Path: sh.Path, State: activeOrDisabled(sh.NFS.Enabled)})
				}
			}
			return out
		}
	}
	if d.appsManager != nil {
		src.Apps = func() []api.AppInfo {
			out := []api.AppInfo{}
			for _, a := range d.appsManager.GetInstalledApps() {
				out = append(out, api.AppInfo{ID: a.ID, Name: a.Name, State: string(a.Status), Version: a.Version})
			}
			return out
		}
	}
	return src
}

func activeOrDisabled(on bool) string {
	if on {
		return "active"
	}
	return "disabled"
}

// smartCacheTTL is how often the dashboard's disk health is read again.
const smartCacheTTL = 5 * time.Minute

// smartCache holds the disks' SMART health; reading it takes seconds per
// disk (through nos-agent: smartctl needs root), so it is refreshed in the
// background and the dashboard shows the last result.
type smartCache struct {
	mu         sync.Mutex
	summary    api.DisksSummary
	at         time.Time
	refreshing bool
	collect    func(context.Context) []SMARTDevice
}

func (c *smartCache) get() api.DisksSummary {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.refreshing && time.Since(c.at) > smartCacheTTL {
		c.refreshing = true
		go c.refresh()
	}
	return c.summary
}

func (c *smartCache) refresh() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	devices := c.collect(ctx)
	s := api.DisksSummary{Total: len(devices), LastScanISO: time.Now().UTC().Format(time.RFC3339)}
	for _, d := range devices {
		switch d.Health {
		case "good":
			s.Healthy++
		case "warning":
			s.Warning++
		case "critical":
			s.Critical++
		default:
			s.Unknown++
		}
	}
	c.mu.Lock()
	c.summary, c.at, c.refreshing = s, time.Now(), false
	c.mu.Unlock()
}
