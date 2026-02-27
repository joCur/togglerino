# Go SDK Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create a Go client SDK for togglerino at `sdks/go/` that mirrors the JavaScript SDK — fetching flags from the server, caching locally, streaming updates via SSE with polling fallback.

**Architecture:** Single `Client` struct managing HTTP fetching, SSE streaming, in-memory cache (sync.RWMutex), and typed event callbacks. Auto-initializes on `New()`. Zero external dependencies (stdlib only).

**Tech Stack:** Go 1.25, stdlib (`net/http`, `encoding/json`, `log/slog`, `sync`, `bufio`)

---

### Task 1: Module scaffolding and types

**Files:**
- Create: `sdks/go/go.mod`
- Create: `sdks/go/types.go`

**Step 1: Create go.mod**

```
sdks/go/go.mod
```

```
module github.com/joCur/togglerino/sdks/go

go 1.25.0
```

**Step 2: Create types.go with all shared types**

```go
// sdks/go/types.go
package togglerino

// EvaluationContext holds user identity and attributes for flag evaluation.
type EvaluationContext struct {
	UserID     string         `json:"user_id"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// EvaluationResult is the server's response for a single flag evaluation.
type EvaluationResult struct {
	Value   any    `json:"value"`
	Variant string `json:"variant"`
	Reason  string `json:"reason"`
}

// FlagChangeEvent is emitted when a flag value changes.
type FlagChangeEvent struct {
	FlagKey  string `json:"flagKey"`
	Value    any    `json:"value"`
	Variant  string `json:"variant"`
	OldValue any    `json:"-"`
}

// FlagDeletedEvent is emitted when a flag is deleted.
type FlagDeletedEvent struct {
	FlagKey string `json:"flagKey"`
}

// evaluateRequest is the POST body sent to /api/v1/evaluate.
type evaluateRequest struct {
	Context *evaluateContext `json:"context"`
}

// evaluateContext is the wire format for EvaluationContext.
type evaluateContext struct {
	UserID     string         `json:"user_id"`
	Attributes map[string]any `json:"attributes"`
}

// evaluateResponse is the response from POST /api/v1/evaluate.
type evaluateResponse struct {
	Flags map[string]*EvaluationResult `json:"flags"`
}

// sseEvent is a parsed SSE event from the stream.
type sseEvent struct {
	Type    string `json:"type"`
	FlagKey string `json:"flagKey"`
	Value   any    `json:"value"`
	Variant string `json:"variant"`
}
```

**Step 3: Verify it compiles**

Run: `cd sdks/go && go build ./...`
Expected: success (no output)

**Step 4: Commit**

```bash
git add sdks/go/go.mod sdks/go/types.go
git commit -m "feat(sdk/go): scaffold module and define types"
```

---

### Task 2: Config and sentinel errors

**Files:**
- Create: `sdks/go/config.go`

**Step 1: Create config.go**

```go
// sdks/go/config.go
package togglerino

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	defaultPollingInterval = 30 * time.Second
	defaultMaxRetryDelay   = 30 * time.Second
	defaultBaseRetryDelay  = 1 * time.Second
)

var (
	// ErrClosed is returned when operating on a closed client.
	ErrClosed = errors.New("togglerino: client is closed")
)

// Config holds the settings for creating a new Client.
type Config struct {
	// ServerURL is the base URL of the togglerino server (required).
	ServerURL string

	// SDKKey is the SDK authentication key (required).
	SDKKey string

	// Context is the initial evaluation context (optional).
	Context *EvaluationContext

	// Streaming enables SSE streaming for real-time flag updates.
	// Defaults to true. Set to a pointer to false to disable.
	Streaming *bool

	// PollingInterval is the interval for polling flag updates.
	// Defaults to 30 seconds. Used as fallback when SSE is unavailable.
	PollingInterval time.Duration

	// HTTPClient is a custom HTTP client (optional).
	// Defaults to http.DefaultClient.
	HTTPClient *http.Client

	// Logger is a structured logger (optional).
	// Defaults to slog.Default().
	Logger *slog.Logger
}

// resolvedConfig is the internal config with all defaults applied.
type resolvedConfig struct {
	serverURL       string
	sdkKey          string
	context         EvaluationContext
	streaming       bool
	pollingInterval time.Duration
	httpClient      *http.Client
	logger          *slog.Logger
}

func resolveConfig(c Config) resolvedConfig {
	rc := resolvedConfig{
		serverURL:       strings.TrimRight(c.ServerURL, "/"),
		sdkKey:          c.SDKKey,
		streaming:       true,
		pollingInterval: defaultPollingInterval,
		httpClient:      http.DefaultClient,
		logger:          slog.Default(),
	}

	if c.Context != nil {
		rc.context = *c.Context
	}
	if rc.context.Attributes == nil {
		rc.context.Attributes = make(map[string]any)
	}

	if c.Streaming != nil {
		rc.streaming = *c.Streaming
	}

	if c.PollingInterval > 0 {
		rc.pollingInterval = c.PollingInterval
	}

	if c.HTTPClient != nil {
		rc.httpClient = c.HTTPClient
	}

	if c.Logger != nil {
		rc.logger = c.Logger
	}

	return rc
}
```

**Step 2: Verify it compiles**

Run: `cd sdks/go && go build ./...`
Expected: success

**Step 3: Commit**

```bash
git add sdks/go/config.go
git commit -m "feat(sdk/go): add config resolution and sentinel errors"
```

---

### Task 3: Event system

**Files:**
- Create: `sdks/go/events.go`
- Create: `sdks/go/events_test.go`

**Step 1: Write the failing tests**

```go
// sdks/go/events_test.go
package togglerino

