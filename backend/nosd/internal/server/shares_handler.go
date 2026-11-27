package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"nos/backend/nosd/internal/groups"
	"nos/backend/nosd/internal/policy"
	"nos/backend/nosd/internal/shares"
	"nos/backend/nosd/pkg/httpx"
	"nos/backend/nosd/pkg/validate"
)

// userExists answers whether a nOS user exists.
type userExists interface {
	UserExists(username string) bool
}

// SharesHandler serves /api/v1/shares. A share is /srv/shares/<name>,
// exported over SMB and/or NFS, with owners (read and write) and readers.
// Every change is saved first and then applied to the host by nos-agent.
type SharesHandler struct {
	store   *shares.Store
	groups  *groups.Store
	users   userExists
	policy  *policy.Store
	applier *sharesApplier
}

// shareInput is the body of create, update and test. Pointers tell "not
// sent" from "false/empty", so an update changes only what it sends.
type shareInput struct {
	Name        string      `json:"name"`
	Description *string     `json:"description"`
	SMB         *shares.SMB `json:"smb"`
	NFS         *shares.NFS `json:"nfs"`
	Owners      *[]string   `json:"owners"`
	Readers     *[]string   `json:"readers"`
	Apps        *[]string   `json:"apps"`
	Sync        *struct {
		Enabled bool `json:"enabled"`
	} `json:"sync"`
}

// fieldError is one problem with a share, as the UI's test call shows it.
type fieldError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

// Routes registers the shares routes. Reads are for any signed-in user;
// changes need an admin.
func (h *SharesHandler) Routes(adminRequired func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Get("/{name}", h.get)
	r.Group(func(r chi.Router) {
		r.Use(adminRequired)
		r.Post("/", h.create)
		r.Patch("/{name}", h.update)
		r.Put("/{name}", h.update)
		r.Delete("/{name}", h.delete)
		r.Post("/{name}/test", h.test)
		r.Get("/apply-status", h.applyStatus)
		r.Post("/apply", h.reapply)
	})
	return r
}

func (h *SharesHandler) list(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, shareViews(h.store.List()))
}

func (h *SharesHandler) get(w http.ResponseWriter, r *http.Request) {
	sh, ok := h.store.Lookup(chi.URLParam(r, "name"))
	if !ok {
		httpx.WriteTypedError(w, http.StatusNotFound, "shares.not_found", "Share not found", 0)
		return
	}
	writeJSON(w, newShareView(sh))
}

func decodeShareInput(w http.ResponseWriter, r *http.Request) (shareInput, bool) {
	var in shareInput
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	if err := dec.Decode(&in); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "shares.invalid_body", "Invalid request body", 0)
		return in, false
	}
	return in, true
}

// merge applies in to sh (a copy).
func (in shareInput) merge(sh *shares.Share) {
	if in.Description != nil {
		sh.Description = strings.TrimSpace(*in.Description)
	}
	if in.SMB != nil {
		smb := *in.SMB
		if smb.Recycle != nil {
			rc := *smb.Recycle
			if rc.Directory == "" {
				rc.Directory = shares.DefaultRecycleDir
			}
			smb.Recycle = &rc
		}
		sh.SMB = &smb
	}
	if in.NFS != nil {
		nfs := *in.NFS
		sh.NFS = &nfs
	}
	if in.Owners != nil {
		sh.Owners = dedupe(*in.Owners)
	}
	if in.Readers != nil {
		sh.Readers = dedupe(*in.Readers)
	}
	if in.Apps != nil {
		sh.Apps = dedupe(*in.Apps)
	}
	if in.Sync != nil {
		sh.SyncDisabled = !in.Sync.Enabled
	}
}

