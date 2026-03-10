package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func collectGauge(g prometheus.Gauge) float64 {
	m := &dto.Metric{}
	g.Write(m)
	return m.GetGauge().GetValue()
}

func collectCounter(c prometheus.Counter) float64 {
	m := &dto.Metric{}
	c.Write(m)
	return m.GetCounter().GetValue()
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("expected non-nil registry")
	}
	if r.Prometheus == nil {
		t.Fatal("expected non-nil prometheus registry")
	}
}

func TestRecordEvaluation(t *testing.T) {
	r := NewRegistry()
	r.RecordEvaluation("myproj", "dev", "my-flag", "on")
	r.RecordEvaluation("myproj", "dev", "my-flag", "on")

	counter, err := r.EvaluationsTotal.GetMetricWithLabelValues("myproj", "dev", "my-flag", "on")
	if err != nil {
		t.Fatal(err)
	}
	if got := collectCounter(counter); got != 2 {
		t.Errorf("expected 2 evaluations, got %f", got)
	}
}

func TestObserveEvaluationDuration(t *testing.T) {
	r := NewRegistry()
	r.ObserveEvaluationDuration(0.005)
	// Just verify it doesn't panic
}

func TestSetCacheFlagCount(t *testing.T) {
	r := NewRegistry()
	r.SetCacheFlagCount(42)
	if got := collectGauge(r.CacheFlagsTotal); got != 42 {
		t.Errorf("expected 42, got %f", got)
	}
}

func TestSetDBPoolStats(t *testing.T) {
	r := NewRegistry()
	r.SetDBPoolStats(5, 3)
	if got := collectGauge(r.DBPoolActive); got != 5 {
		t.Errorf("expected active=5, got %f", got)
	}
	if got := collectGauge(r.DBPoolIdle); got != 3 {
		t.Errorf("expected idle=3, got %f", got)
	}
}

func TestSetSSEConnections(t *testing.T) {
	r := NewRegistry()
	r.SetSSEConnections(map[string]int{
		"proj1:dev":  3,
		"proj2:prod": 1,
	})
	gauge, err := r.SSEConnectionsActive.GetMetricWithLabelValues("proj1", "dev")
	if err != nil {
		t.Fatal(err)
	}
	if got := collectGauge(gauge); got != 3 {
		t.Errorf("expected 3, got %f", got)
	}
}
