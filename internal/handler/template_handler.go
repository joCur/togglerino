package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

var templateKeyRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)

type TemplateHandler struct {
	templates *store.TemplateStore
	projects  *store.ProjectStore
	audit     *store.AuditStore
}

func NewTemplateHandler(templates *store.TemplateStore, projects *store.ProjectStore, audit *store.AuditStore) *TemplateHandler {
	return &TemplateHandler{templates: templates, projects: projects, audit: audit}
}

func (h *TemplateHandler) recordAudit(r *http.Request, action string, projectID *string, entityID string, oldVal, newVal any) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		return
	}
	var oldJSON, newJSON json.RawMessage
	if oldVal != nil {
		oldJSON, _ = json.Marshal(oldVal)
	}
	if newVal != nil {
		newJSON, _ = json.Marshal(newVal)
	}
	if err := h.audit.Record(r.Context(), model.AuditEntry{
		ProjectID:  projectID,
		UserID:     &user.ID,
		UserEmail:  &user.Email,
		Action:     action,
		EntityType: "template",
		EntityID:   entityID,
		OldValue:   oldJSON,
		NewValue:   newJSON,
	}); err != nil {
		slog.Warn("failed to record template audit log", "error", err)
	}
}

// templateRequest is the shared request body for create/update operations.
type templateRequest struct {
	Key                 string          `json:"key"`
	Name                string          `json:"name"`
	Description         string          `json:"description"`
	FlagType            model.FlagType  `json:"flag_type"`
	ValueType           model.ValueType `json:"value_type"`
	DefaultValue        json.RawMessage `json:"default_value"`
	Tags                []string        `json:"tags"`
	EnvironmentDefaults json.RawMessage `json:"environment_defaults"`
	VariantConfig       json.RawMessage `json:"variant_config"`
	SortOrder           int             `json:"sort_order"`
}

func validateTemplateRequest(req *templateRequest, requireKey bool) string {
	if requireKey && req.Key == "" {
		return "key is required"
	}
	if requireKey && !templateKeyRegex.MatchString(req.Key) {
		return "key must be 3-64 lowercase alphanumeric characters or hyphens, starting and ending with alphanumeric"
	}
	if req.Name == "" {
		return "name is required"
	}
	if req.FlagType == "" {
		return "flag_type is required"
	}
	if !model.ValidFlagTypes[req.FlagType] {
		return "invalid flag_type"
	}
	if req.ValueType == "" {
		return "value_type is required"
	}
	if !model.ValidValueTypes[req.ValueType] {
		return "invalid value_type"
	}
	return ""
}

func defaultRawMessage(v json.RawMessage, def string) json.RawMessage {
	if len(v) == 0 {
		return json.RawMessage(def)
	}
	return v
}

// ListGlobal handles GET /api/v1/templates
func (h *TemplateHandler) ListGlobal(w http.ResponseWriter, r *http.Request) {
	templates, err := h.templates.ListGlobal(r.Context())
	if err != nil {
		slog.Error("failed to list templates", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list templates")
		return
	}
	writeJSON(w, http.StatusOK, templates)
}

// CreateGlobal handles POST /api/v1/templates
func (h *TemplateHandler) CreateGlobal(w http.ResponseWriter, r *http.Request) {
	var req templateRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if msg := validateTemplateRequest(&req, true); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	req.DefaultValue = defaultRawMessage(req.DefaultValue, "null")
	req.EnvironmentDefaults = defaultRawMessage(req.EnvironmentDefaults, "{}")
	req.VariantConfig = defaultRawMessage(req.VariantConfig, "{}")
	if req.Tags == nil {
		req.Tags = []string{}
	}

	tmpl, err := h.templates.Create(r.Context(), nil, req.Key, req.Name, req.Description, req.FlagType, req.ValueType, req.DefaultValue, req.Tags, req.EnvironmentDefaults, req.VariantConfig, false, req.SortOrder)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "template key already exists")
			return
		}
		slog.Error("failed to create template", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create template")
		return
	}

	h.recordAudit(r, "create", nil, tmpl.Key, nil, tmpl)
	writeJSON(w, http.StatusCreated, tmpl)
}

