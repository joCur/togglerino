package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type UnknownFlagStore struct {
	pool *pgxpool.Pool
}

func NewUnknownFlagStore(pool *pgxpool.Pool) *UnknownFlagStore {
	return &UnknownFlagStore{pool: pool}
}

// Upsert records an unknown flag evaluation. If the flag_key already exists for the
// given project+environment, it increments request_count, updates last_seen_at, and
// clears any previous dismissal.
func (s *UnknownFlagStore) Upsert(ctx context.Context, projectID, environmentID, flagKey string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO unknown_flags (project_id, environment_id, flag_key)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (project_id, environment_id, flag_key) DO UPDATE
		 SET request_count = unknown_flags.request_count + 1,
		     last_seen_at = now(),
		     dismissed_at = NULL`,
		projectID, environmentID, flagKey,
	)
	if err != nil {
		return fmt.Errorf("upserting unknown flag: %w", err)
	}
	return nil
}

// ListByProject returns all non-dismissed unknown flags for a project, ordered by
// last_seen_at descending. Environment key and name are populated via JOIN.
func (s *UnknownFlagStore) ListByProject(ctx context.Context, projectID string) ([]model.UnknownFlag, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT uf.id, uf.project_id, uf.environment_id, uf.flag_key,
		        uf.request_count, uf.first_seen_at, uf.last_seen_at,
		        e.key, e.name
		 FROM unknown_flags uf
		 JOIN environments e ON e.id = uf.environment_id
		 WHERE uf.project_id = $1 AND uf.dismissed_at IS NULL
		 ORDER BY uf.last_seen_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing unknown flags: %w", err)
	}
	defer rows.Close()

	var flags []model.UnknownFlag
	for rows.Next() {
		var f model.UnknownFlag
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.EnvironmentID, &f.FlagKey,
			&f.RequestCount, &f.FirstSeenAt, &f.LastSeenAt,
			&f.EnvironmentKey, &f.EnvironmentName); err != nil {
			return nil, fmt.Errorf("scanning unknown flag: %w", err)
		}
		flags = append(flags, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating unknown flags: %w", err)
	}
	if flags == nil {
		flags = []model.UnknownFlag{}
	}
	return flags, nil
}

// Dismiss soft-deletes an unknown flag by setting dismissed_at.
func (s *UnknownFlagStore) Dismiss(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE unknown_flags SET dismissed_at = now() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("dismissing unknown flag: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("unknown flag not found")
	}
	return nil
}

// DeleteByProjectAndKey permanently deletes all unknown_flags rows matching the
// given project and flag key (across all environments). Used when the flag is
// created in the system so the unknown entries are no longer relevant.
func (s *UnknownFlagStore) DeleteByProjectAndKey(ctx context.Context, projectID, flagKey string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM unknown_flags WHERE project_id = $1 AND flag_key = $2`,
		projectID, flagKey,
	)
	if err != nil {
		return fmt.Errorf("deleting unknown flags: %w", err)
	}
	return nil
}