func dedupe(list []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range list {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// check lists everything wrong with sh.
func (h *SharesHandler) check(sh shares.Share) []fieldError {
	var errs []fieldError
	add := func(field, code, format string, a ...any) {
		errs = append(errs, fieldError{Code: code, Field: field, Message: fmt.Sprintf(format, a...)})
	}
	if err := validate.ShareName(sh.Name); err != nil {
		add("name", "shares.invalid_name", "Share name: use 1-32 letters, digits, '-' or '_'")
	}
	if err := validate.Description(sh.Description); err != nil {
		add("description", "shares.invalid_description", "%s", err.Error())
	}
	if err := validate.PathUnder(validate.ShareRoots, sh.Path); err != nil {
		add("path", "shares.invalid_path", "Share path must be inside %s", strings.Join(validate.ShareRoots, " or "))
	}
	for field, list := range map[string][]string{"owners": sh.Owners, "readers": sh.Readers} {
		for _, p := range list {
			kind, name, err := shares.ParsePrincipal(p)
			switch {
			case err != nil:
				add(field, "shares.invalid_principal", "%s", err.Error())
			case kind == "user" && !h.users.UserExists(name):
				add(field, "shares.unknown_user", "Unknown user %q", name)
			case kind == "group" && name != shares.AllUsersGroup:
				if _, ok := h.groups.Get(name); !ok {
					add(field, "shares.unknown_group", "Unknown group %q", name)
				}
			}
		}
	}
	for _, id := range sh.Apps {
		if err := validate.AppID(id); err != nil {
			add("apps", "shares.invalid_app", "App id %q: use lowercase letters, digits and '-'", id)
		}
	}
	if sh.SMB != nil && sh.SMB.Enabled {
		if sh.SMB.Guest && h.policy.Get().Shares.GuestAccessForbidden {
			add("smb.guest", "shares.guest_forbidden", "Guest access is forbidden by policy")
		}
		if rc := sh.SMB.Recycle; rc != nil && rc.Enabled {
			d := rc.Directory
			if d == "." || d == ".." || strings.ContainsAny(d, "/\\") || validate.ShareName(strings.TrimPrefix(d, ".")) != nil {
				add("smb.recycle.directory", "shares.invalid_recycle_dir", "Recycle directory: a folder name of letters, digits, '.', '-' or '_'")
			}
		}
	}
	if sh.NFS != nil && sh.NFS.Enabled {
		if len(sh.NFS.Networks) == 0 {
			add("nfs.networks", "shares.nfs_networks_required", "NFS needs at least one allowed network or host")
		}
		for _, c := range sh.NFS.Networks {
			if err := validate.NFSClient(c); err != nil {
				add("nfs.networks", "shares.invalid_nfs_client", "%s: %q", err.Error(), c)
			}
		}
	}
	return errs
}

func writeShareErrors(w http.ResponseWriter, errs []fieldError) {
	httpx.WriteErrorWithDetails(w, http.StatusBadRequest, errs[0].Code, errs[0].Message, map[string]any{"errors": errs})
}

func (h *SharesHandler) create(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeShareInput(w, r)
	if !ok {
		return
	}
	sh := shares.Share{Name: strings.TrimSpace(in.Name)}
	sh.Path = filepath.Join(shares.Root, sh.Name)
	in.merge(&sh)
	if len(sh.Owners) == 0 && len(sh.Readers) == 0 {
		// A new share without access lists is for everyone; set the lists to
		// restrict it. (An existing share emptied later is closed instead.)
		sh.Owners = []string{shares.Everyone}
	}
	if errs := h.check(sh); len(errs) > 0 {
		writeShareErrors(w, errs)
		return
	}
	created, err := h.store.Create(sh)
	if errors.Is(err, shares.ErrExists) {
		httpx.WriteTypedError(w, http.StatusConflict, "shares.exists", "A share with this name already exists", 0)
		return
	}
	if err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "shares.save_failed", "Failed to save the share", 0)
		return
	}
	if !h.applyOrFail(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(newShareView(created))
}

func (h *SharesHandler) update(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeShareInput(w, r)
	if !ok {
		return
	}
	cur, found := h.store.Lookup(chi.URLParam(r, "name"))
	if !found {
		httpx.WriteTypedError(w, http.StatusNotFound, "shares.not_found", "Share not found", 0)
		return
	}
	if in.Name != "" && in.Name != cur.Name {
		httpx.WriteTypedError(w, http.StatusBadRequest, "shares.rename_unsupported", "A share can't be renamed; create a new one", 0)
		return
	}
	var errs []fieldError
	updated, err := h.store.Modify(cur.ID, func(sh *shares.Share) error {
		in.merge(sh)
		if errs = h.check(*sh); len(errs) > 0 {
			return errors.New("invalid")
		}
		return nil
	})
	if len(errs) > 0 {
		writeShareErrors(w, errs)
		return
	}
	if err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "shares.save_failed", "Failed to save the share", 0)
		return
	}
	if !h.applyOrFail(w, r) {
		return
	}
	writeJSON(w, newShareView(updated))
}

// delete removes the share from the configuration. The folder and its files
// stay on disk.
func (h *SharesHandler) delete(w http.ResponseWriter, r *http.Request) {
	sh, ok := h.store.Lookup(chi.URLParam(r, "name"))
	if !ok {
		httpx.WriteTypedError(w, http.StatusNotFound, "shares.not_found", "Share not found", 0)
		return
	}
	if err := h.store.Delete(sh.ID); err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "shares.save_failed", "Failed to delete the share", 0)
		return
	}
	if !h.applyOrFail(w, r) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// test checks a proposed configuration for the share (an existing one, or a
// new one with that name) without saving it.
func (h *SharesHandler) test(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Config shareInput `json:"config"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "shares.invalid_body", "Invalid request body", 0)
		return
	}
	name := chi.URLParam(r, "name")
	sh, ok := h.store.Lookup(name)
	if !ok {
		sh = shares.Share{Name: name, Path: filepath.Join(shares.Root, name)}
	}
	body.Config.merge(&sh)
	errs := h.check(sh)
	if errs == nil {
		errs = []fieldError{}
	}
	writeJSON(w, map[string]any{"valid": len(errs) == 0, "errors": errs})
}

func (h *SharesHandler) applyStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.applier.Status())
}

func (h *SharesHandler) reapply(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.applier.Apply(r.Context()))
}

// applyOrFail applies the saved state; when the agent can't, the change is
// kept (it applies with the next change or at startup) and the caller gets
// 502 shares.apply_failed.
func (h *SharesHandler) applyOrFail(w http.ResponseWriter, r *http.Request) bool {
	st := h.applier.Apply(r.Context())
	if !st.OK {
		httpx.WriteTypedError(w, http.StatusBadGateway, "shares.apply_failed",
			"Saved, but the system could not apply it: "+st.Error, 0)
		return false
	}
	return true
}

// shareView is a share as the API shows it: sync as {"enabled": bool}, the
// shape the share form sends.
type shareView struct {
	shares.Share
	SyncDisabled bool `json:"-"` // shown as sync.enabled
	Sync         struct {
		Enabled bool `json:"enabled"`
	} `json:"sync"`
}

func newShareView(sh shares.Share) shareView {
	v := shareView{Share: sh}
	v.Sync.Enabled = !sh.SyncDisabled
	return v
}

func shareViews(list []shares.Share) []shareView {
	out := make([]shareView, 0, len(list))
	for _, sh := range list {
		out = append(out, newShareView(sh))
	}
	return out
}
