package metrics

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Registry holds all Prometheus metrics for Togglerino.
type Registry struct {
	Prometheus *prometheus.Registry

	EvaluationsTotal           *prometheus.CounterVec
	EvaluationDurationSeconds  prometheus.Histogram
	SSEConnectionsActive       *prometheus.GaugeVec
	CacheFlagsTotal            prometheus.Gauge
	HTTPRequestsTotal          *prometheus.CounterVec
	HTTPRequestDurationSeconds *prometheus.HistogramVec
	DBPoolActive               prometheus.Gauge
	DBPoolIdle                 prometheus.Gauge

	sseKeys []string // tracks previous SSE label keys for stale cleanup
}

func NewRegistry() *Registry {
	reg := prometheus.NewRegistry()

	r := &Registry{
		Prometheus: reg,
		EvaluationsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "togglerino_evaluations_total",
			Help: "Total number of flag evaluations.",
		}, []string{"project", "environment", "flag", "variant"}),
		EvaluationDurationSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "togglerino_evaluation_duration_seconds",
			Help:    "Duration of flag evaluation requests.",
			Buckets: []float64{0.0001, 0.00025, 0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		}),
		SSEConnectionsActive: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "togglerino_sse_connections_active",
			Help: "Number of active SSE connections.",
		}, []string{"project", "environment"}),
		CacheFlagsTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "togglerino_cache_flags_total",
			Help: "Total number of flags in the in-memory cache.",
		}),
		HTTPRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "togglerino_http_requests_total",
			Help: "Total number of HTTP requests.",
		}, []string{"method", "path", "status"}),
		HTTPRequestDurationSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "togglerino_http_request_duration_seconds",
			Help:    "Duration of HTTP requests.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),
		DBPoolActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "togglerino_db_pool_active_connections",
			Help: "Number of active database connections.",
		}),
		DBPoolIdle: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "togglerino_db_pool_idle_connections",
			Help: "Number of idle database connections.",
		}),
	}

	reg.MustRegister(
		r.EvaluationsTotal, r.EvaluationDurationSeconds,
		r.SSEConnectionsActive, r.CacheFlagsTotal,
		r.HTTPRequestsTotal, r.HTTPRequestDurationSeconds,
		r.DBPoolActive, r.DBPoolIdle,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	return r
}

func (r *Registry) RecordEvaluation(project, environment, flag, variant string) {
	r.EvaluationsTotal.WithLabelValues(project, environment, flag, variant).Inc()
}

func (r *Registry) ObserveEvaluationDuration(seconds float64) {
	r.EvaluationDurationSeconds.Observe(seconds)
}

func (r *Registry) SetCacheFlagCount(count int) {
	r.CacheFlagsTotal.Set(float64(count))
}

func (r *Registry) SetDBPoolStats(active, idle int) {
	r.DBPoolActive.Set(float64(active))
	r.DBPoolIdle.Set(float64(idle))
}

func (r *Registry) SetSSEConnections(counts map[string]int) {
	// Set current values and track which keys are present.
	seen := make(map[string]struct{}, len(counts))
	for key, count := range counts {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) == 2 {
			r.SSEConnectionsActive.WithLabelValues(parts[0], parts[1]).Set(float64(count))
			seen[key] = struct{}{}
		}
	}
	// Remove stale keys that are no longer present.
	for _, key := range r.sseKeys {
		if _, ok := seen[key]; !ok {
			parts := strings.SplitN(key, ":", 2)
			if len(parts) == 2 {
				r.SSEConnectionsActive.DeleteLabelValues(parts[0], parts[1])
			}
		}
	}
	// Update tracked keys.
	r.sseKeys = make([]string, 0, len(seen))
	for key := range seen {
		r.sseKeys = append(r.sseKeys, key)
	}
}
