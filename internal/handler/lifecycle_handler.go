package handler

import (
	"net/http"
	"strconv"

	"github.com/togglerino/togglerino/internal/store"
)

type LifecycleHandler struct {
	flags     *store.FlagStore
	snapshots *store.LifecycleSnapshotStore
	projects  *store.ProjectStore
}

func NewLifecycleHandler(flags *store.FlagStore, snapshots *store.LifecycleSnapshotStore, projects *store.ProjectStore) *LifecycleHandler {
	return &LifecycleHandler{flags: flags, snapshots: snapshots, projects: projects}
}

// Summary handles GET /api/v1/projects/{key}/lifecycle/summary
func (h *LifecycleHandler) Summary(w http.ResponseWriter, r *http.Request) {
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

	summary, err := h.flags.LifecycleSummary(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get lifecycle summary")
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

// Trends handles GET /api/v1/projects/{key}/lifecycle/trends?days=30
func (h *LifecycleHandler) Trends(w http.ResponseWriter, r *http.Request) {
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

	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		parsed, err := strconv.Atoi(d)
		if err != nil {
			writeError(w, http.StatusBadRequest, "days must be a number")
			return
		}
		if parsed < 1 {
			days = 1
		} else if parsed > 365 {
			days = 365
		} else {
			days = parsed
		}
	}

	trends, err := h.snapshots.GetTrends(r.Context(), project.ID, days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get lifecycle trends")
		return
	}

	writeJSON(w, http.StatusOK, trends)
}
