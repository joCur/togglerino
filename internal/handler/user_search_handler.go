package handler

import (
	"log/slog"
	"net/http"

	"github.com/togglerino/togglerino/internal/store"
)

// UserSearchHandler provides a lightweight user search for any authenticated user.
type UserSearchHandler struct {
	users *store.UserStore
}

// NewUserSearchHandler creates a new UserSearchHandler.
func NewUserSearchHandler(users *store.UserStore) *UserSearchHandler {
	return &UserSearchHandler{users: users}
}

// Search returns users matching a query string (email or display name).
// GET /api/v1/users/search?q=...
func (h *UserSearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	users, err := h.users.Search(r.Context(), q)
	if err != nil {
		slog.Error("failed to search users", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to search users")
		return
	}

	type userResult struct {
		ID          string  `json:"id"`
		Email       string  `json:"email"`
		DisplayName *string `json:"display_name"`
	}

	results := make([]userResult, len(users))
	for i, u := range users {
		results[i] = userResult{
			ID:          u.ID,
			Email:       u.Email,
			DisplayName: u.DisplayName,
		}
	}
	writeJSON(w, http.StatusOK, results)
}
