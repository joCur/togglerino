package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type ScheduleStore struct {
	pool *pgxpool.Pool
}

func NewScheduleStore(pool *pgxpool.Pool) *ScheduleStore {
	return &ScheduleStore{pool: pool}
}

// Create inserts a new pending scheduled flag change.
func (s *ScheduleStore) Create(ctx context.Context, flagID, environmentID string, scheduledAt time.Time, snapshot json.RawMessage, createdBy *string) (*model.ScheduledFlagChange, error) {
	var sc model.ScheduledFlagChange
	err := s.pool.QueryRow(ctx,
		`INSERT INTO scheduled_flag_changes
		    (flag_id, environment_id, scheduled_at, config_snapshot, created_by)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, flag_id, environment_id, scheduled_at, status, config_snapshot,
		           created_by, created_at, executed_at, cancelled_at, cancel_reason`,
		flagID, environmentID, scheduledAt, snapshot, createdBy,
	).Scan(
		&sc.ID, &sc.FlagID, &sc.EnvironmentID, &sc.ScheduledAt, &sc.Status,
		&sc.ConfigSnapshot, &sc.CreatedBy, &sc.CreatedAt, &sc.ExecutedAt, &sc.CancelledAt, &sc.CancelReason,
	)
	if err != nil {
		return nil, fmt.Errorf("creating scheduled change: %w", err)
	}
	return &sc, nil
}

// Get returns a single scheduled change by ID.
func (s *ScheduleStore) Get(ctx context.Context, id string) (*model.ScheduledFlagChange, error) {
	var sc model.ScheduledFlagChange
	err := s.pool.QueryRow(ctx,
		`SELECT id, flag_id, environment_id, scheduled_at, status, config_snapshot,
		        created_by, created_at, executed_at, cancelled_at, cancel_reason
		 FROM scheduled_flag_changes WHERE id = $1`,
		id,
	).Scan(
		&sc.ID, &sc.FlagID, &sc.EnvironmentID, &sc.ScheduledAt, &sc.Status,
		&sc.ConfigSnapshot, &sc.CreatedBy, &sc.CreatedAt, &sc.ExecutedAt, &sc.CancelledAt, &sc.CancelReason,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting scheduled change: %w", err)
	}
	return &sc, nil
}

