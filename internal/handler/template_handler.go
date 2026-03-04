package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type TemplateHandler struct {
	templates *store.TemplateStore
	projects  *store.ProjectStore
}

func NewTemplateHandler(templates *store.TemplateStore, projects *store.ProjectStore) *TemplateHandler {
	return &TemplateHandler{templates: templates, projects: projects}
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
		writeError(w, http.StatusInternalServerError, "failed to create template")
		return
	}

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
		writeError(w, http.StatusInternalServerError, "failed to update template")
		return
	}

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
		writeError(w, http.StatusInternalServerError, "failed to delete template")
		return
	}

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
		writeError(w, http.StatusInternalServerError, "failed to list global templates")
		return
	}

	projectTemplates, err := h.templates.ListByProject(r.Context(), project.ID)
	if err != nil {
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
		writeError(w, http.StatusInternalServerError, "failed to create template")
		return
	}

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
		writeError(w, http.StatusInternalServerError, "failed to update template")
		return
	}

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
		writeError(w, http.StatusInternalServerError, "failed to delete template")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
