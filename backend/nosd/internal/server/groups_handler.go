package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"

	"nos/backend/nosd/internal/groups"
	"nos/backend/nosd/internal/policy"
	"nos/backend/nosd/internal/shares"
	"nos/backend/nosd/pkg/httpx"
	"nos/backend/nosd/pkg/validate"
)

// GroupsHandler serves /api/v1/groups. Groups are lists of nOS users that
// shares grant access to; nos-agent mirrors them as Unix groups.
type GroupsHandler struct {
	store   *groups.Store
	shares  *shares.Store
	users   userExists
	applier *sharesApplier
}

type groupInput struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Members     []string `json:"members"`
}

// Routes registers the groups routes: reads for any signed-in user, changes
// for admins.
func (h *GroupsHandler) Routes(adminRequired func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, h.store.List()) })
	r.Get("/{name}", h.get)
	r.Group(func(r chi.Router) {
		r.Use(adminRequired)
		r.Post("/", h.create)
		r.Put("/{name}", h.update)
		r.Patch("/{name}", h.update)
		r.Delete("/{name}", h.delete)
	})
	return r
}

func (h *GroupsHandler) get(w http.ResponseWriter, r *http.Request) {
	g, ok := h.store.Get(chi.URLParam(r, "name"))
	if !ok {
		httpx.WriteTypedError(w, http.StatusNotFound, "groups.not_found", "Group not found", 0)
		return
	}
	writeJSON(w, g)
}

func (h *GroupsHandler) decode(w http.ResponseWriter, r *http.Request) (groupInput, bool) {
	var in groupInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&in); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "groups.invalid_body", "Invalid request body", 0)
		return in, false
	}
	in.Description = strings.TrimSpace(in.Description)
	if err := validate.Description(in.Description); err != nil {
		httpx.WriteTypedError(w, http.StatusBadRequest, "groups.invalid_description", err.Error(), 0)
		return in, false
	}
	for _, m := range in.Members {
		if !h.users.UserExists(m) {
			httpx.WriteTypedError(w, http.StatusBadRequest, "groups.unknown_user", "Unknown user "+m, 0)
			return in, false
		}
	}
	return in, true
}

func (h *GroupsHandler) create(w http.ResponseWriter, r *http.Request) {
	in, ok := h.decode(w, r)
	if !ok {
		return
	}
	g, err := h.store.Create(groups.Group{Name: strings.TrimSpace(in.Name), Description: in.Description, Members: in.Members})
	switch {
	case errors.Is(err, groups.ErrBadName):
		httpx.WriteTypedError(w, http.StatusBadRequest, "groups.invalid_name", err.Error(), 0)
		return
	case errors.Is(err, groups.ErrExists):
		httpx.WriteTypedError(w, http.StatusConflict, "groups.exists", err.Error(), 0)
		return
	case err != nil:
		httpx.WriteTypedError(w, http.StatusInternalServerError, "groups.save_failed", "Failed to save the group", 0)
		return
	}
	if !h.apply(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(g)
}

func (h *GroupsHandler) update(w http.ResponseWriter, r *http.Request) {
	in, ok := h.decode(w, r)
	if !ok {
		return
	}
	g, err := h.store.Update(chi.URLParam(r, "name"), in.Description, in.Members)
	if errors.Is(err, groups.ErrNotFound) {
		httpx.WriteTypedError(w, http.StatusNotFound, "groups.not_found", "Group not found", 0)
		return
	}
	if err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "groups.save_failed", "Failed to save the group", 0)
		return
	}
	if !h.apply(w, r) {
		return
	}
	writeJSON(w, g)
}

// delete refuses a group that shares still grant access to.
func (h *GroupsHandler) delete(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	principal := shares.GroupPrefix + name
	var usedBy []string
	for _, sh := range h.shares.List() {
		if slices.Contains(sh.Owners, principal) || slices.Contains(sh.Readers, principal) {
			usedBy = append(usedBy, sh.Name)
		}
	}
	if len(usedBy) > 0 {
		httpx.WriteErrorWithDetails(w, http.StatusConflict, "groups.in_use",
			"The group is used by shares: "+strings.Join(usedBy, ", "), map[string]any{"shares": usedBy})
		return
	}
	if err := h.store.Delete(name); errors.Is(err, groups.ErrNotFound) {
		httpx.WriteTypedError(w, http.StatusNotFound, "groups.not_found", "Group not found", 0)
		return
	} else if err != nil {
		httpx.WriteTypedError(w, http.StatusInternalServerError, "groups.save_failed", "Failed to delete the group", 0)
		return
	}
	if !h.apply(w, r) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *GroupsHandler) apply(w http.ResponseWriter, r *http.Request) bool {
	if st := h.applier.Apply(r.Context()); !st.OK {
		httpx.WriteTypedError(w, http.StatusBadGateway, "shares.apply_failed",
			"Saved, but the system could not apply it: "+st.Error, 0)
		return false
	}
	return true
}

// PolicyHandler serves /api/v1/policy.
type PolicyHandler struct{ store *policy.Store }

// Routes: anyone signed in reads the policy (the UI hides what it forbids);
// admins change it.
func (h *PolicyHandler) Routes(adminRequired func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, h.store.Get()) })
	r.With(adminRequired).Put("/", func(w http.ResponseWriter, r *http.Request) {
		var p policy.Policy
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&p); err != nil {
			httpx.WriteTypedError(w, http.StatusBadRequest, "policy.invalid_body", "Invalid request body", 0)
			return
		}
		if err := h.store.Set(p); err != nil {
			httpx.WriteTypedError(w, http.StatusInternalServerError, "policy.save_failed", "Failed to save the policy", 0)
			return
		}
		writeJSON(w, p)
	})
	return r
}
