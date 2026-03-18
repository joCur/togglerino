package togglerino

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testDefinitionsServer creates an httptest.Server that serves flag definitions
// and an SSE stream. The definitions can be updated atomically for testing.
type testDefinitionsServer struct {
	*httptest.Server
	mu          sync.Mutex
	flags       []FlagDefinition
	segments    []SegmentDefinition
	fetchCount  atomic.Int64
	sseClients  chan chan struct{} // signals to close SSE connections
}

func newTestDefinitionsServer(flags []FlagDefinition, segments []SegmentDefinition) *testDefinitionsServer {
	ts := &testDefinitionsServer{
		flags:      flags,
		segments:   segments,
		sseClients: make(chan chan struct{}, 10),
	}
	ts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/definitions" && r.Method == http.MethodGet {
			ts.fetchCount.Add(1)
			ts.mu.Lock()
			resp := definitionsResponse{
				Flags:    ts.flags,
				Segments: ts.segments,
			}
			ts.mu.Unlock()
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
			// Block until the request context is canceled.
			<-r.Context().Done()
			return
		}
		http.NotFound(w, r)
	}))
	return ts
}

func (ts *testDefinitionsServer) setDefinitions(flags []FlagDefinition, segments []SegmentDefinition) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.flags = flags
	ts.segments = segments
}

func makeVariants(pairs ...any) []VariantDefinition {
	var variants []VariantDefinition
	for i := 0; i < len(pairs); i += 2 {
		name := pairs[i].(string)
		val := pairs[i+1]
		raw, _ := json.Marshal(val)
		variants = append(variants, VariantDefinition{Name: name, Value: raw})
	}
	return variants
}

func boolVariants() []VariantDefinition {
	return makeVariants("true", true, "false", false)
}

func intPtr(n int) *int { return &n }

