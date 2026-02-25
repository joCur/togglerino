package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/stream"
)

// StreamHandler handles SSE connections for real-time flag updates.
type StreamHandler struct {
	hub *stream.Hub
}

// NewStreamHandler creates a new StreamHandler backed by the given Hub.
func NewStreamHandler(hub *stream.Hub) *StreamHandler {
	return &StreamHandler{hub: hub}
}

// Handle serves GET /api/v1/stream/{project}/{env} as an SSE endpoint.
// Clients connect and receive flag_update events as they occur.
func (h *StreamHandler) Handle(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("project")
	envKey := r.PathValue("env")

	sdkKey := auth.SDKKeyFromContext(r.Context())
	if sdkKey.ProjectKey != projectKey || sdkKey.EnvironmentKey != envKey {
		writeError(w, http.StatusForbidden, "SDK key is not authorized for this project/environment")
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Subscribe to events
	ch := h.hub.Subscribe(projectKey, envKey)
	defer h.hub.Unsubscribe(projectKey, envKey, ch)

	// Send initial keepalive
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	// Stream events until client disconnects
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "event: flag_update\ndata: %s\n\n", data)
			flusher.Flush()
		}
	}
}
