package metrics

import (
	"testing"
)

type mockStatsSource struct {
	flagCount int
	sseCounts map[string]int
	dbActive  int32
	dbIdle    int32
}

func (m *mockStatsSource) FlagCount() int                      { return m.flagCount }
func (m *mockStatsSource) AllSubscriberCounts() map[string]int  { return m.sseCounts }
func (m *mockStatsSource) ActiveConns() int32                   { return m.dbActive }
func (m *mockStatsSource) IdleConns() int32                     { return m.dbIdle }

func TestCollectOnce(t *testing.T) {
	r := NewRegistry()
	src := &mockStatsSource{
		flagCount: 25,
		sseCounts: map[string]int{"proj1:dev": 3},
		dbActive:  5,
		dbIdle:    10,
	}

	r.CollectOnce(src)

	if got := collectGauge(r.CacheFlagsTotal); got != 25 {
		t.Errorf("expected cache=25, got %f", got)
	}
	if got := collectGauge(r.DBPoolActive); got != 5 {
		t.Errorf("expected dbActive=5, got %f", got)
	}
	if got := collectGauge(r.DBPoolIdle); got != 10 {
		t.Errorf("expected dbIdle=10, got %f", got)
	}
	gauge, _ := r.SSEConnectionsActive.GetMetricWithLabelValues("proj1", "dev")
	if got := collectGauge(gauge); got != 3 {
		t.Errorf("expected sse=3, got %f", got)
	}
}
