package togglerino

import (
	"context"
	"time"
)

func (c *Client) runPolling(ctx context.Context) {
	ticker := time.NewTicker(c.config.pollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.fetchFlags(ctx); err != nil {
				c.events.emit(eventError, err)
			}
		}
	}
}
