package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

// ProjectMemberStore manages project membership records.
type ProjectMemberStore struct {
	pool *pgxpool.Pool
}

// NewProjectMemberStore creates a new ProjectMemberStore.
func NewProjectMemberStore(pool *pgxpool.Pool) *ProjectMemberStore {
	return &ProjectMemberStore{pool: pool}
}

// Add creates a project membership for a user with the given role.
func (s *ProjectMemberStore) Add(ctx context.Context, projectID, userID string, role model.ProjectRole) (*model.ProjectMember, error) {
	var m model.ProjectMember
	err := s.pool.QueryRow(ctx,
		`INSERT INTO project_members (project_id, user_id, role)
		 VALUES ($1, $2, $3)
		 RETURNING project_id, user_id, role, created_at, updated_at`,
		projectID, userID, role,
	).Scan(&m.ProjectID, &m.UserID, &m.Role, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("adding project member: %w", err)
	}
	return &m, nil
}

// GetRole returns the project role for a specific user in a project.
// Returns an error wrapping pgx.ErrNoRows if no membership exists.
func (s *ProjectMemberStore) GetRole(ctx context.Context, projectID, userID string) (model.ProjectRole, error) {
	var role model.ProjectRole
	err := s.pool.QueryRow(ctx,
		`SELECT role FROM project_members WHERE project_id = $1 AND user_id = $2`,
		projectID, userID,
	).Scan(&role)
	if err != nil {
		return "", fmt.Errorf("getting project member role: %w", err)
	}
	return role, nil
}

// Update changes the role of an existing project membership.
func (s *ProjectMemberStore) Update(ctx context.Context, projectID, userID string, role model.ProjectRole) (*model.ProjectMember, error) {
	var m model.ProjectMember
	err := s.pool.QueryRow(ctx,
		`UPDATE project_members SET role = $3, updated_at = NOW()
		 WHERE project_id = $1 AND user_id = $2
		 RETURNING project_id, user_id, role, created_at, updated_at`,
		projectID, userID, role,
	).Scan(&m.ProjectID, &m.UserID, &m.Role, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating project member: %w", err)
	}
	return &m, nil
}

// Remove deletes a project membership.
func (s *ProjectMemberStore) Remove(ctx context.Context, projectID, userID string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM project_members WHERE project_id = $1 AND user_id = $2`,
		projectID, userID,
	)
	if err != nil {
		return fmt.Errorf("removing project member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("project member not found")
	}
	return nil
}

// ListByProject returns all members of a project with their user details.
func (s *ProjectMemberStore) ListByProject(ctx context.Context, projectID string) ([]model.ProjectMemberWithUser, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT pm.project_id, pm.user_id, pm.role, u.email, u.display_name, pm.created_at, pm.updated_at
		 FROM project_members pm
		 JOIN users u ON u.id = pm.user_id
		 WHERE pm.project_id = $1
		 ORDER BY pm.created_at`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing project members: %w", err)
	}
	defer rows.Close()

	var members []model.ProjectMemberWithUser
	for rows.Next() {
		var m model.ProjectMemberWithUser
		if err := rows.Scan(&m.ProjectID, &m.UserID, &m.Role, &m.Email, &m.DisplayName, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning project member: %w", err)
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating project members: %w", err)
	}
	return members, nil
}

// ListByUser returns all projects a user is a member of with their role.
func (s *ProjectMemberStore) ListByUser(ctx context.Context, userID string) ([]model.UserProjectAssignment, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT p.id, p.key, p.name, pm.role
		 FROM project_members pm
		 JOIN projects p ON p.id = pm.project_id
		 WHERE pm.user_id = $1
		 ORDER BY p.name`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing user project assignments: %w", err)
	}
	defer rows.Close()

	var assignments []model.UserProjectAssignment
	for rows.Next() {
		var a model.UserProjectAssignment
		if err := rows.Scan(&a.ProjectID, &a.ProjectKey, &a.ProjectName, &a.Role); err != nil {
			return nil, fmt.Errorf("scanning user project assignment: %w", err)
		}
		assignments = append(assignments, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating user project assignments: %w", err)
	}
	return assignments, nil
}

// ListAccessibleProjectIDs returns the IDs of all projects a user is a member of.
func (s *ProjectMemberStore) ListAccessibleProjectIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT project_id FROM project_members WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing accessible project IDs: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning project ID: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating project IDs: %w", err)
	}
	return ids, nil
}