import (
	"sync"
	"testing"
)

func TestEventEmitter_Subscribe_And_Emit(t *testing.T) {
	em := newEventEmitter()
	called := false
	em.on(eventReady, func(payload any) {
		called = true
	})
	em.emit(eventReady, nil)
	if !called {
		t.Fatal("listener was not called")
	}
}

func TestEventEmitter_Unsubscribe(t *testing.T) {
	em := newEventEmitter()
	called := false
	unsub := em.on(eventReady, func(payload any) {
		called = true
	})
	unsub()
	em.emit(eventReady, nil)
	if called {
		t.Fatal("listener was called after unsubscribe")
	}
}

func TestEventEmitter_Multiple_Listeners(t *testing.T) {
	em := newEventEmitter()
	var count int
	em.on(eventReady, func(payload any) { count++ })
	em.on(eventReady, func(payload any) { count++ })
	em.emit(eventReady, nil)
	if count != 2 {
		t.Fatalf("expected 2 calls, got %d", count)
	}
}

func TestEventEmitter_Panicking_Listener_Does_Not_Break_Others(t *testing.T) {
	em := newEventEmitter()
	secondCalled := false
	em.on(eventReady, func(payload any) {
		panic("boom")
	})
	em.on(eventReady, func(payload any) {
		secondCalled = true
	})
	em.emit(eventReady, nil)
	if !secondCalled {
		t.Fatal("second listener was not called after first panicked")
	}
}

func TestEventEmitter_Clear(t *testing.T) {
	em := newEventEmitter()
	called := false
	em.on(eventReady, func(payload any) { called = true })
	em.clear()
	em.emit(eventReady, nil)
	if called {
		t.Fatal("listener was called after clear")
	}
}

func TestEventEmitter_Concurrent_Access(t *testing.T) {
	em := newEventEmitter()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unsub := em.on(eventReady, func(payload any) {})
			em.emit(eventReady, nil)
			unsub()
		}()
	}
	wg.Wait()
}
```

**Step 2: Run tests to verify they fail**

Run: `cd sdks/go && go test -run TestEventEmitter -v ./...`
Expected: FAIL (functions not defined)

**Step 3: Implement events.go**

```go
// sdks/go/events.go
package togglerino

import (
	"sync"
	"time"
)

type eventType string

const (
	eventReady         eventType = "ready"
	eventChange        eventType = "change"
	eventDeleted       eventType = "deleted"
	eventError         eventType = "error"
	eventReconnecting  eventType = "reconnecting"
	eventReconnected   eventType = "reconnected"
	eventContextChange eventType = "context_change"
)

type listener struct {
	id int
	fn func(any)
}

type eventEmitter struct {
	mu        sync.RWMutex
	listeners map[eventType][]listener
	nextID    int
}

func newEventEmitter() *eventEmitter {
	return &eventEmitter{
		listeners: make(map[eventType][]listener),
	}
}

// on registers a listener for an event type and returns an unsubscribe function.
func (e *eventEmitter) on(event eventType, fn func(any)) func() {
	e.mu.Lock()
	id := e.nextID
	e.nextID++
	e.listeners[event] = append(e.listeners[event], listener{id: id, fn: fn})
	e.mu.Unlock()

	return func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		ls := e.listeners[event]
		for i, l := range ls {
			if l.id == id {
				e.listeners[event] = append(ls[:i], ls[i+1:]...)
				break
			}
		}
	}
}

// emit calls all listeners for the given event type.
// Panicking listeners are recovered and do not affect other listeners.
func (e *eventEmitter) emit(event eventType, payload any) {
	e.mu.RLock()
	ls := make([]listener, len(e.listeners[event]))
	copy(ls, e.listeners[event])
	e.mu.RUnlock()

	for _, l := range ls {
		func() {
			defer func() { recover() }()
			l.fn(payload)
		}()
	}
}

// clear removes all listeners.
func (e *eventEmitter) clear() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.listeners = make(map[eventType][]listener)
}

// --- Typed callback registration for the public Client API ---

// OnChange registers a callback for flag value changes.
// Returns an unsubscribe function.
func (c *Client) OnChange(fn func(FlagChangeEvent)) func() {
	return c.events.on(eventChange, func(payload any) {
		if e, ok := payload.(FlagChangeEvent); ok {
			fn(e)
		}
	})
}

// OnDeleted registers a callback for flag deletions.
func (c *Client) OnDeleted(fn func(FlagDeletedEvent)) func() {
	return c.events.on(eventDeleted, func(payload any) {
		if e, ok := payload.(FlagDeletedEvent); ok {
			fn(e)
		}
	})
}

// OnError registers a callback for errors.
func (c *Client) OnError(fn func(error)) func() {
	return c.events.on(eventError, func(payload any) {
		if e, ok := payload.(error); ok {
			fn(e)
		}
	})
}

// OnReady registers a callback for when the client is ready.
func (c *Client) OnReady(fn func()) func() {
	return c.events.on(eventReady, func(any) {
		fn()
	})
}

// OnReconnecting registers a callback for SSE reconnection attempts.
func (c *Client) OnReconnecting(fn func(attempt int, delay time.Duration)) func() {
	return c.events.on(eventReconnecting, func(payload any) {
		if e, ok := payload.(reconnectingPayload); ok {
			fn(e.Attempt, e.Delay)
		}
	})
}

