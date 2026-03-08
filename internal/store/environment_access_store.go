package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EnvironmentAccessRestriction represents the set of environments a role
// is allowed to access within a project. An empty slice (no rows) means
// the role is unrestricted.
type EnvironmentAccessRestriction struct {
	RoleName       string   `json:"role_name"`
	EnvironmentIDs []string `json:"environment_ids"`
}

// EnvironmentAccessStore manages per-project, per-role environment access restrictions.
type EnvironmentAccessStore struct {
	pool *pgxpool.Pool
}

// NewEnvironmentAccessStore creates a new EnvironmentAccessStore.
func NewEnvironmentAccessStore(pool *pgxpool.Pool) *EnvironmentAccessStore {
	return &EnvironmentAccessStore{pool: pool}
}

// ListByProjectAndRole returns the environment IDs that a role is restricted to
// within a project. An empty slice means the role is unrestricted.
func (s *EnvironmentAccessStore) ListByProjectAndRole(ctx context.Context, projectID, roleName string) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT environment_id FROM project_environment_access
		 WHERE project_id = $1 AND role_name = $2`,
		projectID, roleName,
	)
	if err != nil {
		return nil, fmt.Errorf("listing environment access: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning environment access: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating environment access: %w", err)
	}
	return ids, nil
}

// ListByProject returns all environment access restrictions for a project,
// grouped by role name.
func (s *EnvironmentAccessStore) ListByProject(ctx context.Context, projectID string) ([]EnvironmentAccessRestriction, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT role_name, environment_id FROM project_environment_access
		 WHERE project_id = $1
		 ORDER BY role_name, environment_id`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing project environment access: %w", err)
	}
	defer rows.Close()

	byRole := make(map[string][]string)
	var roleOrder []string
	for rows.Next() {
		var roleName, envID string
		if err := rows.Scan(&roleName, &envID); err != nil {
			return nil, fmt.Errorf("scanning project environment access: %w", err)
		}
		if _, exists := byRole[roleName]; !exists {
			roleOrder = append(roleOrder, roleName)
		}
		byRole[roleName] = append(byRole[roleName], envID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating project environment access: %w", err)
	}

	restrictions := make([]EnvironmentAccessRestriction, 0, len(byRole))
	for _, role := range roleOrder {
		restrictions = append(restrictions, EnvironmentAccessRestriction{
			RoleName:       role,
			EnvironmentIDs: byRole[role],
		})
	}
	return restrictions, nil
}

// ReplaceForProject atomically replaces all environment access restrictions
// for a project. Deletes existing rows and inserts new ones in a transaction.
func (s *EnvironmentAccessStore) ReplaceForProject(ctx context.Context, projectID string, restrictions []EnvironmentAccessRestriction) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Delete all existing restrictions for the project
	_, err = tx.Exec(ctx,
		`DELETE FROM project_environment_access WHERE project_id = $1`,
		projectID,
	)
	if err != nil {
		return fmt.Errorf("clearing environment access: %w", err)
	}

	// Insert new restrictions
	for _, r := range restrictions {
		for _, envID := range r.EnvironmentIDs {
			_, err = tx.Exec(ctx,
				`INSERT INTO project_environment_access (project_id, role_name, environment_id)
				 VALUES ($1, $2, $3)`,
				projectID, r.RoleName, envID,
			)
			if err != nil {
				return fmt.Errorf("inserting environment access for role %q: %w", r.RoleName, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing environment access: %w", err)
	}
	return nil
}

// HasAccess checks whether a role has access to a specific environment (by ID)
// within a project. If no restrictions exist for the role, it is considered
// unrestricted and returns true.
func (s *EnvironmentAccessStore) HasAccess(ctx context.Context, projectID, roleName, environmentID string) (bool, error) {
	var restrictionCount int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM project_environment_access
		 WHERE project_id = $1 AND role_name = $2`,
		projectID, roleName,
	).Scan(&restrictionCount)
	if err != nil {
		return false, fmt.Errorf("counting environment restrictions: %w", err)
	}

	// No restrictions means unrestricted access
	if restrictionCount == 0 {
		return true, nil
	}

	var matchCount int
	err = s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM project_environment_access
		 WHERE project_id = $1 AND role_name = $2 AND environment_id = $3`,
		projectID, roleName, environmentID,
	).Scan(&matchCount)
	if err != nil {
		return false, fmt.Errorf("checking environment access: %w", err)
	}

	return matchCount > 0, nil
}

// HasAccessByEnvKey checks whether a role has access to a specific environment
// (resolved by key) within a project. If no restrictions exist for the role,
// it is considered unrestricted and returns true.
func (s *EnvironmentAccessStore) HasAccessByEnvKey(ctx context.Context, projectID, roleName, envKey string) (bool, error) {
	var restrictionCount int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM project_environment_access
		 WHERE project_id = $1 AND role_name = $2`,
		projectID, roleName,
	).Scan(&restrictionCount)
	if err != nil {
		return false, fmt.Errorf("counting environment restrictions: %w", err)
	}

	// No restrictions means unrestricted access
	if restrictionCount == 0 {
		return true, nil
	}

	var matchCount int
	err = s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM project_environment_access pea
		 JOIN environments e ON e.id = pea.environment_id
		 WHERE pea.project_id = $1 AND pea.role_name = $2 AND e.key = $3 AND e.project_id = $1`,
		projectID, roleName, envKey,
	).Scan(&matchCount)
	if err != nil {
		return false, fmt.Errorf("checking environment access by key: %w", err)
	}

	return matchCount > 0, nil
}
