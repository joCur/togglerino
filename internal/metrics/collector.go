package metrics

import (
	"context"
	"time"
)

type StatsSource interface {
	FlagCount() int
	AllSubscriberCounts() map[string]int
	ActiveConns() int32
	IdleConns() int32
}

func (r *Registry) CollectOnce(src StatsSource) {
	r.SetCacheFlagCount(src.FlagCount())
	r.SetSSEConnections(src.AllSubscriberCounts())
	r.SetDBPoolStats(int(src.ActiveConns()), int(src.IdleConns()))
}

func (r *Registry) RunCollector(ctx context.Context, src StatsSource, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	r.CollectOnce(src)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.CollectOnce(src)
		}
	}
}