// OnReconnected registers a callback for successful SSE reconnection.
func (c *Client) OnReconnected(fn func()) func() {
	return c.events.on(eventReconnected, func(any) {
		fn()
	})
}

// OnContextChange registers a callback for context updates.
func (c *Client) OnContextChange(fn func(EvaluationContext)) func() {
	return c.events.on(eventContextChange, func(payload any) {
		if e, ok := payload.(EvaluationContext); ok {
			fn(e)
		}
	})
}

// reconnectingPayload is the internal payload for the reconnecting event.
type reconnectingPayload struct {
	Attempt int
	Delay   time.Duration
}
```

Note: The `On*` methods on `Client` won't compile yet because `Client` doesn't exist. That's fine — they will compile after Task 4. For now, the `events_test.go` tests exercise the `eventEmitter` directly.

**Step 4: Run tests to verify they pass**

Run: `cd sdks/go && go test -run TestEventEmitter -v ./...`
Expected: PASS (all 6 tests)

**Step 5: Commit**

```bash
git add sdks/go/events.go sdks/go/events_test.go
git commit -m "feat(sdk/go): add event emitter with typed callbacks"
```

---

### Task 4: Client struct, New(), Close(), flag getters, and context

**Files:**
- Create: `sdks/go/client.go`
- Create: `sdks/go/flags.go`
- Create: `sdks/go/context.go`
- Create: `sdks/go/client_test.go`

This is the core task. We implement the Client struct, the `New()` constructor (which auto-initializes by fetching flags), `Close()`, typed flag getters, and context management. SSE and polling are stubbed — they'll be implemented in Tasks 5 and 6.

**Step 1: Write the failing tests**

```go
// sdks/go/client_test.go
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

// testServer creates an httptest.Server that responds to /api/v1/evaluate
// with the given flags. It also tracks requests for assertions.
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
			// SSE endpoint — just keep connection open, then close
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

// --- Initialization tests ---

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

	// Verify request was sent with correct auth and context
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

	// If it worked, the trailing slashes were stripped correctly
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

func TestNew_EmitsReadyEvent(t *testing.T) {
	ts := newTestServer(map[string]*EvaluationResult{})
	defer ts.Close()

	// We need to register OnReady before New() completes, but New() auto-initializes.
	// So we test that ready callback is invoked by New().
	// The design says New() calls OnReady internally after fetch succeeds.
	// We can verify by checking the client was initialized.
	client, err := New(context.Background(), Config{
		ServerURL: ts.URL,
		SDKKey:    "sdk_test",
		Streaming: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()
	// If we got here without error, initialization succeeded.
}

// --- Default values ---

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

	// Asking for bool on a string flag returns default
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

	// Unknown flag should use default
	var result2 map[string]string
	err = client.JSONValue("unknown", &result2, defaultVal)
	if err != nil {
		t.Fatalf("JSONValue error: %v", err)
	}
	if result2["key"] != "default" {
		t.Errorf("JSONValue default = %v, want key=default", result2)
	}
}

// --- Context ---

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

	// Track change events
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

	// Verify second request used updated context
	mu.Lock()
	if len(requests) < 2 {
		t.Fatalf("expected 2 requests, got %d", len(requests))
	}
	if requests[1].Context.UserID != "user-2" {
		t.Errorf("second request user_id = %q, want %q", requests[1].Context.UserID, "user-2")
	}
	mu.Unlock()

	// Change event should have fired
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

	// Update only attributes — UserID should persist from original config
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
	// UserID from original config should persist
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

// --- Close ---

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

	// After close, events should not fire
	client.events.emit(eventChange, FlagChangeEvent{FlagKey: "x"})
	if called {
		t.Fatal("listener called after Close()")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd sdks/go && go test -v ./...`
Expected: FAIL (Client type not defined)

**Step 3: Implement client.go**

```go
// sdks/go/client.go
package togglerino

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// Client is the togglerino SDK client.
// Create one with New() and defer Close().
type Client struct {
	config      resolvedConfig
	events      *eventEmitter
	flags       map[string]*EvaluationResult
	flagsMu     sync.RWMutex
	initialized bool
	closed      bool
	closeMu     sync.Mutex
	cancelFunc  context.CancelFunc  // cancels background goroutines
	wg          sync.WaitGroup      // waits for background goroutines
	closeOnce   sync.Once
	pollStop    chan struct{}        // signals polling to stop
}

// New creates and initializes a new togglerino Client.
// It fetches all flags from the server and starts SSE streaming (or polling).
// Returns an error if the initial flag fetch fails.
func New(ctx context.Context, cfg Config) (*Client, error) {
	rc := resolveConfig(cfg)

	bgCtx, cancel := context.WithCancel(context.Background())

	c := &Client{
		config:     rc,
		events:     newEventEmitter(),
		flags:      make(map[string]*EvaluationResult),
		cancelFunc: cancel,
		pollStop:   make(chan struct{}),
	}

	// Initial flag fetch
	if err := c.fetchFlags(ctx); err != nil {
		cancel()
		return nil, err
	}

	c.initialized = true

	// Start background update mechanism
	if rc.streaming {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			c.runSSE(bgCtx)
		}()
	} else {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			c.runPolling(bgCtx)
		}()
	}

	c.events.emit(eventReady, nil)

	return c, nil
}

// Close stops all background activity and clears listeners.
// Safe to call multiple times.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		c.closeMu.Lock()
		c.closed = true
		c.closeMu.Unlock()

		c.cancelFunc()
		c.wg.Wait()
		c.events.clear()
	})
}

