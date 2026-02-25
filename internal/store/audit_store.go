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
		`INSERT INTO audit_log (project_id, user_id, action, entity_type, entity_id, old_value, new_value)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		entry.ProjectID, entry.UserID, entry.Action, entry.EntityType, entry.EntityID, entry.OldValue, entry.NewValue,
	)
	if err != nil {
		return fmt.Errorf("recording audit entry: %w", err)
	}
	return nil
}

// ListByProject returns audit entries for a project, ordered by created_at DESC, with pagination.
func (s *AuditStore) ListByProject(ctx context.Context, projectID string, limit, offset int) ([]model.AuditEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, user_id, action, entity_type, entity_id, old_value, new_value, created_at
		 FROM audit_log WHERE project_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		projectID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("listing audit entries: %w", err)
	}
	defer rows.Close()

	var entries []model.AuditEntry
	for rows.Next() {
		var e model.AuditEntry
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.UserID, &e.Action, &e.EntityType, &e.EntityID, &e.OldValue, &e.NewValue, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning audit entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating audit entries: %w", err)
	}
	return entries, nil
}