func TestServerNew_FetchesDefinitions(t *testing.T) {
	ts := newTestDefinitionsServer(
		[]FlagDefinition{
			{
				Key:          "dark-mode",
				ValueType:    "boolean",
				Status:       "active",
				DefaultValue: false,
				Config: FlagDefinitionConfig{
					Enabled:            true,
					FallthroughVariant: "true",
					OffVariant:         "false",
					Variants:           boolVariants(),
				},
			},
			{
				Key:       "welcome-msg",
				ValueType: "string",
				Status:    "active",
				Config: FlagDefinitionConfig{
					Enabled:            true,
					FallthroughVariant: "greeting",
					Variants:           makeVariants("greeting", "Hello!"),
				},
			},
			{
				Key:       "max-uploads",
				ValueType: "number",
				Status:    "active",
				Config: FlagDefinitionConfig{
					Enabled:            true,
					FallthroughVariant: "ten",
					Variants:           makeVariants("ten", float64(10)),
				},
			},
		},
		nil,
	)
	defer ts.Close()

	server, err := NewServer(context.Background(), ServerConfig{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test123",
		Streaming: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}
	defer server.Close()

	eval := server.Evaluate(EvaluationContext{UserID: "user-1"})

	if got := eval.BoolValue("dark-mode", false); got != true {
		t.Errorf("BoolValue(dark-mode) = %v, want true", got)
	}
	if got := eval.StringValue("welcome-msg", ""); got != "Hello!" {
		t.Errorf("StringValue(welcome-msg) = %q, want %q", got, "Hello!")
	}
	if got := eval.NumberValue("max-uploads", 0); got != 10 {
		t.Errorf("NumberValue(max-uploads) = %v, want 10", got)
	}
}

func TestServerNew_ReturnsErrorOnFetchFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
	}))
	defer ts.Close()

	_, err := NewServer(context.Background(), ServerConfig{
		ServerURL: ts.URL,
		SDKKey:    "sdk_bad",
		Streaming: boolPtr(false),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestServerEvaluate_DifferentContextsProduceDifferentResults(t *testing.T) {
	pct := 50
	ts := newTestDefinitionsServer(
		[]FlagDefinition{
			{
				Key:       "feature-x",
				ValueType: "string",
				Status:    "active",
				Config: FlagDefinitionConfig{
					Enabled:            true,
					FallthroughVariant: "off",
					Variants:           makeVariants("on", "enabled", "off", "disabled"),
					TargetingRules: []TargetingRuleDefinition{
						{
							Variant:    "on",
							Percentage: &pct,
							Conditions: []ConditionDefinition{
								{Attribute: "plan", Operator: "equals", Value: "pro"},
							},
						},
					},
				},
			},
			{
				Key:       "admin-flag",
				ValueType: "string",
				Status:    "active",
				Config: FlagDefinitionConfig{
					Enabled:            true,
					FallthroughVariant: "off",
					Variants:           makeVariants("on", "admin-access", "off", "no-access"),
					TargetingRules: []TargetingRuleDefinition{
						{
							Variant: "on",
							Conditions: []ConditionDefinition{
								{Attribute: "role", Operator: "equals", Value: "admin"},
							},
						},
					},
				},
			},
		},
		nil,
	)
	defer ts.Close()

	server, err := NewServer(context.Background(), ServerConfig{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}
	defer server.Close()

	// Admin user gets admin-flag="admin-access" via targeting rule.
	adminEval := server.Evaluate(EvaluationContext{
		UserID:     "admin-1",
		Attributes: map[string]any{"role": "admin"},
	})
	if got := adminEval.StringValue("admin-flag", ""); got != "admin-access" {
		t.Errorf("admin StringValue(admin-flag) = %q, want %q", got, "admin-access")
	}

	// Regular user falls through to default variant = "no-access".
	userEval := server.Evaluate(EvaluationContext{
		UserID:     "user-1",
		Attributes: map[string]any{"role": "member"},
	})
	if got := userEval.StringValue("admin-flag", ""); got != "no-access" {
		t.Errorf("user StringValue(admin-flag) = %q, want %q", got, "no-access")
	}
}

func TestServerEvaluate_AllGetters(t *testing.T) {
	ts := newTestDefinitionsServer(
		[]FlagDefinition{
			{
				Key:          "bool-flag",
				ValueType:    "boolean",
				Status:       "active",
				DefaultValue: false,
				Config: FlagDefinitionConfig{
					Enabled:            true,
					FallthroughVariant: "true",
					OffVariant:         "false",
					Variants:           boolVariants(),
				},
			},
			{
				Key:       "str-flag",
				ValueType: "string",
				Status:    "active",
				Config: FlagDefinitionConfig{
					Enabled:            true,
					FallthroughVariant: "v1",
					Variants:           makeVariants("v1", "hello"),
				},
			},
			{
				Key:       "num-flag",
				ValueType: "number",
				Status:    "active",
				Config: FlagDefinitionConfig{
					Enabled:            true,
					FallthroughVariant: "v1",
					Variants:           makeVariants("v1", float64(42)),
				},
			},
			{
				Key:       "json-flag",
				ValueType: "json",
				Status:    "active",
				Config: FlagDefinitionConfig{
					Enabled:            true,
					FallthroughVariant: "v1",
					Variants:           makeVariants("v1", map[string]any{"key": "val"}),
				},
			},
		},
		nil,
	)
	defer ts.Close()

	server, err := NewServer(context.Background(), ServerConfig{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}
	defer server.Close()

	eval := server.Evaluate(EvaluationContext{UserID: "user-1"})

	// BoolValue
	if got := eval.BoolValue("bool-flag", false); got != true {
		t.Errorf("BoolValue = %v, want true", got)
	}

	// StringValue
	if got := eval.StringValue("str-flag", ""); got != "hello" {
		t.Errorf("StringValue = %q, want %q", got, "hello")
	}

	// NumberValue
	if got := eval.NumberValue("num-flag", 0); got != 42 {
		t.Errorf("NumberValue = %v, want 42", got)
	}

	// JSONValue
	var result map[string]string
	defaultVal := map[string]string{"key": "default"}
	err = eval.JSONValue("json-flag", &result, defaultVal)
	if err != nil {
		t.Fatalf("JSONValue error: %v", err)
	}
	if result["key"] != "val" {
		t.Errorf("JSONValue result = %v, want key=val", result)
	}

	// Detail
	detail, ok := eval.Detail("bool-flag")
	if !ok {
		t.Fatal("Detail returned not-ok for existing flag")
	}
	if detail.Reason != "default" {
		t.Errorf("Detail reason = %q, want %q", detail.Reason, "default")
	}
}

func TestServerEvaluate_DefaultValues(t *testing.T) {
	ts := newTestDefinitionsServer(nil, nil)
	defer ts.Close()

	server, err := NewServer(context.Background(), ServerConfig{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}
	defer server.Close()

	eval := server.Evaluate(EvaluationContext{UserID: "user-1"})

	if got := eval.BoolValue("unknown", false); got != false {
		t.Errorf("BoolValue default = %v, want false", got)
	}
	if got := eval.BoolValue("unknown", true); got != true {
		t.Errorf("BoolValue default = %v, want true", got)
	}
	if got := eval.StringValue("unknown", "fallback"); got != "fallback" {
		t.Errorf("StringValue default = %q, want %q", got, "fallback")
	}
	if got := eval.NumberValue("unknown", 42); got != 42 {
		t.Errorf("NumberValue default = %v, want 42", got)
	}

	var result map[string]string
	defaultVal := map[string]string{"key": "default"}
	err = eval.JSONValue("unknown", &result, defaultVal)
	if err != nil {
		t.Fatalf("JSONValue error: %v", err)
	}
	if result["key"] != "default" {
		t.Errorf("JSONValue default = %v, want key=default", result)
	}

	_, ok := eval.Detail("unknown")
	if ok {
		t.Fatal("Detail returned ok for nonexistent flag")
	}
}

func TestServerEvaluate_TypeMismatchReturnsDefault(t *testing.T) {
	ts := newTestDefinitionsServer(
		[]FlagDefinition{
			{
				Key:       "str-flag",
				ValueType: "string",
				Status:    "active",
				Config: FlagDefinitionConfig{
					Enabled:            true,
					FallthroughVariant: "v1",
					Variants:           makeVariants("v1", "hello"),
				},
			},
		},
		nil,
	)
	defer ts.Close()

	server, err := NewServer(context.Background(), ServerConfig{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}
	defer server.Close()

	eval := server.Evaluate(EvaluationContext{UserID: "user-1"})

	if got := eval.BoolValue("str-flag", false); got != false {
		t.Errorf("BoolValue on string flag = %v, want false", got)
	}
	if got := eval.NumberValue("str-flag", 0); got != 0 {
		t.Errorf("NumberValue on string flag = %v, want 0", got)
	}
}

func TestServerEvaluate_WithSegments(t *testing.T) {
	ts := newTestDefinitionsServer(
		[]FlagDefinition{
			{
				Key:       "premium-feature",
				ValueType: "string",
				Status:    "active",
				Config: FlagDefinitionConfig{
					Enabled:            true,
					FallthroughVariant: "basic",
					Variants:           makeVariants("premium", "premium-access", "basic", "basic-access"),
					TargetingRules: []TargetingRuleDefinition{
						{
							Variant: "premium",
							Conditions: []ConditionDefinition{
								{Attribute: "", Operator: "segment_match", Value: "premium-users"},
							},
						},
					},
				},
			},
		},
		[]SegmentDefinition{
			{
				Key: "premium-users",
				Conditions: []ConditionDefinition{
					{Attribute: "plan", Operator: "in", Value: `["pro","enterprise"]`},
				},
			},
		},
	)
	defer ts.Close()

	server, err := NewServer(context.Background(), ServerConfig{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}
	defer server.Close()

	// Pro user matches the segment and gets premium variant.
	proEval := server.Evaluate(EvaluationContext{
		UserID:     "user-1",
		Attributes: map[string]any{"plan": "pro"},
	})
	if got := proEval.StringValue("premium-feature", ""); got != "premium-access" {
		t.Errorf("pro user StringValue(premium-feature) = %q, want %q", got, "premium-access")
	}

	// Free user does not match the segment and gets basic variant.
	freeEval := server.Evaluate(EvaluationContext{
		UserID:     "user-2",
		Attributes: map[string]any{"plan": "free"},
	})
	if got := freeEval.StringValue("premium-feature", ""); got != "basic-access" {
		t.Errorf("free user StringValue(premium-feature) = %q, want %q", got, "basic-access")
	}
}

func TestServerClose(t *testing.T) {
	ts := newTestDefinitionsServer(
		[]FlagDefinition{
			{
				Key:          "flag-a",
				ValueType:    "boolean",
				Status:       "active",
				DefaultValue: false,
				Config: FlagDefinitionConfig{
					Enabled:            true,
					FallthroughVariant: "true",
					OffVariant:         "false",
					Variants:           boolVariants(),
				},
			},
		},
		nil,
	)
	defer ts.Close()

	server, err := NewServer(context.Background(), ServerConfig{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}

	// Close should not panic and should be safe to call multiple times.
	server.Close()
	server.Close()
}

func TestServerClose_SSE(t *testing.T) {
	ts := newTestDefinitionsServer(
		[]FlagDefinition{
			{
				Key:          "flag-a",
				ValueType:    "boolean",
				Status:       "active",
				DefaultValue: false,
				Config: FlagDefinitionConfig{
					Enabled:            true,
					FallthroughVariant: "true",
					OffVariant:         "false",
					Variants:           boolVariants(),
				},
			},
		},
		nil,
	)
	defer ts.Close()

	server, err := NewServer(context.Background(), ServerConfig{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(true),
	})
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}

	// Close should cleanly shut down the SSE connection.
	server.Close()
}

func TestServerPolling_RefetchesDefinitions(t *testing.T) {
	ts := newTestDefinitionsServer(
		[]FlagDefinition{
			{
				Key:          "flag-a",
				ValueType:    "boolean",
				Status:       "active",
				DefaultValue: false,
				Config: FlagDefinitionConfig{
					Enabled:            true,
					FallthroughVariant: "true",
					OffVariant:         "false",
					Variants:           boolVariants(),
				},
			},
		},
		nil,
	)
	defer ts.Close()

	server, err := NewServer(context.Background(), ServerConfig{
		ServerURL:       ts.URL,
		SDKKey:          "sdk_test",
		Streaming:       boolPtr(false),
		PollingInterval: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}
	defer server.Close()

	// Wait for at least one polling cycle.
	time.Sleep(150 * time.Millisecond)

	// Initial fetch (1) + at least 1 polling fetch.
	count := ts.fetchCount.Load()
	if count < 2 {
		t.Errorf("expected at least 2 definition fetches, got %d", count)
	}
}

func TestServerSSE_RefetchesOnFlagUpdate(t *testing.T) {
	var fetchCount atomic.Int64
	var mu sync.Mutex
	flags := []FlagDefinition{
		{
			Key:          "flag-a",
			ValueType:    "boolean",
			Status:       "active",
			DefaultValue: false,
			Config: FlagDefinitionConfig{
				Enabled:            true,
				FallthroughVariant: "true",
				OffVariant:         "false",
				Variants:           boolVariants(),
			},
		},
	}

	// Custom server that sends SSE events.
	sseCh := make(chan string, 10)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/definitions" && r.Method == http.MethodGet {
			fetchCount.Add(1)
			mu.Lock()
			resp := definitionsResponse{
				Flags:    flags,
				Segments: nil,
			}
			mu.Unlock()
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
			for {
				select {
				case event := <-sseCh:
					fmt.Fprint(w, event)
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}
				case <-r.Context().Done():
					return
				}
			}
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	server, err := NewServer(context.Background(), ServerConfig{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(true),
	})
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}
	defer server.Close()

	// Initial fetch should have happened.
	if got := fetchCount.Load(); got != 1 {
		t.Fatalf("expected 1 initial fetch, got %d", got)
	}

	// Update definitions and send SSE event.
	mu.Lock()
	flags = []FlagDefinition{
		{
			Key:          "flag-a",
			ValueType:    "boolean",
			Status:       "active",
			DefaultValue: false,
			Config: FlagDefinitionConfig{
				Enabled:            false,
				FallthroughVariant: "true",
				OffVariant:         "false",
				Variants:           boolVariants(),
			},
		},
	}
	mu.Unlock()

	sseCh <- "event: flag_update\ndata: {\"flagKey\":\"flag-a\"}\n\n"

	// Wait for the re-fetch to happen.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fetchCount.Load() >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if got := fetchCount.Load(); got < 2 {
		t.Fatalf("expected at least 2 fetches after SSE event, got %d", got)
	}

	// Verify the updated definitions are reflected.
	eval := server.Evaluate(EvaluationContext{UserID: "user-1"})
	if got := eval.BoolValue("flag-a", true); got != false {
		t.Errorf("BoolValue(flag-a) after update = %v, want false", got)
	}
}

func TestServerSSE_RefetchesOnFlagDeleted(t *testing.T) {
	var fetchCount atomic.Int64
	var mu sync.Mutex
	flags := []FlagDefinition{
		{
			Key:          "flag-a",
			ValueType:    "boolean",
			Status:       "active",
			DefaultValue: false,
			Config: FlagDefinitionConfig{
				Enabled:            true,
				FallthroughVariant: "true",
				OffVariant:         "false",
				Variants:           boolVariants(),
			},
		},
		{
			Key:          "flag-b",
			ValueType:    "boolean",
			Status:       "active",
			DefaultValue: false,
			Config: FlagDefinitionConfig{
				Enabled:            true,
				FallthroughVariant: "true",
				OffVariant:         "false",
				Variants:           boolVariants(),
			},
		},
	}

	sseCh := make(chan string, 10)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/definitions" && r.Method == http.MethodGet {
			fetchCount.Add(1)
			mu.Lock()
			resp := definitionsResponse{Flags: flags, Segments: nil}
			mu.Unlock()
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
			for {
				select {
				case event := <-sseCh:
					fmt.Fprint(w, event)
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}
				case <-r.Context().Done():
					return
				}
			}
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	server, err := NewServer(context.Background(), ServerConfig{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(true),
	})
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}
	defer server.Close()

	// Remove flag-b from definitions.
	mu.Lock()
	flags = []FlagDefinition{
		{
			Key:          "flag-a",
			ValueType:    "boolean",
			Status:       "active",
			DefaultValue: false,
			Config: FlagDefinitionConfig{
				Enabled:            true,
				FallthroughVariant: "true",
				OffVariant:         "false",
				Variants:           boolVariants(),
			},
		},
	}
	mu.Unlock()

	sseCh <- "event: flag_deleted\ndata: {\"flagKey\":\"flag-b\"}\n\n"

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fetchCount.Load() >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if got := fetchCount.Load(); got < 2 {
		t.Fatalf("expected at least 2 fetches after flag_deleted event, got %d", got)
	}

	// Verify flag-b is no longer present.
	eval := server.Evaluate(EvaluationContext{UserID: "user-1"})
	_, ok := eval.Detail("flag-b")
	if ok {
		t.Error("flag-b should not be present after deletion")
	}
	if got := eval.BoolValue("flag-a", false); got != true {
		t.Errorf("flag-a should still be present, got %v", got)
	}
}

func TestServerNew_StripsTrailingSlashes(t *testing.T) {
	var requestURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURL = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(definitionsResponse{})
	}))
	defer ts.Close()

	server, err := NewServer(context.Background(), ServerConfig{
		ServerURL: ts.URL + "///",
		SDKKey:    "sdk_test",
		Streaming: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}
	defer server.Close()

	if requestURL != "/api/v1/definitions" {
		t.Errorf("request URL = %q, want %q", requestURL, "/api/v1/definitions")
	}
}

func TestServerNew_SendsAuthorizationHeader(t *testing.T) {
	var authHeader string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(definitionsResponse{})
	}))
	defer ts.Close()

	server, err := NewServer(context.Background(), ServerConfig{
		ServerURL: ts.URL,
		SDKKey:    "sdk_my_key_123",
		Streaming: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}
	defer server.Close()

	if authHeader != "Bearer sdk_my_key_123" {
		t.Errorf("Authorization header = %q, want %q", authHeader, "Bearer sdk_my_key_123")
	}
}