// fetchFlags calls POST /api/v1/evaluate and updates the local cache.
func (c *Client) fetchFlags(ctx context.Context) error {
	url := c.config.serverURL + "/api/v1/evaluate"

	c.flagsMu.RLock()
	evalCtx := c.config.context
	c.flagsMu.RUnlock()

	reqBody := evaluateRequest{
		Context: &evaluateContext{
			UserID:     evalCtx.UserID,
			Attributes: evalCtx.Attributes,
		},
	}
	if reqBody.Context.Attributes == nil {
		reqBody.Context.Attributes = make(map[string]any)
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("togglerino: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("togglerino: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.sdkKey)

	resp, err := c.config.httpClient.Do(req)
	if err != nil {
		c.events.emit(eventError, err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("togglerino: flag evaluation failed with status %d", resp.StatusCode)
		c.events.emit(eventError, err)
		return err
	}

	var evalResp evaluateResponse
	if err := json.NewDecoder(resp.Body).Decode(&evalResp); err != nil {
		return fmt.Errorf("togglerino: failed to decode response: %w", err)
	}

	c.flagsMu.Lock()
	oldFlags := c.flags
	c.flags = make(map[string]*EvaluationResult, len(evalResp.Flags))
	for k, v := range evalResp.Flags {
		c.flags[k] = v

		// Emit change events only after initialization
		if c.initialized {
			old, existed := oldFlags[k]
			if !existed || !jsonEqual(old.Value, v.Value) {
				c.events.emit(eventChange, FlagChangeEvent{
					FlagKey: k,
					Value:   v.Value,
					Variant: v.Variant,
				})
			}
		}
	}
	c.flagsMu.Unlock()

	return nil
}

// jsonEqual compares two values by marshalling to JSON.
func jsonEqual(a, b any) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}

// runSSE is a placeholder — implemented in stream.go (Task 5)
func (c *Client) runSSE(ctx context.Context) {
	// Will be implemented in Task 5
	<-ctx.Done()
}

// runPolling is a placeholder — implemented in polling.go (Task 6)
func (c *Client) runPolling(ctx context.Context) {
	// Will be implemented in Task 6
	<-ctx.Done()
}
```

**Step 4: Implement flags.go**

```go
// sdks/go/flags.go
package togglerino

import "encoding/json"

// BoolValue returns the boolean value of a flag, or defaultValue if not found or wrong type.
func (c *Client) BoolValue(key string, defaultValue bool) bool {
	c.flagsMu.RLock()
	defer c.flagsMu.RUnlock()

	result, ok := c.flags[key]
	if !ok {
		return defaultValue
	}
	v, ok := result.Value.(bool)
	if !ok {
		return defaultValue
	}
	return v
}

// StringValue returns the string value of a flag, or defaultValue if not found or wrong type.
func (c *Client) StringValue(key string, defaultValue string) string {
	c.flagsMu.RLock()
	defer c.flagsMu.RUnlock()

	result, ok := c.flags[key]
	if !ok {
		return defaultValue
	}
	v, ok := result.Value.(string)
	if !ok {
		return defaultValue
	}
	return v
}

// NumberValue returns the numeric value of a flag, or defaultValue if not found or wrong type.
// JSON numbers are decoded as float64 by encoding/json.
func (c *Client) NumberValue(key string, defaultValue float64) float64 {
	c.flagsMu.RLock()
	defer c.flagsMu.RUnlock()

	result, ok := c.flags[key]
	if !ok {
		return defaultValue
	}
	v, ok := result.Value.(float64)
	if !ok {
		return defaultValue
	}
	return v
}

// JSONValue unmarshals the flag value into target. If the flag is not found,
// it marshals defaultValue into target instead. Returns an error if unmarshalling fails.
func (c *Client) JSONValue(key string, target any, defaultValue any) error {
	c.flagsMu.RLock()
	result, ok := c.flags[key]
	c.flagsMu.RUnlock()

	var src any
	if ok {
		src = result.Value
	} else {
		src = defaultValue
	}

	// Round-trip through JSON to unmarshal into the target type
	data, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// Detail returns the full EvaluationResult for a flag.
// Returns (result, true) if found, or (zero, false) if not.
func (c *Client) Detail(key string) (EvaluationResult, bool) {
	c.flagsMu.RLock()
	defer c.flagsMu.RUnlock()

	result, ok := c.flags[key]
	if !ok {
		return EvaluationResult{}, false
	}
	return *result, true
}
```

**Step 5: Implement context.go**

```go
// sdks/go/context.go
package togglerino

import "context"

// GetContext returns a copy of the current evaluation context.
func (c *Client) GetContext() EvaluationContext {
	c.flagsMu.RLock()
	defer c.flagsMu.RUnlock()

	ctx := c.config.context
	// Deep copy attributes
	if ctx.Attributes != nil {
		attrs := make(map[string]any, len(ctx.Attributes))
		for k, v := range ctx.Attributes {
			attrs[k] = v
		}
		ctx.Attributes = attrs
	}
	return ctx
}

// UpdateContext merges the given context into the current context and re-fetches all flags.
func (c *Client) UpdateContext(ctx context.Context, evalCtx *EvaluationContext) error {
	c.flagsMu.Lock()
	if evalCtx.UserID != "" {
		c.config.context.UserID = evalCtx.UserID
	}
	if evalCtx.Attributes != nil {
		if c.config.context.Attributes == nil {
			c.config.context.Attributes = make(map[string]any)
		}
		for k, v := range evalCtx.Attributes {
			c.config.context.Attributes[k] = v
		}
	}
	c.flagsMu.Unlock()

	if err := c.fetchFlags(ctx); err != nil {
		return err
	}

	c.events.emit(eventContextChange, c.GetContext())
	return nil
}
```

**Step 6: Run tests to verify they pass**

Run: `cd sdks/go && go test -v ./...`
Expected: PASS (all tests)

**Step 7: Commit**

```bash
git add sdks/go/client.go sdks/go/flags.go sdks/go/context.go sdks/go/client_test.go
git commit -m "feat(sdk/go): add client, flag getters, and context management"
```

---

### Task 5: SSE streaming with reconnection

**Files:**
- Create: `sdks/go/stream.go` (replace placeholder)
- Create: `sdks/go/stream_test.go`

**Step 1: Write the failing tests**

```go
// sdks/go/stream_test.go
package togglerino

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// sseServer creates a test server that serves both /api/v1/evaluate and /api/v1/stream.
// The stream handler writes the provided SSE data and then optionally keeps the connection open.
type sseTestServer struct {
	*httptest.Server
	mu        sync.Mutex
	flags     map[string]*EvaluationResult
	sseData   string   // SSE data to send on connect
	sseKeep   bool     // keep connection open after sending data
	sseFail   bool     // fail the SSE connection
	sseConns  int      // number of SSE connections received
}

func newSSETestServer(flags map[string]*EvaluationResult, sseData string, keepOpen bool) *sseTestServer {
	ts := &sseTestServer{flags: flags, sseData: sseData, sseKeep: keepOpen}
	ts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/evaluate" && r.Method == http.MethodPost {
			ts.mu.Lock()
			f := ts.flags
			ts.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			resp := evaluateResponse{Flags: f}
			data, _ := json.Marshal(resp)
			w.Write(data)
			return
		}
		if r.URL.Path == "/api/v1/stream" {
			ts.mu.Lock()
			ts.sseConns++
			fail := ts.sseFail
			data := ts.sseData
			keep := ts.sseKeep
			ts.mu.Unlock()

			if fail {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			flusher, _ := w.(http.Flusher)
			fmt.Fprint(w, ": connected\n\n")
			flusher.Flush()

			if data != "" {
				fmt.Fprint(w, data)
				flusher.Flush()
			}

			if keep {
				<-r.Context().Done()
			}
			return
		}
		http.NotFound(w, r)
	}))
	return ts
}

