package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

var roleNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,48}[a-z0-9]$`)

// RoleCacheRefresher reloads the in-memory role cache after mutations.
type RoleCacheRefresher interface {
	Refresh()
}

// RoleHandler manages role definition endpoints.
type RoleHandler struct {
	roles     *store.RoleStore
	refresher RoleCacheRefresher
	audit     *store.AuditStore
}

func NewRoleHandler(roles *store.RoleStore, refresher RoleCacheRefresher, audit *store.AuditStore) *RoleHandler {
	return &RoleHandler{roles: roles, refresher: refresher, audit: audit}
}

// List returns all roles.
// GET /api/v1/roles
func (h *RoleHandler) List(w http.ResponseWriter, r *http.Request) {
	roles, err := h.roles.List(r.Context())
	if err != nil {
		slog.Error("failed to list roles", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list roles")
		return
	}
	if roles == nil {
		roles = []model.RoleDefinition{}
	}
	writeJSON(w, http.StatusOK, roles)
}

// Get returns a single role by name.
// GET /api/v1/roles/{name}
func (h *RoleHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	role, err := h.roles.GetByName(r.Context(), name)
	if err != nil {
		if store.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "role not found")
			return
		}
		slog.Error("failed to get role", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get role")
		return
	}
	writeJSON(w, http.StatusOK, role)
}

// Create creates a new custom role.
// POST /api/v1/roles
func (h *RoleHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Permissions []string `json:"permissions"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Name = strings.TrimSpace(strings.ToLower(req.Name))

	if !roleNamePattern.MatchString(req.Name) {
		writeError(w, http.StatusBadRequest, "name must be 2-50 lowercase alphanumeric characters or hyphens")
		return
	}
	if len(req.Description) > 200 {
		writeError(w, http.StatusBadRequest, "description must be 200 characters or fewer")
		return
	}
	if len(req.Permissions) == 0 {
		writeError(w, http.StatusBadRequest, "at least one permission is required")
		return
	}
	for _, p := range req.Permissions {
		if !model.ValidPermission(p) {
			writeError(w, http.StatusBadRequest, "invalid permission: "+p)
			return
		}
	}

	req.Permissions = dedupeStrings(req.Permissions)

	role, err := h.roles.Create(r.Context(), req.Name, req.Description, req.Permissions)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "role name already exists")
			return
		}
		slog.Error("failed to create role", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create role")
		return
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		newVal, _ := json.Marshal(role)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			UserID:     &user.ID,
			Action:     "create",
			EntityType: "role",
			EntityID:   role.Name,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	h.refresh()
	writeJSON(w, http.StatusCreated, role)
}

// Update modifies a custom role.
// PUT /api/v1/roles/{name}
func (h *RoleHandler) Update(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	var req struct {
		Description string   `json:"description"`
		Permissions []string `json:"permissions"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Description) > 200 {
		writeError(w, http.StatusBadRequest, "description must be 200 characters or fewer")
		return
	}
	if len(req.Permissions) == 0 {
		writeError(w, http.StatusBadRequest, "at least one permission is required")
		return
	}
	for _, p := range req.Permissions {
		if !model.ValidPermission(p) {
			writeError(w, http.StatusBadRequest, "invalid permission: "+p)
			return
		}
	}

	req.Permissions = dedupeStrings(req.Permissions)

	// Fetch old state for audit logging before the transactional update.
	oldRole, _ := h.roles.GetByName(r.Context(), name)

	role, err := h.roles.Update(r.Context(), name, req.Description, req.Permissions)
	if err != nil {
		if errors.Is(err, store.ErrBuiltInRole) {
			writeError(w, http.StatusForbidden, "cannot modify built-in role")
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "role not found")
			return
		}
		slog.Error("failed to update role", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update role")
		return
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		newVal, _ := json.Marshal(role)
		var oldVal json.RawMessage
		if oldRole != nil {
			oldVal, _ = json.Marshal(oldRole)
		}
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			UserID:     &user.ID,
			Action:     "update",
			EntityType: "role",
			EntityID:   role.Name,
			OldValue:   oldVal,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	h.refresh()
	writeJSON(w, http.StatusOK, role)
}

// Delete removes a custom role.
// DELETE /api/v1/roles/{name}
func (h *RoleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	// Fetch old state for audit logging before deletion.
	oldRole, _ := h.roles.GetByName(r.Context(), name)

	err := h.roles.Delete(r.Context(), name)
	if err != nil {
		if errors.Is(err, store.ErrBuiltInRole) {
			writeError(w, http.StatusForbidden, "cannot delete built-in role")
			return
		}
		if errors.Is(err, store.ErrRoleInUse) {
			writeError(w, http.StatusConflict, "role is in use and cannot be deleted")
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "role not found")
			return
		}
		slog.Error("failed to delete role", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete role")
		return
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		var oldVal json.RawMessage
		if oldRole != nil {
			oldVal, _ = json.Marshal(oldRole)
		}
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			UserID:     &user.ID,
			Action:     "delete",
			EntityType: "role",
			EntityID:   name,
			OldValue:   oldVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	h.refresh()
	w.WriteHeader(http.StatusNoContent)
}

func dedupeStrings(s []string) []string {
	seen := make(map[string]bool, len(s))
	result := make([]string, 0, len(s))
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

func (h *RoleHandler) refresh() {
	if h.refresher != nil {
		h.refresher.Refresh()
	}
}
