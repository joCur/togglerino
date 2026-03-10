package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type WebhookDeliveryStore struct {
	pool *pgxpool.Pool
}

func NewWebhookDeliveryStore(pool *pgxpool.Pool) *WebhookDeliveryStore {
	return &WebhookDeliveryStore{pool: pool}
}

func (s *WebhookDeliveryStore) Record(ctx context.Context, webhookID, eventType string, payload json.RawMessage, statusCode *int, responseBody, errMsg *string, attempt int, success bool, durationMs *int) (*model.WebhookDelivery, error) {
	var d model.WebhookDelivery
	err := s.pool.QueryRow(ctx,
		`INSERT INTO webhook_deliveries (webhook_id, event_type, payload, status_code, response_body, error, attempt, success, duration_ms)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, webhook_id, event_type, payload, status_code, response_body, error, attempt, success, duration_ms, created_at`,
		webhookID, eventType, payload, statusCode, responseBody, errMsg, attempt, success, durationMs,
	).Scan(&d.ID, &d.WebhookID, &d.EventType, &d.Payload, &d.StatusCode, &d.ResponseBody, &d.Error, &d.Attempt, &d.Success, &d.DurationMs, &d.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("recording webhook delivery: %w", err)
	}
	return &d, nil
}

func (s *WebhookDeliveryStore) ListByWebhook(ctx context.Context, webhookID string, limit, offset int) ([]model.WebhookDelivery, int, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, webhook_id, event_type, payload, status_code, response_body, error, attempt, success, duration_ms, created_at, COUNT(*) OVER() AS total_count
		 FROM webhook_deliveries WHERE webhook_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		webhookID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("listing webhook deliveries: %w", err)
	}
	defer rows.Close()

	var deliveries []model.WebhookDelivery
	totalCount := 0
	for rows.Next() {
		var d model.WebhookDelivery
		if err := rows.Scan(&d.ID, &d.WebhookID, &d.EventType, &d.Payload, &d.StatusCode, &d.ResponseBody, &d.Error, &d.Attempt, &d.Success, &d.DurationMs, &d.CreatedAt, &totalCount); err != nil {
			return nil, 0, fmt.Errorf("scanning webhook delivery: %w", err)
		}
		deliveries = append(deliveries, d)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating webhook deliveries: %w", err)
	}
	return deliveries, totalCount, nil
}

func (s *WebhookDeliveryStore) ListFailedRecent(ctx context.Context) ([]model.WebhookDelivery, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT d.id, d.webhook_id, d.event_type, d.payload, d.status_code, d.response_body, d.error, d.attempt, d.success, d.duration_ms, d.created_at
		 FROM webhook_deliveries d
		 JOIN webhooks w ON w.id = d.webhook_id AND w.enabled = true
		 WHERE d.success = false AND d.attempt < 3 AND d.created_at > NOW() - INTERVAL '1 hour'
		 ORDER BY d.created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing failed recent deliveries: %w", err)
	}
	defer rows.Close()

	var deliveries []model.WebhookDelivery
	for rows.Next() {
		var d model.WebhookDelivery
		if err := rows.Scan(&d.ID, &d.WebhookID, &d.EventType, &d.Payload, &d.StatusCode, &d.ResponseBody, &d.Error, &d.Attempt, &d.Success, &d.DurationMs, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning webhook delivery: %w", err)
		}
		deliveries = append(deliveries, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating webhook deliveries: %w", err)
	}
	return deliveries, nil
}

func (s *WebhookDeliveryStore) DeleteOlderThan(ctx context.Context, days int) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM webhook_deliveries WHERE created_at < NOW() - make_interval(days => $1)`,
		days,
	)
	if err != nil {
		return 0, fmt.Errorf("deleting old webhook deliveries: %w", err)
	}
	return tag.RowsAffected(), nil
}
