package server

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"nos/backend/nosd/internal/groups"
	"nos/backend/nosd/internal/shares"
)

// sharesAgent is the part of the agent client the share applier uses.
type sharesAgent interface {
	PostJSON(ctx context.Context, path string, body any, out any) error
}

// usernames lists every nOS user.
type usernames interface {
	Usernames() ([]string, error)
}

// sharesApplier sends the whole share and group state to nos-agent, which
// writes the Samba and NFS configuration, the Unix groups and the folder
// ACLs. It runs after every change and once at startup, so the host always
// matches the stores.
type sharesApplier struct {
	shares *shares.Store
	groups *groups.Store
	users  usernames
	agent  sharesAgent

	mu   sync.Mutex // one apply at a time, in order
	last applyStatus
}

// applyStatus is the result of the last apply.
type applyStatus struct {
	At       time.Time `json:"at"`
	OK       bool      `json:"ok"`
	Error    string    `json:"error,omitempty"`
	Warnings []string  `json:"warnings"`
}

type agentShare struct {
	Name        string      `json:"name"`
	Path        string      `json:"path"`
	Description string      `json:"description"`
	SMB         *shares.SMB `json:"smb,omitempty"`
	NFS         *shares.NFS `json:"nfs,omitempty"`
	Owners      []string    `json:"owners"`
	Readers     []string    `json:"readers"`
}

type agentGroup struct {
	Name    string   `json:"name"`
	Members []string `json:"members"`
}

type agentSharesRequest struct {
	Shares []agentShare `json:"shares"`
	Groups []agentGroup `json:"groups"`
	Users  []string     `json:"users"`
}

// Apply pushes the current state to the agent.
func (a *sharesApplier) Apply(ctx context.Context) applyStatus {
	a.mu.Lock()
	defer a.mu.Unlock()

	st := applyStatus{At: time.Now().UTC(), Warnings: []string{}}
	req := agentSharesRequest{Shares: []agentShare{}, Groups: []agentGroup{}, Users: []string{}}
	for _, sh := range a.shares.List() {
		req.Shares = append(req.Shares, agentShare{
			Name: sh.Name, Path: sh.Path, Description: sh.Description,
			SMB: sh.SMB, NFS: sh.NFS, Owners: sh.Owners, Readers: sh.Readers,
		})
	}
	for _, g := range a.groups.List() {
		req.Groups = append(req.Groups, agentGroup{Name: g.Name, Members: g.Members})
	}
	if names, err := a.users.Usernames(); err == nil {
		req.Users = names
	} else {
		st.Warnings = append(st.Warnings, "could not list users: "+err.Error())
	}

	var resp struct {
		Warnings []string `json:"warnings"`
	}
	if err := a.agent.PostJSON(ctx, "/v1/shares/apply", req, &resp); err != nil {
		st.Error = err.Error()
		log.Error().Err(err).Msg("shares: agent could not apply the share configuration")
	} else {
		st.OK = true
		st.Warnings = append(st.Warnings, resp.Warnings...)
		for _, w := range resp.Warnings {
			log.Warn().Str("warning", w).Msg("shares: applied with a warning")
		}
	}
	a.last = st
	return st
}

// Status returns the result of the last apply.
func (a *sharesApplier) Status() applyStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.last
}

// applyAtStartup applies once nos-agent answers (it may start after nosd).
func (a *sharesApplier) applyAtStartup() {
	for attempt := 0; attempt < 12; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		st := a.Apply(ctx)
		cancel()
		if st.OK {
			return
		}
		time.Sleep(10 * time.Second)
	}
}
