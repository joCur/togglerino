package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
	"github.com/togglerino/togglerino/internal/webhook"
)

type WebhookHandler struct {
	webhooks   *store.WebhookStore
	deliveries *store.WebhookDeliveryStore
	projects   *store.ProjectStore
	dispatcher *webhook.Dispatcher
}

func NewWebhookHandler(webhooks *store.WebhookStore, deliveries *store.WebhookDeliveryStore, projects *store.ProjectStore, dispatcher *webhook.Dispatcher) *WebhookHandler {
	return &WebhookHandler{webhooks: webhooks, deliveries: deliveries, projects: projects, dispatcher: dispatcher}
}

// Create handles POST /api/v1/projects/{key}/webhooks
func (h *WebhookHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		Name       string   `json:"name"`
		URL        string   `json:"url"`
		EventTypes []string `json:"event_types"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.URL == "" || len(req.EventTypes) == 0 {
		writeError(w, http.StatusBadRequest, "name, url, and event_types are required")
		return
	}
	if len(req.Name) > 100 {
		writeError(w, http.StatusBadRequest, "name must be 100 characters or fewer")
		return
	}

	for _, et := range req.EventTypes {
		if !webhook.ValidEventTypes[et] {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid event type: %s", et))
			return
		}
	}

	if err := webhook.ValidateURL(req.URL); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid url: %v", err))
		return
	}

	secret, err := webhook.GenerateSecret()
	if err != nil {
		slog.Error("failed to generate webhook secret", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to generate webhook secret")
		return
	}

	wh, err := h.webhooks.Create(r.Context(), project.ID, req.Name, req.URL, secret, req.EventTypes)
	if err != nil {
		slog.Error("failed to create webhook", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create webhook")
		return
	}

	writeJSON(w, http.StatusCreated, wh)
}

// List handles GET /api/v1/projects/{key}/webhooks
func (h *WebhookHandler) List(w http.ResponseWriter, r *http.Request) {
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

	limit, offset := parsePagination(r)
	webhooks, totalCount, err := h.webhooks.ListByProject(r.Context(), project.ID, limit, offset)
	if err != nil {
		slog.Error("failed to list webhooks", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list webhooks")
		return
	}
	if webhooks == nil {
		webhooks = []model.Webhook{}
	}

	for i := range webhooks {
		webhooks[i].Secret = webhook.MaskSecret(webhooks[i].Secret)
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:   webhooks,
		Total:  totalCount,
		Limit:  limit,
		Offset: offset,
	})
}

// Get handles GET /api/v1/projects/{key}/webhooks/{id}
func (h *WebhookHandler) Get(w http.ResponseWriter, r *http.Request) {
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

	webhookID := r.PathValue("id")
	if webhookID == "" {
		writeError(w, http.StatusBadRequest, "webhook id is required")
		return
	}

	wh, err := h.webhooks.GetByID(r.Context(), webhookID)
	if err != nil {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}

	if wh.ProjectID != project.ID {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}

	wh.Secret = webhook.MaskSecret(wh.Secret)

	writeJSON(w, http.StatusOK, wh)
}

// Update handles PUT /api/v1/projects/{key}/webhooks/{id}
func (h *WebhookHandler) Update(w http.ResponseWriter, r *http.Request) {
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

	webhookID := r.PathValue("id")
	if webhookID == "" {
		writeError(w, http.StatusBadRequest, "webhook id is required")
		return
	}

	existing, err := h.webhooks.GetByID(r.Context(), webhookID)
	if err != nil {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}

	if existing.ProjectID != project.ID {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}

	var req struct {
		Name       string   `json:"name"`
		URL        string   `json:"url"`
		EventTypes []string `json:"event_types"`
		Enabled    *bool    `json:"enabled"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := req.Name
	if name == "" {
		name = existing.Name
	}
	if len(name) > 100 {
		writeError(w, http.StatusBadRequest, "name must be 100 characters or fewer")
		return
	}
	url := req.URL
	if url == "" {
		url = existing.URL
	}
	eventTypes := req.EventTypes
	if len(eventTypes) == 0 {
		eventTypes = existing.EventTypes
	} else {
		for _, et := range eventTypes {
			if !webhook.ValidEventTypes[et] {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid event type: %s", et))
				return
			}
		}
	}
	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	if url != existing.URL {
		if err := webhook.ValidateURL(url); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid url: %v", err))
			return
		}
	}

	updated, err := h.webhooks.Update(r.Context(), webhookID, name, url, eventTypes, enabled)
	if err != nil {
		slog.Error("failed to update webhook", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update webhook")
		return
	}

	updated.Secret = webhook.MaskSecret(updated.Secret)

	writeJSON(w, http.StatusOK, updated)
}

