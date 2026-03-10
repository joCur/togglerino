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
			Buckets: prometheus.DefBuckets,
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
	r.SSEConnectionsActive.Reset()
	for key, count := range counts {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) == 2 {
			r.SSEConnectionsActive.WithLabelValues(parts[0], parts[1]).Set(float64(count))
		}
	}
}
