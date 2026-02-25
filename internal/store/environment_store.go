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
		`INSERT INTO environments (project_id, key, name) VALUES ($1, $2, $3)
		 RETURNING id, project_id, key, name, created_at`,
		projectID, key, name,
	).Scan(&e.ID, &e.ProjectID, &e.Key, &e.Name, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating environment: %w", err)
	}
	return &e, nil
}

// ListByProject returns all environments for a project.
func (s *EnvironmentStore) ListByProject(ctx context.Context, projectID string) ([]model.Environment, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, key, name, created_at FROM environments WHERE project_id = $1 ORDER BY created_at`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing environments: %w", err)
	}
	defer rows.Close()

	var envs []model.Environment
	for rows.Next() {
		var e model.Environment
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.Key, &e.Name, &e.CreatedAt); err != nil {
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
		`SELECT id, project_id, key, name, created_at FROM environments WHERE project_id = $1 AND key = $2`,
		projectID, key,
	).Scan(&e.ID, &e.ProjectID, &e.Key, &e.Name, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("finding environment by key: %w", err)
	}
	return &e, nil
}

// Delete deletes an environment by ID.
func (s *EnvironmentStore) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM environments WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting environment: %w", err)
	}
	return nil
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

	for _, d := range defaults {
		_, err := s.pool.Exec(ctx,
			`INSERT INTO environments (project_id, key, name) VALUES ($1, $2, $3)`,
			projectID, d.key, d.name,
		)
		if err != nil {
			return fmt.Errorf("creating default environment %q: %w", d.key, err)
		}
	}
	return nil
}
