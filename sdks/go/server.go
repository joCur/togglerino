package togglerino

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ServerConfig holds configuration for a server-side evaluation client.
type ServerConfig struct {
	// ServerURL is the base URL of the Togglerino server (e.g. "https://flags.example.com").
	ServerURL string

	// SDKKey is the SDK key used to authenticate with the server.
	SDKKey string

	// Streaming controls whether SSE streaming is used for real-time updates.
	// Defaults to true. Set to false to use polling instead.
	Streaming *bool

	// PollingInterval is the interval between polling requests when streaming
	// is disabled. Defaults to 30 seconds.
	PollingInterval time.Duration

	// HTTPClient is the HTTP client used for requests. Defaults to http.DefaultClient.
	HTTPClient *http.Client

	// Logger is the structured logger. Defaults to slog.Default().
	Logger *slog.Logger
}

// resolvedServerConfig holds the resolved (defaulted) server configuration.
type resolvedServerConfig struct {
	serverURL       string
	sdkKey          string
	streaming       bool
	pollingInterval time.Duration
	httpClient      *http.Client
	logger          *slog.Logger
}

func resolveServerConfig(c ServerConfig) resolvedServerConfig {
	rc := resolvedServerConfig{
		serverURL:       strings.TrimRight(c.ServerURL, "/"),
		sdkKey:          c.SDKKey,
		streaming:       true,
		pollingInterval: defaultPollingInterval,
		httpClient:      http.DefaultClient,
		logger:          slog.Default(),
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

// Server is a server-side evaluation client that caches flag definitions
// locally and evaluates them per-request with the provided EvaluationContext.
// Unlike Client, Server does not send user context to the server — evaluation
// happens entirely in-process.
type Server struct {
	config     resolvedServerConfig
	flags      []FlagDefinition
	segments   []SegmentDefinition
	mu         sync.RWMutex
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
	closeOnce  sync.Once
}

// NewServer creates a new server-side evaluation client. It fetches the initial
// flag definitions from the server and starts background synchronization (SSE
// or polling). The provided ctx is used only for the initial fetch; a separate
// background context governs the sync goroutine's lifetime.
func NewServer(ctx context.Context, cfg ServerConfig) (*Server, error) {
	rc := resolveServerConfig(cfg)
	bgCtx, cancel := context.WithCancel(context.Background())

	s := &Server{
		config:     rc,
		cancelFunc: cancel,
	}

	if err := s.fetchDefinitions(ctx); err != nil {
		cancel()
		return nil, err
	}

	if rc.streaming {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.runSSE(bgCtx)
		}()
	} else {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.runPolling(bgCtx)
		}()
	}

	return s, nil
}

// Close shuts down background goroutines and waits for them to finish.
// It is safe to call multiple times.
func (s *Server) Close() {
	s.closeOnce.Do(func() {
		s.cancelFunc()
		s.wg.Wait()
	})
}

// Evaluate evaluates all cached flag definitions against the provided context
// and returns an Evaluator holding the results. This method is safe to call
// concurrently from multiple goroutines.
func (s *Server) Evaluate(ctx EvaluationContext) *Evaluator {
	if ctx.Attributes == nil {
		ctx.Attributes = make(map[string]any)
	}

	s.mu.RLock()
	flags := s.flags
	segments := s.segments
	s.mu.RUnlock()

	results := make(map[string]EvaluationResult, len(flags))
	for _, flag := range flags {
		results[flag.Key] = evaluateFlag(flag, ctx, segments)
	}

	return &Evaluator{results: results}
}

// fetchDefinitions performs a GET /api/v1/definitions request to refresh
// the local definition cache.
func (s *Server) fetchDefinitions(ctx context.Context) error {
	url := s.config.serverURL + "/api/v1/definitions"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("togglerino: failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.config.sdkKey)

	resp, err := s.config.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("togglerino: definitions fetch failed with status %d", resp.StatusCode)
	}

	var defs definitionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&defs); err != nil {
		return fmt.Errorf("togglerino: failed to decode definitions response: %w", err)
	}

	s.mu.Lock()
	s.flags = defs.Flags
	s.segments = defs.Segments
	s.mu.Unlock()

	return nil
}

// runSSE connects to the SSE stream and re-fetches definitions on flag events.
// On connection failure, it retries with exponential backoff.
func (s *Server) runSSE(ctx context.Context) {
	var retryCount int

	for {
		if ctx.Err() != nil {
			return
		}

		err := s.connectSSE(ctx, func() {
			retryCount = 0
		})

		if ctx.Err() != nil {
			return
		}

		if err != nil {
			s.config.logger.Warn("SSE connection error", "error", err)
		}

		delay := s.retryDelay(retryCount)
		retryCount++

		// Poll once during reconnection to stay up-to-date.
		if err := s.fetchDefinitions(ctx); err != nil && ctx.Err() == nil {
			s.config.logger.Warn("failed to poll definitions during SSE reconnect", "error", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

// connectSSE opens an SSE connection to the server. The onConnected callback
// is invoked once when a 200 OK response is received, before reading events.
func (s *Server) connectSSE(ctx context.Context, onConnected func()) error {
	url := s.config.serverURL + "/api/v1/stream"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.config.sdkKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := s.config.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("SSE returned status %d", resp.StatusCode)
	}

	if onConnected != nil {
		onConnected()
	}

	scanner := bufio.NewScanner(resp.Body)
	var eventType, data string

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			if data != "" {
				s.handleSSEEvent(ctx, eventType, data)
			}
			eventType = ""
			data = ""
			continue
		}

		if strings.HasPrefix(line, ":") {
			continue // comment/keepalive
		}

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			line := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" {
				data = line
			} else {
				data = data + "\n" + line
			}
		}
	}

	return scanner.Err()
}

// handleSSEEvent handles a parsed SSE event. For the server-side client,
// any flag_update or flag_deleted event triggers a full re-fetch of all
// definitions rather than fetching a single flag.
func (s *Server) handleSSEEvent(ctx context.Context, eventType, _ string) {
	switch eventType {
	case "flag_update", "flag_deleted":
		if err := s.fetchDefinitions(ctx); err != nil {
			s.config.logger.Warn("failed to re-fetch definitions after SSE event", "event", eventType, "error", err)
		}
	}
}

// runPolling periodically re-fetches flag definitions.
func (s *Server) runPolling(ctx context.Context) {
	ticker := time.NewTicker(s.config.pollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.fetchDefinitions(ctx); err != nil {
				s.config.logger.Warn("failed to poll definitions", "error", err)
			}
		}
	}
}

// retryDelay calculates the backoff delay for SSE reconnection attempts.
func (s *Server) retryDelay(retryCount int) time.Duration {
	delay := defaultBaseRetryDelay * time.Duration(math.Pow(2, float64(retryCount)))
	if delay > defaultMaxRetryDelay {
		delay = defaultMaxRetryDelay
	}
	return delay
}
