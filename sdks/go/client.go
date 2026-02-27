package togglerino

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Client is the main entry point for the Togglerino Go SDK.
// It fetches flag evaluations from the server and keeps them in sync
// via SSE streaming or polling.
type Client struct {
	config      resolvedConfig
	events      *eventEmitter
	flags       map[string]*EvaluationResult
	flagsMu     sync.RWMutex
	initialized bool
	cancelFunc  context.CancelFunc
	wg          sync.WaitGroup
	closeOnce   sync.Once
}

// New creates a new Client, fetches the initial flag state, and starts
// background synchronization (SSE or polling). The provided ctx is used
// only for the initial fetch; a separate background context governs the
// sync goroutine's lifetime.
func New(ctx context.Context, cfg Config) (*Client, error) {
	rc := resolveConfig(cfg)
	bgCtx, cancel := context.WithCancel(context.Background())

	c := &Client{
		config:     rc,
		events:     newEventEmitter(),
		flags:      make(map[string]*EvaluationResult),
		cancelFunc: cancel,
	}

	if err := c.fetchFlags(ctx); err != nil {
		cancel()
		return nil, err
	}

	c.initialized = true

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

// Close shuts down background goroutines, waits for them to finish,
// and clears all event listeners. It is safe to call multiple times.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		c.cancelFunc()
		c.wg.Wait()
		c.events.clear()
	})
}

// fetchFlags performs a POST /api/v1/evaluate request to refresh the
// local flag cache. After initialization, it emits change events for
// any flags whose values differ from the previous fetch.
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

// jsonEqual compares two values by their JSON representations.
func jsonEqual(a, b any) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}

// Stub — replaced in Task 6
func (c *Client) runPolling(ctx context.Context) { <-ctx.Done() }

// Typed On* callback methods

// OnChange registers a callback invoked when a flag's value changes.
// Returns an unsubscribe function.
func (c *Client) OnChange(fn func(FlagChangeEvent)) func() {
	return c.events.on(eventChange, func(payload any) {
		if e, ok := payload.(FlagChangeEvent); ok {
			fn(e)
		}
	})
}

// OnDeleted registers a callback invoked when a flag is deleted.
// Returns an unsubscribe function.
func (c *Client) OnDeleted(fn func(FlagDeletedEvent)) func() {
	return c.events.on(eventDeleted, func(payload any) {
		if e, ok := payload.(FlagDeletedEvent); ok {
			fn(e)
		}
	})
}

// OnError registers a callback invoked when an error occurs.
// Returns an unsubscribe function.
func (c *Client) OnError(fn func(error)) func() {
	return c.events.on(eventError, func(payload any) {
		if e, ok := payload.(error); ok {
			fn(e)
		}
	})
}

// OnReady registers a callback invoked when the client is ready.
// Returns an unsubscribe function.
func (c *Client) OnReady(fn func()) func() {
	return c.events.on(eventReady, func(any) { fn() })
}

// OnReconnecting registers a callback invoked when the client is
// attempting to reconnect. Returns an unsubscribe function.
func (c *Client) OnReconnecting(fn func(attempt int, delay time.Duration)) func() {
	return c.events.on(eventReconnecting, func(payload any) {
		if e, ok := payload.(reconnectingPayload); ok {
			fn(e.Attempt, e.Delay)
		}
	})
}

// OnReconnected registers a callback invoked when the client
// successfully reconnects. Returns an unsubscribe function.
func (c *Client) OnReconnected(fn func()) func() {
	return c.events.on(eventReconnected, func(any) { fn() })
}

// OnContextChange registers a callback invoked when the evaluation
// context is updated. Returns an unsubscribe function.
func (c *Client) OnContextChange(fn func(EvaluationContext)) func() {
	return c.events.on(eventContextChange, func(payload any) {
		if e, ok := payload.(EvaluationContext); ok {
			fn(e)
		}
	})
}
