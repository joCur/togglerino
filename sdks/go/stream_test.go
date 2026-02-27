package togglerino

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestSSE_ProcessesFlagUpdate(t *testing.T) {
	sseData := "event: flag_update\ndata: {\"type\":\"flag_update\",\"flagKey\":\"dark-mode\",\"value\":true,\"variant\":\"on\"}\n\n"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/evaluate" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(evaluateResponse{
				Flags: map[string]*EvaluationResult{
					"dark-mode": {Value: false, Variant: "off", Reason: "default"},
				},
			})
			return
		}
		if r.URL.Path == "/api/v1/stream" {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			flusher, _ := w.(http.Flusher)
			fmt.Fprint(w, ": connected\n\n")
			flusher.Flush()
			fmt.Fprint(w, sseData)
			flusher.Flush()
			// Keep connection open
			<-r.Context().Done()
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	var changes []FlagChangeEvent
	var mu sync.Mutex

	client, err := New(context.Background(), Config{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(true),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	client.OnChange(func(e FlagChangeEvent) {
		mu.Lock()
		changes = append(changes, e)
		mu.Unlock()
	})

	time.Sleep(300 * time.Millisecond)

	if got := client.BoolValue("dark-mode", false); got != true {
		t.Errorf("BoolValue after SSE update = %v, want true", got)
	}

	mu.Lock()
	if len(changes) == 0 {
		t.Fatal("expected change event, got none")
	}
	if changes[0].FlagKey != "dark-mode" {
		t.Errorf("change flagKey = %q, want dark-mode", changes[0].FlagKey)
	}
	mu.Unlock()
}

func TestSSE_ProcessesFlagDeleted(t *testing.T) {
	sseData := "event: flag_deleted\ndata: {\"type\":\"flag_deleted\",\"flagKey\":\"delete-me\"}\n\n"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/evaluate" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(evaluateResponse{
				Flags: map[string]*EvaluationResult{
					"delete-me": {Value: true, Variant: "on", Reason: "default"},
					"keep-me":   {Value: false, Variant: "off", Reason: "default"},
				},
			})
			return
		}
		if r.URL.Path == "/api/v1/stream" {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)
			fmt.Fprint(w, ": connected\n\n")
			flusher.Flush()
			fmt.Fprint(w, sseData)
			flusher.Flush()
			<-r.Context().Done()
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	var deleted []FlagDeletedEvent
	var mu sync.Mutex

	client, err := New(context.Background(), Config{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(true),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	client.OnDeleted(func(e FlagDeletedEvent) {
		mu.Lock()
		deleted = append(deleted, e)
		mu.Unlock()
	})

	time.Sleep(300 * time.Millisecond)

	_, ok := client.Detail("delete-me")
	if ok {
		t.Fatal("deleted flag still present")
	}
	_, ok = client.Detail("keep-me")
	if !ok {
		t.Fatal("kept flag was removed")
	}

	mu.Lock()
	if len(deleted) != 1 || deleted[0].FlagKey != "delete-me" {
		t.Errorf("deleted events = %v, want [{FlagKey: delete-me}]", deleted)
	}
	mu.Unlock()
}

func TestSSE_IgnoresCommentLines(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/evaluate" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(evaluateResponse{
				Flags: map[string]*EvaluationResult{
					"my-flag": {Value: true, Variant: "on", Reason: "default"},
				},
			})
			return
		}
		if r.URL.Path == "/api/v1/stream" {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)
			fmt.Fprint(w, ": connected\n\n")
			flusher.Flush()
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
			<-r.Context().Done()
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	client, err := New(context.Background(), Config{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(true),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	time.Sleep(200 * time.Millisecond)

	if got := client.BoolValue("my-flag", false); got != true {
		t.Errorf("flag changed unexpectedly after comment-only SSE")
	}
}

func TestSSE_ReconnectsOnFailure(t *testing.T) {
	var mu sync.Mutex
	sseConnCount := 0
	sseFail := true

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/evaluate" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(evaluateResponse{Flags: map[string]*EvaluationResult{}})
			return
		}
		if r.URL.Path == "/api/v1/stream" {
			mu.Lock()
			sseConnCount++
			fail := sseFail
			mu.Unlock()

			if fail {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)
			fmt.Fprint(w, ": connected\n\n")
			flusher.Flush()
			<-r.Context().Done()
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	var reconnecting []int
	var reconnectMu sync.Mutex

	client, err := New(context.Background(), Config{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(true),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	client.OnReconnecting(func(attempt int, delay time.Duration) {
		reconnectMu.Lock()
		reconnecting = append(reconnecting, attempt)
		reconnectMu.Unlock()
	})

	// Wait for a couple reconnection attempts
	time.Sleep(4 * time.Second)

	reconnectMu.Lock()
	if len(reconnecting) < 2 {
		t.Errorf("expected at least 2 reconnection attempts, got %d", len(reconnecting))
	}
	reconnectMu.Unlock()
}

func TestSSE_EmitsReconnectedOnSuccess(t *testing.T) {
	var mu sync.Mutex
	sseFail := true

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/evaluate" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(evaluateResponse{Flags: map[string]*EvaluationResult{}})
			return
		}
		if r.URL.Path == "/api/v1/stream" {
			mu.Lock()
			fail := sseFail
			mu.Unlock()

			if fail {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)
			fmt.Fprint(w, ": connected\n\n")
			flusher.Flush()
			<-r.Context().Done()
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	var reconnected bool
	var reconnectedMu sync.Mutex

	client, err := New(context.Background(), Config{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(true),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	client.OnReconnected(func() {
		reconnectedMu.Lock()
		reconnected = true
		reconnectedMu.Unlock()
	})

	// Wait for first retry attempt
	time.Sleep(500 * time.Millisecond)

	// Now make SSE succeed
	mu.Lock()
	sseFail = false
	mu.Unlock()

	// Wait for successful reconnect
	time.Sleep(2 * time.Second)

	reconnectedMu.Lock()
	if !reconnected {
		t.Fatal("expected reconnected event")
	}
	reconnectedMu.Unlock()
}
