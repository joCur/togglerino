package override

import (
	"context"
	"log/slog"
	"time"

	"github.com/togglerino/togglerino/internal/store"
)

type Cleaner struct {
	overrides *store.OverrideStore
	interval  time.Duration
}

func NewCleaner(overrides *store.OverrideStore, interval time.Duration) *Cleaner {
	return &Cleaner{overrides: overrides, interval: interval}
}

func (c *Cleaner) Run(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := c.overrides.DeleteExpired(ctx)
			if err != nil {
				slog.Error("cleaning expired overrides", "error", err)
				continue
			}
			if count > 0 {
				slog.Info("cleaned expired overrides", "count", count)
			}
		}
	}
}
