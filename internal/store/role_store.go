package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

// RoleStore manages role definitions in the database.
type RoleStore struct {
	pool *pgxpool.Pool
}

// NewRoleStore creates a new RoleStore.
func NewRoleStore(pool *pgxpool.Pool) *RoleStore {
	return &RoleStore{pool: pool}
}

// List returns all role definitions, ordered by built-in first then by name.
func (s *RoleStore) List(ctx context.Context) ([]model.RoleDefinition, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, description, permissions, is_built_in, created_at, updated_at
		 FROM roles
		 ORDER BY is_built_in DESC, name ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing roles: %w", err)
	}
	defer rows.Close()

	var roles []model.RoleDefinition
	for rows.Next() {
		var r model.RoleDefinition
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.Permissions, &r.IsBuiltIn, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning role: %w", err)
		}
		roles = append(roles, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating roles: %w", err)
	}
	return roles, nil
}

// GetByName returns a role definition by its unique name.
func (s *RoleStore) GetByName(ctx context.Context, name string) (*model.RoleDefinition, error) {
	var r model.RoleDefinition
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, description, permissions, is_built_in, created_at, updated_at
		 FROM roles WHERE name = $1`,
		name,
	).Scan(&r.ID, &r.Name, &r.Description, &r.Permissions, &r.IsBuiltIn, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting role by name: %w", err)
	}
	return &r, nil
}

// Create inserts a new custom role definition.
func (s *RoleStore) Create(ctx context.Context, name, description string, permissions []string) (*model.RoleDefinition, error) {
	var r model.RoleDefinition
	err := s.pool.QueryRow(ctx,
		`INSERT INTO roles (name, description, permissions, is_built_in)
		 VALUES ($1, $2, $3, false)
		 RETURNING id, name, description, permissions, is_built_in, created_at, updated_at`,
		name, description, permissions,
	).Scan(&r.ID, &r.Name, &r.Description, &r.Permissions, &r.IsBuiltIn, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating role: %w", err)
	}
	return &r, nil
}

// Update modifies an existing custom role. Built-in roles cannot be updated.
func (s *RoleStore) Update(ctx context.Context, name, description string, permissions []string) (*model.RoleDefinition, error) {
	var r model.RoleDefinition
	err := s.pool.QueryRow(ctx,
		`UPDATE roles SET description = $2, permissions = $3, updated_at = NOW()
		 WHERE name = $1 AND is_built_in = false
		 RETURNING id, name, description, permissions, is_built_in, created_at, updated_at`,
		name, description, permissions,
	).Scan(&r.ID, &r.Name, &r.Description, &r.Permissions, &r.IsBuiltIn, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("updating role: role not found or is built-in: %w", err)
		}
		return nil, fmt.Errorf("updating role: %w", err)
	}
	return &r, nil
}

// Delete removes a custom role. Returns an error if the role is built-in or in use.
func (s *RoleStore) Delete(ctx context.Context, name string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Check if the role exists and is not built-in.
	var isBuiltIn bool
	err = tx.QueryRow(ctx,
		`SELECT is_built_in FROM roles WHERE name = $1 FOR UPDATE`,
		name,
	).Scan(&isBuiltIn)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("deleting role: %w", ErrNotFound)
		}
		return fmt.Errorf("deleting role: %w", err)
	}
	if isBuiltIn {
		return fmt.Errorf("deleting role: %w", ErrBuiltInRole)
	}

	// Check if any project members use this role.
	var memberCount int
	err = tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM project_members WHERE role = $1`,
		name,
	).Scan(&memberCount)
	if err != nil {
		return fmt.Errorf("checking role usage in project members: %w", err)
	}
	if memberCount > 0 {
		return fmt.Errorf("deleting role: %w", ErrRoleInUse)
	}

	// Check if the role is used as the org base project role.
	var baseRoleCount int
	err = tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM org_settings WHERE key = 'base_project_role' AND value = $1`,
		name,
	).Scan(&baseRoleCount)
	if err != nil {
		return fmt.Errorf("checking role usage in org settings: %w", err)
	}
	if baseRoleCount > 0 {
		return fmt.Errorf("deleting role: %w", ErrRoleInUse)
	}

	// Delete the role.
	tag, err := tx.Exec(ctx, `DELETE FROM roles WHERE name = $1`, name)
	if err != nil {
		return fmt.Errorf("deleting role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("deleting role: %w", ErrNotFound)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// Exists returns true if a role with the given name exists.
func (s *RoleStore) Exists(ctx context.Context, name string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM roles WHERE name = $1)`,
		name,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking role existence: %w", err)
	}
	return exists, nil
}
