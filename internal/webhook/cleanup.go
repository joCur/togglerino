package webhook

import (
	"context"
	"log/slog"
	"time"

	"github.com/togglerino/togglerino/internal/store"
)

func StartCleanup(ctx context.Context, deliveries *store.WebhookDeliveryStore) {
	cleanup := func() {
		deleted, err := deliveries.DeleteOlderThan(ctx, 30)
		if err != nil {
			slog.Warn("failed to clean up old webhook deliveries", "error", err)
		} else if deleted > 0 {
			slog.Info("cleaned up old webhook deliveries", "deleted", deleted)
		}
	}

	go cleanup()

	ticker := time.NewTicker(6 * time.Hour)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cleanup()
			}
		}
	}()
}
