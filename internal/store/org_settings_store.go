package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// OrgSettingsStore manages organization-wide settings stored in the org_settings table.
type OrgSettingsStore struct {
	pool *pgxpool.Pool
}

// NewOrgSettingsStore creates a new OrgSettingsStore.
func NewOrgSettingsStore(pool *pgxpool.Pool) *OrgSettingsStore {
	return &OrgSettingsStore{pool: pool}
}

// GetBaseProjectRole returns the default project role assigned to all users
// who do not have an explicit project membership.
func (s *OrgSettingsStore) GetBaseProjectRole(ctx context.Context) (string, error) {
	var value string
	err := s.pool.QueryRow(ctx,
		`SELECT value FROM org_settings WHERE key = 'base_project_role'`,
	).Scan(&value)
	if err != nil {
		return "", fmt.Errorf("getting base project role: %w", err)
	}
	return value, nil
}

// SetBaseProjectRole updates the default project role. The role must exist in
// the roles table, or be "none" to require explicit project membership.
func (s *OrgSettingsStore) SetBaseProjectRole(ctx context.Context, role string) error {
	if role != "none" {
		var exists bool
		err := s.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM roles WHERE name = $1)`, role,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("checking role: %w", err)
		}
		if !exists {
			return fmt.Errorf("invalid base project role: %q", role)
		}
	}

	_, err := s.pool.Exec(ctx,
		`INSERT INTO org_settings (key, value) VALUES ('base_project_role', $1)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		role,
	)
	if err != nil {
		return fmt.Errorf("setting base project role: %w", err)
	}
	return nil
}