// ListByFlagEnvironment returns all schedules for a flag+environment, ordered by scheduled_at.
func (s *ScheduleStore) ListByFlagEnvironment(ctx context.Context, flagID, environmentID string) ([]model.ScheduledFlagChange, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, flag_id, environment_id, scheduled_at, status, config_snapshot,
		        created_by, created_at, executed_at, cancelled_at, cancel_reason
		 FROM scheduled_flag_changes
		 WHERE flag_id = $1 AND environment_id = $2
		 ORDER BY scheduled_at ASC`,
		flagID, environmentID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing schedules: %w", err)
	}
	defer rows.Close()
	return scanScheduleRows(rows)
}

// Update replaces the scheduled_at and config_snapshot of a pending schedule.
// flagID and environmentID are required to verify ownership.
func (s *ScheduleStore) Update(ctx context.Context, id, flagID, environmentID string, scheduledAt time.Time, snapshot json.RawMessage) (*model.ScheduledFlagChange, error) {
	var sc model.ScheduledFlagChange
	err := s.pool.QueryRow(ctx,
		`UPDATE scheduled_flag_changes
		 SET scheduled_at = $4, config_snapshot = $5
		 WHERE id = $1 AND flag_id = $2 AND environment_id = $3 AND status = 'pending'
		 RETURNING id, flag_id, environment_id, scheduled_at, status, config_snapshot,
		           created_by, created_at, executed_at, cancelled_at, cancel_reason`,
		id, flagID, environmentID, scheduledAt, snapshot,
	).Scan(
		&sc.ID, &sc.FlagID, &sc.EnvironmentID, &sc.ScheduledAt, &sc.Status,
		&sc.ConfigSnapshot, &sc.CreatedBy, &sc.CreatedAt, &sc.ExecutedAt, &sc.CancelledAt, &sc.CancelReason,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("updating scheduled change: %w", err)
	}
	return &sc, nil
}

// Cancel marks a pending schedule as cancelled.
// flagID and environmentID are required to verify ownership.
func (s *ScheduleStore) Cancel(ctx context.Context, id, flagID, environmentID string, reason string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE scheduled_flag_changes
		 SET status = 'cancelled', cancelled_at = NOW(), cancel_reason = $4
		 WHERE id = $1 AND flag_id = $2 AND environment_id = $3 AND status = 'pending'`,
		id, flagID, environmentID, reason,
	)
	if err != nil {
		return fmt.Errorf("cancelling scheduled change: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Fail marks a pending schedule as failed with a reason.
func (s *ScheduleStore) Fail(ctx context.Context, id string, reason string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE scheduled_flag_changes
		 SET status = 'failed', cancelled_at = NOW(), cancel_reason = $2
		 WHERE id = $1 AND status = 'pending'`,
		id, reason,
	)
	if err != nil {
		return fmt.Errorf("failing schedule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CancelByFlag cancels all pending schedules for a flag (used on archive/delete).
func (s *ScheduleStore) CancelByFlag(ctx context.Context, flagID string, reason string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE scheduled_flag_changes
		 SET status = 'cancelled', cancelled_at = NOW(), cancel_reason = $2
		 WHERE flag_id = $1 AND status = 'pending'`,
		flagID, reason,
	)
	if err != nil {
		return fmt.Errorf("cancelling schedules for flag: %w", err)
	}
	return nil
}

// ListDue returns all pending schedules whose scheduled_at <= now.
func (s *ScheduleStore) ListDue(ctx context.Context, now time.Time) ([]model.ScheduledFlagChange, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, flag_id, environment_id, scheduled_at, status, config_snapshot,
		        created_by, created_at, executed_at, cancelled_at, cancel_reason
		 FROM scheduled_flag_changes
		 WHERE status = 'pending' AND scheduled_at <= $1
		 ORDER BY scheduled_at ASC`,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("listing due schedules: %w", err)
	}
	defer rows.Close()
	return scanScheduleRows(rows)
}

// Execute atomically applies the config snapshot and marks the schedule as executed.
// Must be called within a caller-managed transaction.
func (s *ScheduleStore) Execute(ctx context.Context, tx pgx.Tx, scheduleID, flagID, environmentID string, snapshot model.ConfigSnapshotPayload) error {
	cfgTag, err := tx.Exec(ctx,
		`UPDATE flag_environment_configs
		 SET enabled = $3, fallthrough_variant = $4, off_variant = $5, targeting_rules = $6, updated_at = NOW()
		 WHERE flag_id = $1 AND environment_id = $2`,
		flagID, environmentID,
		snapshot.Enabled, snapshot.FallthroughVariant, snapshot.OffVariant, snapshot.TargetingRules,
	)
	if err != nil {
		return fmt.Errorf("applying config snapshot: %w", err)
	}
	if cfgTag.RowsAffected() == 0 {
		return fmt.Errorf("flag_environment_configs row not found for flag %s env %s", flagID, environmentID)
	}

	tag, err := tx.Exec(ctx,
		`UPDATE scheduled_flag_changes
		 SET status = 'executed', executed_at = NOW()
		 WHERE id = $1 AND status = 'pending'`,
		scheduleID,
	)
	if err != nil {
		return fmt.Errorf("marking schedule executed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanScheduleRows(rows pgx.Rows) ([]model.ScheduledFlagChange, error) {
	var results []model.ScheduledFlagChange
	for rows.Next() {
		var sc model.ScheduledFlagChange
		if err := rows.Scan(
			&sc.ID, &sc.FlagID, &sc.EnvironmentID, &sc.ScheduledAt, &sc.Status,
			&sc.ConfigSnapshot, &sc.CreatedBy, &sc.CreatedAt, &sc.ExecutedAt, &sc.CancelledAt, &sc.CancelReason,
		); err != nil {
			return nil, fmt.Errorf("scanning schedule row: %w", err)
		}
		results = append(results, sc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating schedule rows: %w", err)
	}
	return results, nil
}
