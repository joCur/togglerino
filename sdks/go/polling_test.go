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
			json.NewEncoder(w).Encode(evaluateResponse{Flags: map[string]*EvaluationResult{}})
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
			json.NewEncoder(w).Encode(evaluateResponse{Flags: map[string]*EvaluationResult{}})
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
