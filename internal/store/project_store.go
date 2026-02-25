package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type ProjectStore struct {
	pool *pgxpool.Pool
}

func NewProjectStore(pool *pgxpool.Pool) *ProjectStore {
	return &ProjectStore{pool: pool}
}

// Create inserts a new project.
func (s *ProjectStore) Create(ctx context.Context, key, name, description string) (*model.Project, error) {
	var p model.Project
	err := s.pool.QueryRow(ctx,
		`INSERT INTO projects (key, name, description) VALUES ($1, $2, $3)
		 RETURNING id, key, name, description, created_at, updated_at`,
		key, name, description,
	).Scan(&p.ID, &p.Key, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating project: %w", err)
	}
	return &p, nil
}

// List returns all projects.
func (s *ProjectStore) List(ctx context.Context) ([]model.Project, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, key, name, description, created_at, updated_at FROM projects ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	defer rows.Close()

	var projects []model.Project
	for rows.Next() {
		var p model.Project
		if err := rows.Scan(&p.ID, &p.Key, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning project: %w", err)
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating projects: %w", err)
	}
	return projects, nil
}

// FindByKey returns a project by its key.
func (s *ProjectStore) FindByKey(ctx context.Context, key string) (*model.Project, error) {
	var p model.Project
	err := s.pool.QueryRow(ctx,
		`SELECT id, key, name, description, created_at, updated_at FROM projects WHERE key = $1`,
		key,
	).Scan(&p.ID, &p.Key, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("finding project by key: %w", err)
	}
	return &p, nil
}

// Update updates a project's name and description.
func (s *ProjectStore) Update(ctx context.Context, key, name, description string) (*model.Project, error) {
	var p model.Project
	err := s.pool.QueryRow(ctx,
		`UPDATE projects SET name=$2, description=$3, updated_at=NOW() WHERE key=$1
		 RETURNING id, key, name, description, created_at, updated_at`,
		key, name, description,
	).Scan(&p.ID, &p.Key, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating project: %w", err)
	}
	return &p, nil
}

// Delete deletes a project by key.
func (s *ProjectStore) Delete(ctx context.Context, key string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM projects WHERE key = $1`, key)
	if err != nil {
		return fmt.Errorf("deleting project: %w", err)
	}
	return nil
}
