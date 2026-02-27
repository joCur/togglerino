package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type ContextAttributeStore struct {
	pool *pgxpool.Pool
}

func NewContextAttributeStore(pool *pgxpool.Pool) *ContextAttributeStore {
	return &ContextAttributeStore{pool: pool}
}

// UpsertByProjectKey inserts or updates context attributes for a project identified by key.
// Uses a single query that resolves the project key to ID and unnests the attribute names.
func (s *ContextAttributeStore) UpsertByProjectKey(ctx context.Context, projectKey string, names []string) error {
	if len(names) == 0 {
		return nil
	}

	_, err := s.pool.Exec(ctx,
		`INSERT INTO context_attributes (project_id, name)
		 SELECT p.id, unnest($2::text[])
		 FROM projects p WHERE p.key = $1
		 ON CONFLICT (project_id, name) DO UPDATE SET last_seen_at = NOW()`,
		projectKey, names,
	)
	if err != nil {
		return fmt.Errorf("upserting context attributes: %w", err)
	}
	return nil
}

// ListByProject returns all context attributes for a project, ordered alphabetically by name.
func (s *ContextAttributeStore) ListByProject(ctx context.Context, projectID string) ([]model.ContextAttribute, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, name, last_seen_at
		 FROM context_attributes WHERE project_id = $1 ORDER BY name`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing context attributes: %w", err)
	}
	defer rows.Close()

	var attrs []model.ContextAttribute
	for rows.Next() {
		var a model.ContextAttribute
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.Name, &a.LastSeenAt); err != nil {
			return nil, fmt.Errorf("scanning context attribute: %w", err)
		}
		attrs = append(attrs, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating context attributes: %w", err)
	}
	return attrs, nil
}
