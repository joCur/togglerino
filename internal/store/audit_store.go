package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type AuditStore struct {
	pool *pgxpool.Pool
}

func NewAuditStore(pool *pgxpool.Pool) *AuditStore {
	return &AuditStore{pool: pool}
}

// Record inserts an audit log entry.
func (s *AuditStore) Record(ctx context.Context, entry model.AuditEntry) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO audit_log (project_id, user_id, user_email, environment_id, batch_id, action, entity_type, entity_id, old_value, new_value)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		entry.ProjectID, entry.UserID, entry.UserEmail, entry.EnvironmentID, entry.BatchID, entry.Action, entry.EntityType, entry.EntityID, entry.OldValue, entry.NewValue,
	)
	if err != nil {
		return fmt.Errorf("recording audit entry: %w", err)
	}
	return nil
}

// ListByProject returns audit entries for a project, ordered by created_at DESC, with pagination.
// Returns the entries, total count (before pagination), and any error.
func (s *AuditStore) ListByProject(ctx context.Context, projectID string, limit, offset int) ([]model.AuditEntry, int, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, user_id, user_email, environment_id, batch_id, action, entity_type, entity_id, old_value, new_value, created_at, COUNT(*) OVER() AS total_count
		 FROM audit_log WHERE project_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		projectID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("listing audit entries: %w", err)
	}
	defer rows.Close()

	var entries []model.AuditEntry
	totalCount := 0
	for rows.Next() {
		var e model.AuditEntry
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.UserID, &e.UserEmail, &e.EnvironmentID, &e.BatchID, &e.Action, &e.EntityType, &e.EntityID, &e.OldValue, &e.NewValue, &e.CreatedAt, &totalCount); err != nil {
			return nil, 0, fmt.Errorf("scanning audit entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating audit entries: %w", err)
	}
	return entries, totalCount, nil
}

// GetByID returns a single audit entry by its ID.
func (s *AuditStore) GetByID(ctx context.Context, id string) (*model.AuditEntry, error) {
	var e model.AuditEntry
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, user_id, user_email, environment_id, batch_id, action, entity_type, entity_id, old_value, new_value, created_at
		 FROM audit_log WHERE id = $1`,
		id,
	).Scan(&e.ID, &e.ProjectID, &e.UserID, &e.UserEmail, &e.EnvironmentID, &e.BatchID, &e.Action, &e.EntityType, &e.EntityID, &e.OldValue, &e.NewValue, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting audit entry by id: %w", err)
	}
	return &e, nil
}

// ListByFlag returns audit entries for a specific flag, optionally filtered by environment.
// Returns the entries, total count (before pagination), and any error.
func (s *AuditStore) ListByFlag(ctx context.Context, projectID, flagKey string, envID *string, limit, offset int) ([]model.AuditEntry, int, error) {
	query := `SELECT id, project_id, user_id, user_email, environment_id, batch_id, action, entity_type, entity_id, old_value, new_value, created_at, COUNT(*) OVER() AS total_count
		 FROM audit_log
		 WHERE project_id = $1 AND entity_id = $2 AND entity_type IN ('flag', 'flag_config')`
	args := []any{projectID, flagKey}
	argIdx := 3

	if envID != nil {
		query += fmt.Sprintf(" AND (environment_id = $%d OR environment_id IS NULL)", argIdx)
		args = append(args, *envID)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing audit entries by flag: %w", err)
	}
	defer rows.Close()

	var entries []model.AuditEntry
	totalCount := 0
	for rows.Next() {
		var e model.AuditEntry
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.UserID, &e.UserEmail, &e.EnvironmentID, &e.BatchID, &e.Action, &e.EntityType, &e.EntityID, &e.OldValue, &e.NewValue, &e.CreatedAt, &totalCount); err != nil {
			return nil, 0, fmt.Errorf("scanning audit entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating audit entries: %w", err)
	}
	return entries, totalCount, nil
}
