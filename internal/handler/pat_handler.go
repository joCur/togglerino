package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type PATHandler struct {
	pats *store.PATStore
}

func NewPATHandler(pats *store.PATStore) *PATHandler {
	return &PATHandler{pats: pats}
}

// Create handles POST /api/v1/auth/tokens
func (h *PATHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Name      string     `json:"name"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(req.Name) > 100 {
		writeError(w, http.StatusBadRequest, "name must be 100 characters or fewer")
		return
	}
	if req.ExpiresAt != nil && !req.ExpiresAt.After(time.Now()) {
		writeError(w, http.StatusBadRequest, "expires_at must be in the future")
		return
	}

	pat, err := h.pats.Create(r.Context(), user.ID, req.Name, req.ExpiresAt)
	if err != nil {
		slog.Error("failed to create personal access token", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create token")
		return
	}

	writeJSON(w, http.StatusCreated, pat)
}

// List handles GET /api/v1/auth/tokens
func (h *PATHandler) List(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	tokens, err := h.pats.ListByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("failed to list personal access tokens", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list tokens")
		return
	}
	if tokens == nil {
		tokens = []model.PersonalAccessToken{}
	}

	writeJSON(w, http.StatusOK, tokens)
}

// Delete handles DELETE /api/v1/auth/tokens/{id}
func (h *PATHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "token id is required")
		return
	}

	if err := h.pats.Delete(r.Context(), id, user.ID); err != nil {
		writeError(w, http.StatusNotFound, "token not found")
		return
	}

	auth.ClearPATLastUsed(id)
	w.WriteHeader(http.StatusNoContent)
}