// UpdateGlobal handles PUT /api/v1/templates/{key}
func (h *TemplateHandler) UpdateGlobal(w http.ResponseWriter, r *http.Request) {
	templateKey := r.PathValue("key")
	if templateKey == "" {
		writeError(w, http.StatusBadRequest, "template key is required")
		return
	}

	existing, err := h.templates.GetByKey(r.Context(), nil, templateKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}

	var req templateRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if msg := validateTemplateRequest(&req, false); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	req.DefaultValue = defaultRawMessage(req.DefaultValue, "null")
	req.EnvironmentDefaults = defaultRawMessage(req.EnvironmentDefaults, "{}")
	req.VariantConfig = defaultRawMessage(req.VariantConfig, "{}")
	if req.Tags == nil {
		req.Tags = []string{}
	}

	updated, err := h.templates.Update(r.Context(), existing.ID, req.Name, req.Description, req.FlagType, req.ValueType, req.DefaultValue, req.Tags, req.EnvironmentDefaults, req.VariantConfig, req.SortOrder)
	if err != nil {
		slog.Error("failed to update template", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update template")
		return
	}

	h.recordAudit(r, "update", nil, existing.Key, existing, updated)
	writeJSON(w, http.StatusOK, updated)
}

// DeleteGlobal handles DELETE /api/v1/templates/{key}
func (h *TemplateHandler) DeleteGlobal(w http.ResponseWriter, r *http.Request) {
	templateKey := r.PathValue("key")
	if templateKey == "" {
		writeError(w, http.StatusBadRequest, "template key is required")
		return
	}

	existing, err := h.templates.GetByKey(r.Context(), nil, templateKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}

	if existing.IsSystem {
		writeError(w, http.StatusForbidden, "system templates cannot be deleted")
		return
	}

	if err := h.templates.Delete(r.Context(), existing.ID); err != nil {
		slog.Error("failed to delete template", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete template")
		return
	}

	h.recordAudit(r, "delete", nil, existing.Key, existing, nil)
	w.WriteHeader(http.StatusNoContent)
}

// ListForProject handles GET /api/v1/projects/{key}/templates
// Returns {"global": [...], "project": [...]}
func (h *TemplateHandler) ListForProject(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	global, err := h.templates.ListGlobal(r.Context())
	if err != nil {
		slog.Error("failed to list global templates", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list global templates")
		return
	}

	projectTemplates, err := h.templates.ListByProject(r.Context(), project.ID)
	if err != nil {
		slog.Error("failed to list project templates", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list project templates")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"global":  global,
		"project": projectTemplates,
	})
}

// CreateForProject handles POST /api/v1/projects/{key}/templates
func (h *TemplateHandler) CreateForProject(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	var req templateRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if msg := validateTemplateRequest(&req, true); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	req.DefaultValue = defaultRawMessage(req.DefaultValue, "null")
	req.EnvironmentDefaults = defaultRawMessage(req.EnvironmentDefaults, "{}")
	req.VariantConfig = defaultRawMessage(req.VariantConfig, "{}")
	if req.Tags == nil {
		req.Tags = []string{}
	}

	tmpl, err := h.templates.Create(r.Context(), &project.ID, req.Key, req.Name, req.Description, req.FlagType, req.ValueType, req.DefaultValue, req.Tags, req.EnvironmentDefaults, req.VariantConfig, false, req.SortOrder)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "template key already exists for this project")
			return
		}
		slog.Error("failed to create template", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create template")
		return
	}

	h.recordAudit(r, "create", &project.ID, tmpl.Key, nil, tmpl)
	writeJSON(w, http.StatusCreated, tmpl)
}

// UpdateForProject handles PUT /api/v1/projects/{key}/templates/{templateKey}
func (h *TemplateHandler) UpdateForProject(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	templateKey := r.PathValue("templateKey")
	if templateKey == "" {
		writeError(w, http.StatusBadRequest, "template key is required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	existing, err := h.templates.GetByKey(r.Context(), &project.ID, templateKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}

	var req templateRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if msg := validateTemplateRequest(&req, false); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	req.DefaultValue = defaultRawMessage(req.DefaultValue, "null")
	req.EnvironmentDefaults = defaultRawMessage(req.EnvironmentDefaults, "{}")
	req.VariantConfig = defaultRawMessage(req.VariantConfig, "{}")
	if req.Tags == nil {
		req.Tags = []string{}
	}

	updated, err := h.templates.Update(r.Context(), existing.ID, req.Name, req.Description, req.FlagType, req.ValueType, req.DefaultValue, req.Tags, req.EnvironmentDefaults, req.VariantConfig, req.SortOrder)
	if err != nil {
		slog.Error("failed to update template", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update template")
		return
	}

	h.recordAudit(r, "update", &project.ID, existing.Key, existing, updated)
	writeJSON(w, http.StatusOK, updated)
}

// DeleteForProject handles DELETE /api/v1/projects/{key}/templates/{templateKey}
func (h *TemplateHandler) DeleteForProject(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	templateKey := r.PathValue("templateKey")
	if templateKey == "" {
		writeError(w, http.StatusBadRequest, "template key is required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	existing, err := h.templates.GetByKey(r.Context(), &project.ID, templateKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}

	if existing.IsSystem {
		writeError(w, http.StatusForbidden, "system templates cannot be deleted")
		return
	}

	if err := h.templates.Delete(r.Context(), existing.ID); err != nil {
		slog.Error("failed to delete template", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete template")
		return
	}

	h.recordAudit(r, "delete", &project.ID, existing.Key, existing, nil)
	w.WriteHeader(http.StatusNoContent)
}
