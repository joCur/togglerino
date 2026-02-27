package togglerino

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
)

func (c *Client) runSSE(ctx context.Context) {
	var retryCount int

	for {
		if ctx.Err() != nil {
			return
		}

		wasReconnecting := retryCount > 0
		err := c.connectSSE(ctx, func() {
			// Called when SSE connection is successfully established (200 OK).
			if wasReconnecting {
				c.events.emit(eventReconnected, nil)
			}
			retryCount = 0
		})

		if ctx.Err() != nil {
			return
		}

		// If err is nil, stream ended normally (server closed) — still need to reconnect.
		// If err is non-nil, connection failed.
		if err != nil {
			c.config.logger.Warn("SSE connection error", "error", err)
		}

		delay := c.retryDelay(retryCount)
		retryCount++
		c.events.emit(eventReconnecting, reconnectingPayload{
			Attempt: retryCount,
			Delay:   delay,
		})

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

// connectSSE opens an SSE connection to the server. The onConnected callback
// is invoked once when a 200 OK response is received, before reading events.
func (c *Client) connectSSE(ctx context.Context, onConnected func()) error {
	url := c.config.serverURL + "/api/v1/stream"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.config.sdkKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.config.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("SSE returned status %d", resp.StatusCode)
	}

	// SSE connected successfully — notify caller before reading events.
	if onConnected != nil {
		onConnected()
	}

	scanner := bufio.NewScanner(resp.Body)
	var eventType, data string

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			if data != "" {
				c.handleSSEEvent(eventType, data)
			}
			eventType = ""
			data = ""
			continue
		}

		if strings.HasPrefix(line, ":") {
			continue // comment/keepalive
		}

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			// Per SSE spec, multiple data: lines are concatenated with \n
			line := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" {
				data = line
			} else {
				data = data + "\n" + line
			}
		}
	}

	return scanner.Err()
}

func (c *Client) handleSSEEvent(eventType, data string) {
	switch eventType {
	case "flag_update":
		var evt sseEvent
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			return
		}

		c.flagsMu.Lock()
		existing := c.flags[evt.FlagKey]
		reason := "stream_update"
		if existing != nil {
			reason = existing.Reason
		}
		c.flags[evt.FlagKey] = &EvaluationResult{
			Value:   evt.Value,
			Variant: evt.Variant,
			Reason:  reason,
		}
		c.flagsMu.Unlock()

		c.events.emit(eventChange, FlagChangeEvent{
			FlagKey: evt.FlagKey,
			Value:   evt.Value,
			Variant: evt.Variant,
		})

	case "flag_deleted":
		var evt sseEvent
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			return
		}

		c.flagsMu.Lock()
		delete(c.flags, evt.FlagKey)
		c.flagsMu.Unlock()

		c.events.emit(eventDeleted, FlagDeletedEvent{
			FlagKey: evt.FlagKey,
		})
	}
}

func (c *Client) retryDelay(retryCount int) time.Duration {
	delay := defaultBaseRetryDelay * time.Duration(math.Pow(2, float64(retryCount)))
	if delay > defaultMaxRetryDelay {
		delay = defaultMaxRetryDelay
	}
	return delay
}