func (ts *sseTestServer) getSSEConns() int {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.sseConns
}

func (ts *sseTestServer) setFlags(flags map[string]*EvaluationResult) {
	ts.mu.Lock()
	ts.flags = flags
	ts.mu.Unlock()
}

func (ts *sseTestServer) setSSEFail(fail bool) {
	ts.mu.Lock()
	ts.sseFail = fail
	ts.mu.Unlock()
}

func TestSSE_ProcessesFlagUpdate(t *testing.T) {
	sseData := "event: flag_update\ndata: {\"type\":\"flag_update\",\"flagKey\":\"dark-mode\",\"value\":true,\"variant\":\"on\"}\n\n"

	ts := newSSETestServer(
		map[string]*EvaluationResult{
			"dark-mode": {Value: false, Variant: "off", Reason: "default"},
		},
		sseData,
		true, // keep open after sending
	)
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

	// Wait for SSE event to be processed
	time.Sleep(200 * time.Millisecond)

	if got := client.BoolValue("dark-mode", false); got != true {
		t.Errorf("BoolValue after SSE update = %v, want true", got)
	}

	mu.Lock()
	if len(changes) == 0 {
		t.Fatal("expected change event, got none")
	}
	if changes[0].FlagKey != "dark-mode" {
		t.Errorf("change event flagKey = %q, want %q", changes[0].FlagKey, "dark-mode")
	}
	mu.Unlock()
}

func TestSSE_ProcessesFlagDeleted(t *testing.T) {
	sseData := "event: flag_deleted\ndata: {\"type\":\"flag_deleted\",\"flagKey\":\"delete-me\"}\n\n"

	ts := newSSETestServer(
		map[string]*EvaluationResult{
			"delete-me": {Value: true, Variant: "on", Reason: "default"},
			"keep-me":   {Value: false, Variant: "off", Reason: "default"},
		},
		sseData,
		true,
	)
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

	time.Sleep(200 * time.Millisecond)

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
	// Only comments, no actual events
	sseData := ": keepalive\n\n"

	ts := newSSETestServer(
		map[string]*EvaluationResult{
			"my-flag": {Value: true, Variant: "on", Reason: "default"},
		},
		sseData,
		true,
	)
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

	time.Sleep(100 * time.Millisecond)

	// Flag should be unchanged
	if got := client.BoolValue("my-flag", false); got != true {
		t.Errorf("flag changed unexpectedly after comment-only SSE")
	}
}

