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
	ErrClosed = errors.New("togglerino: client is closed")
)

type Config struct {
	ServerURL       string
	SDKKey          string
	Context         *EvaluationContext
	Streaming       *bool
	PollingInterval time.Duration
	HTTPClient      *http.Client
	Logger          *slog.Logger
}

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