// Delete handles DELETE /api/v1/projects/{key}/webhooks/{id}
func (h *WebhookHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

	webhookID := r.PathValue("id")
	if webhookID == "" {
		writeError(w, http.StatusBadRequest, "webhook id is required")
		return
	}

	wh, err := h.webhooks.GetByID(r.Context(), webhookID)
	if err != nil {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}

	if wh.ProjectID != project.ID {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}

	if err := h.webhooks.Delete(r.Context(), webhookID); err != nil {
		slog.Error("failed to delete webhook", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete webhook")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Test handles POST /api/v1/projects/{key}/webhooks/{id}/test
func (h *WebhookHandler) Test(w http.ResponseWriter, r *http.Request) {
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

	webhookID := r.PathValue("id")
	if webhookID == "" {
		writeError(w, http.StatusBadRequest, "webhook id is required")
		return
	}

	wh, err := h.webhooks.GetByID(r.Context(), webhookID)
	if err != nil {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}

	if wh.ProjectID != project.ID {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}

	event := webhook.Event{
		Type:      webhook.EventWebhookTest,
		Timestamp: time.Now().UTC(),
		ProjectID: project.ID,
		Actor:     webhookActorFromContext(r.Context()),
		Entity:    json.RawMessage(`{"message":"This is a test webhook delivery from Togglerino."}`),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		slog.Error("failed to marshal test event", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create test event")
		return
	}

	deliveryID := webhook.GenerateDeliveryID()
	result := webhook.Deliver(wh.URL, wh.Secret, webhook.EventWebhookTest, deliveryID, payload)

	// Record the delivery
	if _, err := h.deliveries.Record(r.Context(), deliveryID, wh.ID, webhook.EventWebhookTest, payload, result.StatusCode, result.ResponseBody, result.Error, 1, result.Success, &result.DurationMs); err != nil {
		slog.Warn("failed to record test webhook delivery", "error", err)
	}

	response := map[string]any{
		"success":     result.Success,
		"duration_ms": result.DurationMs,
	}
	if result.StatusCode != nil {
		response["status_code"] = *result.StatusCode
	}
	if result.Error != nil {
		response["error"] = *result.Error
	}

	writeJSON(w, http.StatusOK, response)
}

// Deliveries handles GET /api/v1/projects/{key}/webhooks/{id}/deliveries
func (h *WebhookHandler) Deliveries(w http.ResponseWriter, r *http.Request) {
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

	webhookID := r.PathValue("id")
	if webhookID == "" {
		writeError(w, http.StatusBadRequest, "webhook id is required")
		return
	}

	wh, err := h.webhooks.GetByID(r.Context(), webhookID)
	if err != nil {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}

	if wh.ProjectID != project.ID {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}

	limit, offset := parsePagination(r)
	deliveries, totalCount, err := h.deliveries.ListByWebhook(r.Context(), webhookID, limit, offset)
	if err != nil {
		slog.Error("failed to list webhook deliveries", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list webhook deliveries")
		return
	}
	if deliveries == nil {
		deliveries = []model.WebhookDelivery{}
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:   deliveries,
		Total:  totalCount,
		Limit:  limit,
		Offset: offset,
	})
}