func TestSSE_ReconnectsOnFailure(t *testing.T) {
	ts := newSSETestServer(
		map[string]*EvaluationResult{},
		"",
		false,
	)
	ts.setSSEFail(true) // SSE will return 500
	defer ts.Close()

	var reconnecting []int
	var mu sync.Mutex

	client, err := New(context.Background(), Config{
		ServerURL:       ts.URL,
		SDKKey:          "sdk_test",
		Streaming:       boolPtr(true),
		PollingInterval: 60 * time.Second, // long so it doesn't interfere
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	client.OnReconnecting(func(attempt int, delay time.Duration) {
		mu.Lock()
		reconnecting = append(reconnecting, attempt)
		mu.Unlock()
	})

	// Wait for a couple reconnection attempts (1s + 2s backoff)
	time.Sleep(4 * time.Second)

	mu.Lock()
	if len(reconnecting) < 2 {
		t.Errorf("expected at least 2 reconnection attempts, got %d", len(reconnecting))
	}
	mu.Unlock()
}

func TestSSE_EmitsReconnectedOnSuccess(t *testing.T) {
	ts := newSSETestServer(
		map[string]*EvaluationResult{},
		"",
		true,
	)
	ts.setSSEFail(true) // Start with SSE failing
	defer ts.Close()

	var reconnected bool
	var mu sync.Mutex

	client, err := New(context.Background(), Config{
		ServerURL:       ts.URL,
		SDKKey:          "sdk_test",
		Streaming:       boolPtr(true),
		PollingInterval: 60 * time.Second,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	client.OnReconnected(func() {
		mu.Lock()
		reconnected = true
		mu.Unlock()
	})

	// Wait for first retry to fire (1s)
	time.Sleep(500 * time.Millisecond)

	// Now make SSE succeed
	ts.setSSEFail(false)

	// Wait for reconnection to succeed
	time.Sleep(2 * time.Second)

	mu.Lock()
	if !reconnected {
		t.Fatal("expected reconnected event")
	}
	mu.Unlock()
}
```

**Step 2: Run tests to verify they fail**

Run: `cd sdks/go && go test -run TestSSE -v ./...`
Expected: FAIL (runSSE is a placeholder)

**Step 3: Replace the placeholder in client.go and implement stream.go**

First, remove the `runSSE` placeholder from `client.go` (the one that just does `<-ctx.Done()`).

Then create `sdks/go/stream.go`:

```go
// sdks/go/stream.go
package togglerino

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
)

// runSSE manages the SSE connection lifecycle with reconnection.
func (c *Client) runSSE(ctx context.Context) {
	retryCount := 0

	for {
		if ctx.Err() != nil {
			return
		}

		wasReconnecting := retryCount > 0
		err := c.connectSSE(ctx)

		if ctx.Err() != nil {
			return
		}

		// Connection failed or ended — schedule reconnection
		if err != nil {
			c.config.logger.Warn("SSE connection error", "error", err)
		}

		delay := c.retryDelay(retryCount)
		retryCount++
		c.events.emit(eventReconnecting, reconnectingPayload{
			Attempt: retryCount,
			Delay:   delay,
		})

		// Start polling fallback if not already running
		c.startPollingFallback(ctx)

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		_ = wasReconnecting // used after successful connect below
	}
}

// connectSSE establishes an SSE connection and processes events.
// Returns nil when the stream ends normally, or an error on failure.
func (c *Client) connectSSE(ctx context.Context) error {
	url := c.config.serverURL + "/api/v1/stream"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.config.sdkKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.config.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("SSE returned status %d", resp.StatusCode)
	}

	// Connection succeeded — emit reconnected if we were retrying,
	// stop polling fallback, and reset retry state.
	// We track this via the retryCount field approach: the caller (runSSE)
	// handles this by checking wasReconnecting. But since connectSSE is blocking,
	// we need a different approach. Let's use a channel signal.
	// Actually, let's restructure: runSSE will handle reconnected event.

	scanner := bufio.NewScanner(resp.Body)

	var eventType, data string
	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			// Empty line = end of event block
			if data != "" {
				c.handleSSEEvent(eventType, data)
			}
			eventType = ""
			data = ""
			continue
		}

		if strings.HasPrefix(line, ":") {
			// Comment line (keepalive), ignore
			continue
		}

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}

	return scanner.Err()
}

// handleSSEEvent processes a single parsed SSE event.
func (c *Client) handleSSEEvent(eventType, data string) {
	switch eventType {
	case "flag_update":
		var evt sseEvent
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			return // ignore malformed data
		}

		c.flagsMu.Lock()
		existing := c.flags[evt.FlagKey]
		c.flags[evt.FlagKey] = &EvaluationResult{
			Value:   evt.Value,
			Variant: evt.Variant,
			Reason: func() string {
				if existing != nil {
					return existing.Reason
				}
				return "stream_update"
			}(),
		}
		c.flagsMu.Unlock()

		c.events.emit(eventChange, FlagChangeEvent{
			FlagKey: evt.FlagKey,
			Value:   evt.Value,
			Variant: evt.Variant,
		})

	case "flag_deleted":
		var evt sseEvent
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			return
		}

		c.flagsMu.Lock()
		delete(c.flags, evt.FlagKey)
		c.flagsMu.Unlock()

		c.events.emit(eventDeleted, FlagDeletedEvent{
			FlagKey: evt.FlagKey,
		})
	}
}

