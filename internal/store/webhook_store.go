package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type WebhookStore struct {
	pool *pgxpool.Pool
}

func NewWebhookStore(pool *pgxpool.Pool) *WebhookStore {
	return &WebhookStore{pool: pool}
}

func (s *WebhookStore) Create(ctx context.Context, projectID, name, url, secret string, eventTypes []string) (*model.Webhook, error) {
	var wh model.Webhook
	err := s.pool.QueryRow(ctx,
		`INSERT INTO webhooks (project_id, name, url, secret, event_types)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, project_id, name, url, secret, event_types, enabled, created_at, updated_at`,
		projectID, name, url, secret, eventTypes,
	).Scan(&wh.ID, &wh.ProjectID, &wh.Name, &wh.URL, &wh.Secret, &wh.EventTypes, &wh.Enabled, &wh.CreatedAt, &wh.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating webhook: %w", err)
	}
	return &wh, nil
}

func (s *WebhookStore) GetByID(ctx context.Context, id string) (*model.Webhook, error) {
	var wh model.Webhook
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, name, url, secret, event_types, enabled, created_at, updated_at
		 FROM webhooks WHERE id = $1`,
		id,
	).Scan(&wh.ID, &wh.ProjectID, &wh.Name, &wh.URL, &wh.Secret, &wh.EventTypes, &wh.Enabled, &wh.CreatedAt, &wh.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("finding webhook by id: %w", err)
	}
	return &wh, nil
}

func (s *WebhookStore) ListByProject(ctx context.Context, projectID string, limit, offset int) ([]model.Webhook, int, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, name, url, secret, event_types, enabled, created_at, updated_at, COUNT(*) OVER() AS total_count
		 FROM webhooks WHERE project_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		projectID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("listing webhooks: %w", err)
	}
	defer rows.Close()

	var webhooks []model.Webhook
	totalCount := 0
	for rows.Next() {
		var wh model.Webhook
		if err := rows.Scan(&wh.ID, &wh.ProjectID, &wh.Name, &wh.URL, &wh.Secret, &wh.EventTypes, &wh.Enabled, &wh.CreatedAt, &wh.UpdatedAt, &totalCount); err != nil {
			return nil, 0, fmt.Errorf("scanning webhook: %w", err)
		}
		webhooks = append(webhooks, wh)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating webhooks: %w", err)
	}
	return webhooks, totalCount, nil
}

func (s *WebhookStore) ListEnabledByProject(ctx context.Context, projectID string) ([]model.Webhook, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, name, url, secret, event_types, enabled, created_at, updated_at
		 FROM webhooks WHERE project_id = $1 AND enabled = true`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing enabled webhooks: %w", err)
	}
	defer rows.Close()

	var webhooks []model.Webhook
	for rows.Next() {
		var wh model.Webhook
		if err := rows.Scan(&wh.ID, &wh.ProjectID, &wh.Name, &wh.URL, &wh.Secret, &wh.EventTypes, &wh.Enabled, &wh.CreatedAt, &wh.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning webhook: %w", err)
		}
		webhooks = append(webhooks, wh)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating webhooks: %w", err)
	}
	return webhooks, nil
}

func (s *WebhookStore) Update(ctx context.Context, id, name, url string, eventTypes []string, enabled bool) (*model.Webhook, error) {
	var wh model.Webhook
	err := s.pool.QueryRow(ctx,
		`UPDATE webhooks SET name=$2, url=$3, event_types=$4, enabled=$5, updated_at=NOW() WHERE id=$1
		 RETURNING id, project_id, name, url, secret, event_types, enabled, created_at, updated_at`,
		id, name, url, eventTypes, enabled,
	).Scan(&wh.ID, &wh.ProjectID, &wh.Name, &wh.URL, &wh.Secret, &wh.EventTypes, &wh.Enabled, &wh.CreatedAt, &wh.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating webhook: %w", err)
	}
	return &wh, nil
}

func (s *WebhookStore) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM webhooks WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting webhook: %w", err)
	}
	return nil
}
