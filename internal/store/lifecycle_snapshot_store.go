package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type LifecycleSnapshotStore struct {
	pool *pgxpool.Pool
}

func NewLifecycleSnapshotStore(pool *pgxpool.Pool) *LifecycleSnapshotStore {
	return &LifecycleSnapshotStore{pool: pool}
}

// Record inserts or updates a lifecycle snapshot for the given project on the current date.
func (s *LifecycleSnapshotStore) Record(ctx context.Context, projectID string, active, potentiallyStale, stale, archived int) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO lifecycle_snapshots (project_id, active_count, potentially_stale_count, stale_count, archived_count)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (project_id, recorded_at) DO UPDATE
		 SET active_count = EXCLUDED.active_count,
		     potentially_stale_count = EXCLUDED.potentially_stale_count,
		     stale_count = EXCLUDED.stale_count,
		     archived_count = EXCLUDED.archived_count`,
		projectID, active, potentiallyStale, stale, archived,
	)
	if err != nil {
		return fmt.Errorf("recording lifecycle snapshot: %w", err)
	}
	return nil
}

// GetTrends returns lifecycle snapshots for the given project over the last N days, ordered by date ascending.
// Always returns a non-nil slice.
func (s *LifecycleSnapshotStore) GetTrends(ctx context.Context, projectID string, days int) ([]model.LifecycleSnapshot, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT recorded_at, active_count, potentially_stale_count, stale_count, archived_count
		 FROM lifecycle_snapshots
		 WHERE project_id = $1 AND recorded_at >= CURRENT_DATE - $2::int
		 ORDER BY recorded_at ASC`,
		projectID, days,
	)
	if err != nil {
		return nil, fmt.Errorf("querying lifecycle trends: %w", err)
	}
	defer rows.Close()

	snapshots := []model.LifecycleSnapshot{}
	for rows.Next() {
		var snap model.LifecycleSnapshot
		var recordedAt time.Time
		if err := rows.Scan(&recordedAt, &snap.ActiveCount, &snap.PotentiallyStaleCount, &snap.StaleCount, &snap.ArchivedCount); err != nil {
			return nil, fmt.Errorf("scanning lifecycle snapshot: %w", err)
		}
		snap.Date = recordedAt.Format("2006-01-02")
		snapshots = append(snapshots, snap)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating lifecycle snapshots: %w", err)
	}
	return snapshots, nil
}