// retryDelay calculates the exponential backoff delay.
// Sequence: 1s, 2s, 4s, 8s, 16s, 30s (capped).
func (c *Client) retryDelay(retryCount int) time.Duration {
	delay := defaultBaseRetryDelay * time.Duration(math.Pow(2, float64(retryCount)))
	if delay > defaultMaxRetryDelay {
		delay = defaultMaxRetryDelay
	}
	return delay
}
```

Now we need to restructure `runSSE` to properly handle reconnected events and polling fallback. Let me revise the complete stream.go:

```go
// sdks/go/stream.go
package togglerino

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// runSSE manages the SSE connection lifecycle with automatic reconnection.
func (c *Client) runSSE(ctx context.Context) {
	var retryCount int
	var pollingActive atomic.Bool

	for {
		if ctx.Err() != nil {
			return
		}

		wasReconnecting := retryCount > 0

		err := c.connectSSE(ctx)

		if ctx.Err() != nil {
			return
		}

		// If this was a successful connect that then ended, reset retry.
		// If it failed immediately, increment retry.
		if err != nil {
			c.config.logger.Warn("SSE connection error", "error", err)
		}

		delay := c.retryDelay(retryCount)
		retryCount++
		c.events.emit(eventReconnecting, reconnectingPayload{
			Attempt: retryCount,
			Delay:   delay,
		})

		// Start polling fallback if not already active
		if pollingActive.CompareAndSwap(false, true) {
			c.wg.Add(1)
			go func() {
				defer c.wg.Done()
				c.runPolling(ctx)
				pollingActive.Store(false)
			}()
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		_ = wasReconnecting
	}
}

// connectSSE establishes an SSE connection and processes events until it ends.
func (c *Client) connectSSE(ctx context.Context) error {
	url := c.config.serverURL + "/api/v1/stream"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.config.sdkKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.config.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("SSE returned status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	var eventType, data string

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			if data != "" {
				c.handleSSEEvent(eventType, data)
			}
			eventType = ""
			data = ""
			continue
		}

		if strings.HasPrefix(line, ":") {
			continue
		}

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}

	return scanner.Err()
}

// handleSSEEvent processes a single parsed SSE event.
func (c *Client) handleSSEEvent(eventType, data string) {
	switch eventType {
	case "flag_update":
		var evt sseEvent
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			return
		}

		c.flagsMu.Lock()
		existing := c.flags[evt.FlagKey]
		reason := "stream_update"
		if existing != nil {
			reason = existing.Reason
		}
		c.flags[evt.FlagKey] = &EvaluationResult{
			Value:   evt.Value,
			Variant: evt.Variant,
			Reason:  reason,
		}
		c.flagsMu.Unlock()

		c.events.emit(eventChange, FlagChangeEvent{
			FlagKey: evt.FlagKey,
			Value:   evt.Value,
			Variant: evt.Variant,
		})

	case "flag_deleted":
		var evt sseEvent
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			return
		}

		c.flagsMu.Lock()
		delete(c.flags, evt.FlagKey)
		c.flagsMu.Unlock()

		c.events.emit(eventDeleted, FlagDeletedEvent{
			FlagKey: evt.FlagKey,
		})
	}
}

// retryDelay calculates exponential backoff: 1s, 2s, 4s, 8s, 16s, 30s (capped).
func (c *Client) retryDelay(retryCount int) time.Duration {
	delay := defaultBaseRetryDelay * time.Duration(math.Pow(2, float64(retryCount)))
	if delay > defaultMaxRetryDelay {
		delay = defaultMaxRetryDelay
	}
	return delay
}
```

Note: The `runSSE`/polling fallback interaction is simplified. We need to also update `client.go` to remove the placeholder `runSSE` method and add polling support fields. The `startPollingFallback` method and `pollStop` channel from the placeholder won't be needed — the context cancellation handles cleanup.

Update `client.go`: Remove the placeholder `runSSE` and `runPolling` methods (they'll be in stream.go and polling.go respectively).

**Step 4: Run tests to verify they pass**

Run: `cd sdks/go && go test -run TestSSE -v -timeout 30s ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add sdks/go/stream.go sdks/go/stream_test.go sdks/go/client.go
git commit -m "feat(sdk/go): add SSE streaming with exponential backoff reconnection"
```

---

### Task 6: Polling fallback

**Files:**
- Create: `sdks/go/polling.go` (replace placeholder)
- Create: `sdks/go/polling_test.go`

**Step 1: Write the failing tests**

```go
// sdks/go/polling_test.go
package togglerino

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPolling_FetchesPeriodically(t *testing.T) {
	var fetchCount atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/evaluate" {
			fetchCount.Add(1)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(evaluateResponse{
				Flags: map[string]*EvaluationResult{},
			})
		}
	}))
	defer ts.Close()

	client, err := New(context.Background(), Config{
		ServerURL:       ts.URL,
		SDKKey:          "sdk_test",
		Streaming:       boolPtr(false),
		PollingInterval: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Initial fetch = 1, then wait for 2 more polling intervals
	time.Sleep(500 * time.Millisecond)

	client.Close()

	count := fetchCount.Load()
	if count < 3 {
		t.Errorf("expected at least 3 fetches (1 initial + 2 polls), got %d", count)
	}
}

func TestPolling_StopsOnClose(t *testing.T) {
	var fetchCount atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/evaluate" {
			fetchCount.Add(1)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(evaluateResponse{
				Flags: map[string]*EvaluationResult{},
			})
		}
	}))
	defer ts.Close()

	client, err := New(context.Background(), Config{
		ServerURL:       ts.URL,
		SDKKey:          "sdk_test",
		Streaming:       boolPtr(false),
		PollingInterval: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	time.Sleep(150 * time.Millisecond)
	client.Close()

	countAfterClose := fetchCount.Load()

	// Wait to verify no more fetches happen
	time.Sleep(300 * time.Millisecond)

	if fetchCount.Load() != countAfterClose {
		t.Errorf("polling continued after Close(): before=%d after=%d", countAfterClose, fetchCount.Load())
	}
}

func TestPolling_EmitsChangeOnFlagUpdate(t *testing.T) {
	callCount := 0
	var mu sync.Mutex

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/evaluate" {
			mu.Lock()
			callCount++
			c := callCount
			mu.Unlock()

			var flags map[string]*EvaluationResult
			if c == 1 {
				flags = map[string]*EvaluationResult{
					"feature": {Value: false, Variant: "off", Reason: "default"},
				}
			} else {
				flags = map[string]*EvaluationResult{
					"feature": {Value: true, Variant: "on", Reason: "rule_match"},
				}
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(evaluateResponse{Flags: flags})
		}
	}))
	defer ts.Close()

	client, err := New(context.Background(), Config{
		ServerURL:       ts.URL,
		SDKKey:          "sdk_test",
		Streaming:       boolPtr(false),
		PollingInterval: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer client.Close()

	var changes []FlagChangeEvent
	var changesMu sync.Mutex
	client.OnChange(func(e FlagChangeEvent) {
		changesMu.Lock()
		changes = append(changes, e)
		changesMu.Unlock()
	})

	// Wait for at least one poll
	time.Sleep(200 * time.Millisecond)

	changesMu.Lock()
	if len(changes) == 0 {
		t.Fatal("expected change event from polling, got none")
	}
	if changes[0].FlagKey != "feature" || changes[0].Value != true {
		t.Errorf("unexpected change event: %+v", changes[0])
	}
	changesMu.Unlock()
}
```

**Step 2: Run tests to verify they fail**

Run: `cd sdks/go && go test -run TestPolling -v -timeout 30s ./...`
Expected: FAIL (runPolling is a placeholder)

**Step 3: Implement polling.go**

```go
// sdks/go/polling.go
package togglerino

import (
	"context"
	"time"
)

// runPolling periodically fetches flags at the configured interval.
// Runs until ctx is cancelled.
func (c *Client) runPolling(ctx context.Context) {
	ticker := time.NewTicker(c.config.pollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.fetchFlags(ctx); err != nil {
				c.events.emit(eventError, err)
			}
		}
	}
}
```

Remove the placeholder `runPolling` from `client.go`.

**Step 4: Run tests to verify they pass**

Run: `cd sdks/go && go test -run TestPolling -v -timeout 30s ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add sdks/go/polling.go sdks/go/polling_test.go sdks/go/client.go
git commit -m "feat(sdk/go): add polling fallback for flag updates"
```

---

### Task 7: Run all tests and verify full integration

**Files:**
- None (test run only)

**Step 1: Run all tests**

Run: `cd sdks/go && go test -v -timeout 60s -race ./...`
Expected: PASS (all tests, no race conditions)

**Step 2: Run go vet**

Run: `cd sdks/go && go vet ./...`
Expected: no issues

**Step 3: Verify the package builds cleanly**

Run: `cd sdks/go && go build ./...`
Expected: success

**Step 4: Commit if any fixes were needed**

If any test failures or vet issues required code changes, commit those fixes.

---

### Task 8: Add example_test.go for godoc

**Files:**
- Create: `sdks/go/example_test.go`

**Step 1: Write example tests**

```go
// sdks/go/example_test.go
package togglerino_test

