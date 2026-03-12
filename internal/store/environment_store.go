package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type EnvironmentStore struct {
	pool *pgxpool.Pool
}

func NewEnvironmentStore(pool *pgxpool.Pool) *EnvironmentStore {
	return &EnvironmentStore{pool: pool}
}

// Create inserts a new environment for a project.
func (s *EnvironmentStore) Create(ctx context.Context, projectID, key, name string) (*model.Environment, error) {
	var e model.Environment
	err := s.pool.QueryRow(ctx,
		`INSERT INTO environments (project_id, key, name, sort_order)
		 VALUES ($1, $2, $3, COALESCE((SELECT MAX(sort_order) + 1 FROM environments WHERE project_id = $1), 0))
		 RETURNING id, project_id, key, name, sort_order, created_at`,
		projectID, key, name,
	).Scan(&e.ID, &e.ProjectID, &e.Key, &e.Name, &e.SortOrder, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating environment: %w", err)
	}
	return &e, nil
}

// ListByProject returns all environments for a project.
func (s *EnvironmentStore) ListByProject(ctx context.Context, projectID string) ([]model.Environment, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, key, name, sort_order, created_at FROM environments WHERE project_id = $1 ORDER BY sort_order`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing environments: %w", err)
	}
	defer rows.Close()

	var envs []model.Environment
	for rows.Next() {
		var e model.Environment
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.Key, &e.Name, &e.SortOrder, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning environment: %w", err)
		}
		envs = append(envs, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating environments: %w", err)
	}
	return envs, nil
}

// FindByKey returns an environment by project ID and environment key.
func (s *EnvironmentStore) FindByKey(ctx context.Context, projectID, key string) (*model.Environment, error) {
	var e model.Environment
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, key, name, sort_order, created_at FROM environments WHERE project_id = $1 AND key = $2`,
		projectID, key,
	).Scan(&e.ID, &e.ProjectID, &e.Key, &e.Name, &e.SortOrder, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("finding environment by key: %w", err)
	}
	return &e, nil
}

// FindByID returns an environment by its ID.
func (s *EnvironmentStore) FindByID(ctx context.Context, id string) (*model.Environment, error) {
	var e model.Environment
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, key, name, sort_order, created_at FROM environments WHERE id = $1`, id,
	).Scan(&e.ID, &e.ProjectID, &e.Key, &e.Name, &e.SortOrder, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("finding environment by id: %w", err)
	}
	return &e, nil
}

// DeleteIfNotLast deletes an environment in a transaction, guarding against deleting the last one.
func (s *EnvironmentStore) DeleteIfNotLast(ctx context.Context, envID, projectID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var count int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM (SELECT 1 FROM environments WHERE project_id = $1 FOR UPDATE) AS locked`, projectID,
	).Scan(&count); err != nil {
		return fmt.Errorf("counting environments: %w", err)
	}
	if count <= 1 {
		return ErrLastEnvironment
	}

	tag, err := tx.Exec(ctx, `DELETE FROM environments WHERE id = $1 AND project_id = $2`, envID, projectID)
	if err != nil {
		return fmt.Errorf("deleting environment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("environment not found")
	}

	return tx.Commit(ctx)
}

// EnvKeyByID returns the environment key for an environment ID (used by schedule checker).
func (s *EnvironmentStore) EnvKeyByID(ctx context.Context, environmentID string) (string, error) {
	var key string
	err := s.pool.QueryRow(ctx,
		`SELECT key FROM environments WHERE id = $1`,
		environmentID,
	).Scan(&key)
	if err != nil {
		return "", fmt.Errorf("looking up environment key by ID: %w", err)
	}
	return key, nil
}

// CreateDefaultEnvironments creates development, staging, production environments for a project.
func (s *EnvironmentStore) CreateDefaultEnvironments(ctx context.Context, projectID string) error {
	defaults := []struct {
		key  string
		name string
	}{
		{"development", "Development"},
		{"staging", "Staging"},
		{"production", "Production"},
	}

	for i, d := range defaults {
		_, err := s.pool.Exec(ctx,
			`INSERT INTO environments (project_id, key, name, sort_order) VALUES ($1, $2, $3, $4)`,
			projectID, d.key, d.name, i,
		)
		if err != nil {
			return fmt.Errorf("creating default environment %q: %w", d.key, err)
		}
	}
	return nil
}

// UpdateOrder reorders environments within a project. environmentIDs must contain all environment IDs for the project.
func (s *EnvironmentStore) UpdateOrder(ctx context.Context, projectID string, environmentIDs []string) error {
	// Validate that the list contains all environments for the project
	var count int
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM environments WHERE project_id = $1`, projectID,
	).Scan(&count); err != nil {
		return fmt.Errorf("counting environments: %w", err)
	}
	if len(environmentIDs) != count {
		return fmt.Errorf("expected %d environment IDs, got %d: must include all environments", count, len(environmentIDs))
	}

	// Check for duplicate IDs
	seen := make(map[string]bool, len(environmentIDs))
	for _, id := range environmentIDs {
		if seen[id] {
			return fmt.Errorf("duplicate environment ID: %s", id)
		}
		seen[id] = true
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for i, id := range environmentIDs {
		tag, err := tx.Exec(ctx,
			`UPDATE environments SET sort_order = $1 WHERE id = $2 AND project_id = $3`,
			i, id, projectID,
		)
		if err != nil {
			return fmt.Errorf("updating sort_order for environment %s: %w", id, err)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("environment %s not found in project", id)
		}
	}

	return tx.Commit(ctx)
}
