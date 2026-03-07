package override

import (
	"context"
	"log/slog"
	"time"
)

// ExpiredDeleter is the interface the cleaner needs for deleting expired overrides.
type ExpiredDeleter interface {
	DeleteExpired(ctx context.Context) (int64, error)
}

type Cleaner struct {
	overrides ExpiredDeleter
	interval  time.Duration
}

func NewCleaner(overrides ExpiredDeleter, interval time.Duration) *Cleaner {
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