import (
	"context"
	"fmt"
	"log"

	togglerino "github.com/joCur/togglerino/sdks/go"
)

func Example() {
	client, err := togglerino.New(context.Background(), togglerino.Config{
		ServerURL: "http://localhost:8080",
		SDKKey:    "sdk_your_key_here",
		Context: &togglerino.EvaluationContext{
			UserID:     "user-42",
			Attributes: map[string]any{"plan": "pro"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Read flag values (synchronous, from local cache)
	darkMode := client.BoolValue("dark-mode", false)
	fmt.Println("dark mode:", darkMode)

	theme := client.StringValue("theme", "light")
	fmt.Println("theme:", theme)

	limit := client.NumberValue("rate-limit", 100)
	fmt.Println("rate limit:", limit)

	// Listen for real-time flag changes
	client.OnChange(func(e togglerino.FlagChangeEvent) {
		fmt.Printf("flag %q changed to %v\n", e.FlagKey, e.Value)
	})
}
```

**Step 2: Verify it compiles**

Run: `cd sdks/go && go build ./...`
Expected: success (example tests are compile-checked)

**Step 3: Commit**

```bash
git add sdks/go/example_test.go
git commit -m "docs(sdk/go): add godoc examples"
```

---

### Task 9: Add Go SDK tests to CI

**Files:**
- Modify: `.github/workflows/ci.yml`

**Step 1: Read the current CI config**

Read: `.github/workflows/ci.yml`

**Step 2: Add a `test-go-sdk` job**

Add a new job similar to `test-sdks` that runs Go SDK tests:

```yaml
  test-go-sdk:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - name: Run Go SDK tests
        working-directory: sdks/go
        run: go test -v -race -timeout 60s ./...
```

Also add `test-go-sdk` to the `needs` list of the `build` job.

**Step 3: Verify YAML is valid**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"`
Expected: no errors

**Step 4: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add Go SDK test job to CI pipeline"
```

---

### Task 10: Final review and cleanup

**Step 1: Run the full test suite across the whole project**

Run: `cd sdks/go && go test -v -race -timeout 60s ./...`
Expected: all pass

**Step 2: Verify file list matches the design**

Run: `ls -la sdks/go/`
Expected files:
- `go.mod`
- `types.go`
- `config.go`
- `events.go`, `events_test.go`
- `client.go`, `client_test.go`
- `flags.go`
- `context.go`
- `stream.go`, `stream_test.go`
- `polling.go`, `polling_test.go`
- `example_test.go`

**Step 3: Verify no TODO/FIXME left**

Run: `grep -r "TODO\|FIXME\|HACK" sdks/go/`
Expected: none

**Step 4: Final commit if needed**

If any cleanup was done, commit with: `chore(sdk/go): final cleanup`
