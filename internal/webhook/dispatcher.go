package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/togglerino/togglerino/internal/store"
)

type Dispatcher struct {
	webhooks   *store.WebhookStore
	deliveries *store.WebhookDeliveryStore
}

func NewDispatcher(webhooks *store.WebhookStore, deliveries *store.WebhookDeliveryStore) *Dispatcher {
	return &Dispatcher{webhooks: webhooks, deliveries: deliveries}
}

func (disp *Dispatcher) Dispatch(ctx context.Context, projectID string, event Event) {
	hooks, err := disp.webhooks.ListEnabledByProject(ctx, projectID)
	if err != nil {
		slog.Warn("failed to list webhooks for dispatch", "project_id", projectID, "error", err)
		return
	}

	payload, err := json.Marshal(event)
	if err != nil {
		slog.Warn("failed to marshal webhook event", "error", err)
		return
	}

	for _, hook := range hooks {
		if !matchesEventType(hook.EventTypes, event.Type) {
			continue
		}
		h := hook
		p := payload
		go disp.deliverWithRetry(h.ID, h.URL, h.Secret, event.Type, p)
	}
}

func (disp *Dispatcher) deliverWithRetry(webhookID, url, secret, eventType string, payload []byte) {
	backoffs := []time.Duration{0, 5 * time.Second, 25 * time.Second}
	for attempt := 1; attempt <= 3; attempt++ {
		if attempt > 1 {
			time.Sleep(backoffs[attempt-1])
		}

		ctx := context.Background()
		deliveryID := fmt.Sprintf("%s-%d-%d", webhookID, attempt, time.Now().UnixNano())
		result := Deliver(url, secret, eventType, deliveryID, payload)

		if _, err := disp.deliveries.Record(ctx, webhookID, eventType, payload, result.StatusCode, result.ResponseBody, result.Error, attempt, result.Success, &result.DurationMs); err != nil {
			slog.Warn("failed to record webhook delivery", "webhook_id", webhookID, "attempt", attempt, "error", err)
		}

		if result.Success {
			return
		}
		slog.Warn("webhook delivery failed", "webhook_id", webhookID, "attempt", attempt, "url", url, "error", result.Error)
	}
}

func (disp *Dispatcher) RetryFailed(ctx context.Context) {
	deliveries, err := disp.deliveries.ListFailedRecent(ctx)
	if err != nil {
		slog.Warn("failed to list failed deliveries for retry", "error", err)
		return
	}
	if len(deliveries) == 0 {
		return
	}
	slog.Info("retrying failed webhook deliveries", "count", len(deliveries))
	for _, del := range deliveries {
		wh, err := disp.webhooks.GetByID(ctx, del.WebhookID)
		if err != nil {
			slog.Warn("failed to get webhook for retry", "webhook_id", del.WebhookID, "error", err)
			continue
		}
		d := del
		go func() {
			nextAttempt := d.Attempt + 1
			deliveryID := fmt.Sprintf("%s-retry-%d-%d", d.WebhookID, nextAttempt, time.Now().UnixNano())
			result := Deliver(wh.URL, wh.Secret, d.EventType, deliveryID, d.Payload)
			bgCtx := context.Background()
			if _, err := disp.deliveries.Record(bgCtx, d.WebhookID, d.EventType, d.Payload, result.StatusCode, result.ResponseBody, result.Error, nextAttempt, result.Success, &result.DurationMs); err != nil {
				slog.Warn("failed to record retry delivery", "error", err)
			}
		}()
	}
}

func matchesEventType(hookTypes []string, eventType string) bool {
	for _, t := range hookTypes {
		if t == eventType {
			return true
		}
	}
	return false
}