func TestServerEvaluate_ConcurrentSafe(t *testing.T) {
	ts := newTestDefinitionsServer(
		[]FlagDefinition{
			{
				Key:          "flag-a",
				ValueType:    "boolean",
				Status:       "active",
				DefaultValue: false,
				Config: FlagDefinitionConfig{
					Enabled:            true,
					FallthroughVariant: "true",
					OffVariant:         "false",
					Variants:           boolVariants(),
				},
			},
		},
		nil,
	)
	defer ts.Close()

	server, err := NewServer(context.Background(), ServerConfig{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}
	defer server.Close()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			eval := server.Evaluate(EvaluationContext{
				UserID: fmt.Sprintf("user-%d", n),
			})
			eval.BoolValue("flag-a", false)
		}(i)
	}
	wg.Wait()
}

func TestServerEvaluate_DisabledFlag(t *testing.T) {
	ts := newTestDefinitionsServer(
		[]FlagDefinition{
			{
				Key:          "disabled-flag",
				ValueType:    "boolean",
				Status:       "active",
				DefaultValue: false,
				Config: FlagDefinitionConfig{
					Enabled:            false,
					FallthroughVariant: "true",
					OffVariant:         "false",
					Variants:           boolVariants(),
				},
			},
		},
		nil,
	)
	defer ts.Close()

	server, err := NewServer(context.Background(), ServerConfig{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}
	defer server.Close()

	eval := server.Evaluate(EvaluationContext{UserID: "user-1"})

	if got := eval.BoolValue("disabled-flag", true); got != false {
		t.Errorf("BoolValue(disabled-flag) = %v, want false (disabled)", got)
	}

	detail, ok := eval.Detail("disabled-flag")
	if !ok {
		t.Fatal("Detail returned not-ok for disabled flag")
	}
	if detail.Reason != "disabled" {
		t.Errorf("Detail reason = %q, want %q", detail.Reason, "disabled")
	}
}

func TestServerEvaluate_NilAttributes(t *testing.T) {
	ts := newTestDefinitionsServer(
		[]FlagDefinition{
			{
				Key:          "flag-a",
				ValueType:    "boolean",
				Status:       "active",
				DefaultValue: false,
				Config: FlagDefinitionConfig{
					Enabled:            true,
					FallthroughVariant: "true",
					OffVariant:         "false",
					Variants:           boolVariants(),
				},
			},
		},
		nil,
	)
	defer ts.Close()

	server, err := NewServer(context.Background(), ServerConfig{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}
	defer server.Close()

	// Should not panic with nil Attributes.
	eval := server.Evaluate(EvaluationContext{UserID: "user-1"})
	if got := eval.BoolValue("flag-a", false); got != true {
		t.Errorf("BoolValue = %v, want true", got)
	}
}
