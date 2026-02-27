package togglerino

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

type testServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []evaluateRequest
	flags    map[string]*EvaluationResult
}

func newTestServer(flags map[string]*EvaluationResult) *testServer {
	ts := &testServer{flags: flags}
	ts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/evaluate" && r.Method == http.MethodPost {
			var req evaluateRequest
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &req)
			ts.mu.Lock()
			ts.requests = append(ts.requests, req)
			ts.mu.Unlock()
			resp := evaluateResponse{Flags: ts.flags}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/api/v1/stream" {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			<-r.Context().Done()
			return
		}
		http.NotFound(w, r)
	}))
	return ts
}

func (ts *testServer) getRequests() []evaluateRequest {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	cp := make([]evaluateRequest, len(ts.requests))
	copy(cp, ts.requests)
	return cp
}

func boolPtr(b bool) *bool { return &b }

func TestNew_FetchesFlags(t *testing.T) {
	ts := newTestServer(map[string]*EvaluationResult{
		"dark-mode":   {Value: true, Variant: "on", Reason: "rule_match"},
		"max-uploads": {Value: float64(10), Variant: "ten", Reason: "default"},
		"welcome-msg": {Value: "Hello!", Variant: "greeting", Reason: "default"},
	})
	defer ts.Close()

	client, err := New(context.Background(), Config{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test123",
		Streaming: boolPtr(false),
		Context:   &EvaluationContext{UserID: "user-1", Attributes: map[string]any{"plan": "pro"}},
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	if got := client.BoolValue("dark-mode", false); got != true {
		t.Errorf("BoolValue = %v, want true", got)
	}
	if got := client.NumberValue("max-uploads", 0); got != 10 {
		t.Errorf("NumberValue = %v, want 10", got)
	}
	if got := client.StringValue("welcome-msg", ""); got != "Hello!" {
		t.Errorf("StringValue = %q, want %q", got, "Hello!")
	}

	reqs := ts.getRequests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	if reqs[0].Context.UserID != "user-1" {
		t.Errorf("user_id = %q, want %q", reqs[0].Context.UserID, "user-1")
	}
}

func TestNew_StripsTrailingSlashes(t *testing.T) {
	ts := newTestServer(map[string]*EvaluationResult{})
	defer ts.Close()

	client, err := New(context.Background(), Config{
		ServerURL: ts.URL + "///",
		SDKKey:    "sdk_test",
		Streaming: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	reqs := ts.getRequests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
}

func TestNew_ReturnsErrorOnFetchFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
	}))
	defer ts.Close()

	_, err := New(context.Background(), Config{
		ServerURL: ts.URL,
		SDKKey:    "sdk_bad",
		Streaming: boolPtr(false),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFlagGetters_DefaultValues(t *testing.T) {
	ts := newTestServer(map[string]*EvaluationResult{})
	defer ts.Close()

	client, err := New(context.Background(), Config{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	if got := client.BoolValue("unknown", false); got != false {
		t.Errorf("BoolValue default = %v, want false", got)
	}
	if got := client.BoolValue("unknown", true); got != true {
		t.Errorf("BoolValue default = %v, want true", got)
	}
	if got := client.StringValue("unknown", ""); got != "" {
		t.Errorf("StringValue default = %q, want empty", got)
	}
	if got := client.StringValue("unknown", "fallback"); got != "fallback" {
		t.Errorf("StringValue default = %q, want %q", got, "fallback")
	}
	if got := client.NumberValue("unknown", 0); got != 0 {
		t.Errorf("NumberValue default = %v, want 0", got)
	}
	if got := client.NumberValue("unknown", 42); got != 42 {
		t.Errorf("NumberValue default = %v, want 42", got)
	}
}

func TestFlagGetters_TypeMismatch(t *testing.T) {
	ts := newTestServer(map[string]*EvaluationResult{
		"str-flag": {Value: "hello", Variant: "a", Reason: "default"},
	})
	defer ts.Close()

	client, err := New(context.Background(), Config{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	if got := client.BoolValue("str-flag", false); got != false {
		t.Errorf("BoolValue on string flag = %v, want false", got)
	}
	if got := client.NumberValue("str-flag", 0); got != 0 {
		t.Errorf("NumberValue on string flag = %v, want 0", got)
	}
}

func TestDetail(t *testing.T) {
	ts := newTestServer(map[string]*EvaluationResult{
		"dark-mode": {Value: true, Variant: "on", Reason: "rule_match"},
	})
	defer ts.Close()

	client, err := New(context.Background(), Config{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	detail, ok := client.Detail("dark-mode")
	if !ok {
		t.Fatal("Detail returned not-ok for existing flag")
	}
	if detail.Variant != "on" || detail.Reason != "rule_match" {
		t.Errorf("Detail = %+v, want variant=on reason=rule_match", detail)
	}

	_, ok = client.Detail("nonexistent")
	if ok {
		t.Fatal("Detail returned ok for nonexistent flag")
	}
}

func TestJSONValue(t *testing.T) {
	ts := newTestServer(map[string]*EvaluationResult{
		"config": {Value: map[string]any{"key": "val"}, Variant: "v1", Reason: "default"},
	})
	defer ts.Close()

	client, err := New(context.Background(), Config{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	var result map[string]string
	defaultVal := map[string]string{"key": "default"}
	err = client.JSONValue("config", &result, defaultVal)
	if err != nil {
		t.Fatalf("JSONValue error: %v", err)
	}
	if result["key"] != "val" {
		t.Errorf("JSONValue result = %v, want key=val", result)
	}

	var result2 map[string]string
	err = client.JSONValue("unknown", &result2, defaultVal)
	if err != nil {
		t.Fatalf("JSONValue error: %v", err)
	}
	if result2["key"] != "default" {
		t.Errorf("JSONValue default = %v, want key=default", result2)
	}
}

func TestUpdateContext(t *testing.T) {
	callCount := 0
	flags1 := map[string]*EvaluationResult{
		"dark-mode": {Value: false, Variant: "off", Reason: "default"},
	}
	flags2 := map[string]*EvaluationResult{
		"dark-mode": {Value: true, Variant: "on", Reason: "rule_match"},
	}

	var mu sync.Mutex
	var requests []evaluateRequest

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/evaluate" {
			var req evaluateRequest
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &req)
			mu.Lock()
			requests = append(requests, req)
			callCount++
			count := callCount
			mu.Unlock()

			var flags map[string]*EvaluationResult
			if count == 1 {
				flags = flags1
			} else {
				flags = flags2
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(evaluateResponse{Flags: flags})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	client, err := New(context.Background(), Config{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(false),
		Context:   &EvaluationContext{UserID: "user-1", Attributes: map[string]any{"plan": "pro"}},
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	if got := client.BoolValue("dark-mode", false); got != false {
		t.Errorf("before update: BoolValue = %v, want false", got)
	}

	var changes []FlagChangeEvent
	client.OnChange(func(e FlagChangeEvent) {
		changes = append(changes, e)
	})

	err = client.UpdateContext(context.Background(), &EvaluationContext{
		UserID:     "user-2",
		Attributes: map[string]any{"plan": "enterprise"},
	})
	if err != nil {
		t.Fatalf("UpdateContext error: %v", err)
	}

	if got := client.BoolValue("dark-mode", false); got != true {
		t.Errorf("after update: BoolValue = %v, want true", got)
	}

	mu.Lock()
	if len(requests) < 2 {
		t.Fatalf("expected 2 requests, got %d", len(requests))
	}
	if requests[1].Context.UserID != "user-2" {
		t.Errorf("second request user_id = %q, want %q", requests[1].Context.UserID, "user-2")
	}
	mu.Unlock()

	if len(changes) != 1 {
		t.Fatalf("expected 1 change event, got %d", len(changes))
	}
	if changes[0].FlagKey != "dark-mode" {
		t.Errorf("change event flagKey = %q, want %q", changes[0].FlagKey, "dark-mode")
	}
}

func TestUpdateContext_MergesContext(t *testing.T) {
	var mu sync.Mutex
	var requests []evaluateRequest

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/evaluate" {
			var req evaluateRequest
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &req)
			mu.Lock()
			requests = append(requests, req)
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(evaluateResponse{Flags: map[string]*EvaluationResult{}})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	client, err := New(context.Background(), Config{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(false),
		Context:   &EvaluationContext{UserID: "user-1", Attributes: map[string]any{"plan": "pro"}},
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	err = client.UpdateContext(context.Background(), &EvaluationContext{
		Attributes: map[string]any{"tier": "gold"},
	})
	if err != nil {
		t.Fatalf("UpdateContext error: %v", err)
	}

	mu.Lock()
	if len(requests) < 2 {
		t.Fatalf("expected 2 requests, got %d", len(requests))
	}
	if requests[1].Context.UserID != "user-1" {
		t.Errorf("second request user_id = %q, want %q", requests[1].Context.UserID, "user-1")
	}
	mu.Unlock()
}

func TestGetContext(t *testing.T) {
	ts := newTestServer(map[string]*EvaluationResult{})
	defer ts.Close()

	client, err := New(context.Background(), Config{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(false),
		Context:   &EvaluationContext{UserID: "user-1", Attributes: map[string]any{"plan": "pro"}},
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	ctx := client.GetContext()
	if ctx.UserID != "user-1" {
		t.Errorf("GetContext UserID = %q, want %q", ctx.UserID, "user-1")
	}
	if ctx.Attributes["plan"] != "pro" {
		t.Errorf("GetContext Attributes = %v, want plan=pro", ctx.Attributes)
	}
}

func TestNew_DefaultContext(t *testing.T) {
	var mu sync.Mutex
	var requests []evaluateRequest

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/evaluate" {
			var req evaluateRequest
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &req)
			mu.Lock()
			requests = append(requests, req)
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(evaluateResponse{Flags: map[string]*EvaluationResult{}})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	client, err := New(context.Background(), Config{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	mu.Lock()
	if requests[0].Context.UserID != "" {
		t.Errorf("default user_id = %q, want empty", requests[0].Context.UserID)
	}
	if requests[0].Context.Attributes == nil {
		t.Fatal("default attributes is nil, want empty map")
	}
	mu.Unlock()
}

func TestClose_ClearsListeners(t *testing.T) {
	ts := newTestServer(map[string]*EvaluationResult{})
	defer ts.Close()

	client, err := New(context.Background(), Config{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	called := false
	client.OnChange(func(e FlagChangeEvent) { called = true })
	client.Close()

	client.events.emit(eventChange, FlagChangeEvent{FlagKey: "x"})
	if called {
		t.Fatal("listener called after Close()")
	}
}
