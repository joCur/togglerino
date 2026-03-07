package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type OverrideStore struct {
	pool *pgxpool.Pool
}

func NewOverrideStore(pool *pgxpool.Pool) *OverrideStore {
	return &OverrideStore{pool: pool}
}

func (s *OverrideStore) Set(ctx context.Context, userID, flagID, environmentID string, value json.RawMessage, expiresAt *time.Time) (*model.FlagOverride, error) {
	var o model.FlagOverride
	err := s.pool.QueryRow(ctx,
		`INSERT INTO flag_overrides (user_id, flag_id, environment_id, value, expires_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (user_id, flag_id, environment_id)
		 DO UPDATE SET value = EXCLUDED.value, expires_at = EXCLUDED.expires_at
		 RETURNING id, user_id, flag_id, environment_id, value, expires_at, created_at`,
		userID, flagID, environmentID, value, expiresAt,
	).Scan(&o.ID, &o.UserID, &o.FlagID, &o.EnvironmentID, &o.Value, &o.ExpiresAt, &o.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("setting override: %w", err)
	}
	return &o, nil
}

func (s *OverrideStore) Get(ctx context.Context, userID, flagID, environmentID string) (*model.FlagOverride, error) {
	var o model.FlagOverride
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, flag_id, environment_id, value, expires_at, created_at
		 FROM flag_overrides
		 WHERE user_id = $1 AND flag_id = $2 AND environment_id = $3`,
		userID, flagID, environmentID,
	).Scan(&o.ID, &o.UserID, &o.FlagID, &o.EnvironmentID, &o.Value, &o.ExpiresAt, &o.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting override: %w", err)
	}
	return &o, nil
}

func (s *OverrideStore) Delete(ctx context.Context, userID, flagID, environmentID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM flag_overrides WHERE user_id = $1 AND flag_id = $2 AND environment_id = $3`,
		userID, flagID, environmentID,
	)
	if err != nil {
		return fmt.Errorf("deleting override: %w", err)
	}
	return nil
}

func (s *OverrideStore) DeleteByUserAndFlag(ctx context.Context, userID, flagID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM flag_overrides WHERE user_id = $1 AND flag_id = $2`,
		userID, flagID,
	)
	if err != nil {
		return fmt.Errorf("deleting overrides for flag: %w", err)
	}
	return nil
}

func (s *OverrideStore) ListByUser(ctx context.Context, userID string) ([]model.FlagOverride, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT fo.id, fo.user_id, fo.flag_id, f.key, fo.environment_id, e.key, p.key, fo.value, fo.expires_at, fo.created_at
		 FROM flag_overrides fo
		 JOIN flags f ON f.id = fo.flag_id
		 JOIN environments e ON e.id = fo.environment_id
		 JOIN projects p ON p.id = f.project_id
		 WHERE fo.user_id = $1 AND (fo.expires_at IS NULL OR fo.expires_at > NOW())
		 ORDER BY fo.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing overrides: %w", err)
	}
	defer rows.Close()

	var overrides []model.FlagOverride
	for rows.Next() {
		var o model.FlagOverride
		if err := rows.Scan(&o.ID, &o.UserID, &o.FlagID, &o.FlagKey, &o.EnvironmentID, &o.EnvironmentKey, &o.ProjectKey, &o.Value, &o.ExpiresAt, &o.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning override: %w", err)
		}
		overrides = append(overrides, o)
	}
	if overrides == nil {
		overrides = []model.FlagOverride{}
	}
	return overrides, rows.Err()
}

// ListByUserAndFlag returns all non-expired overrides for a specific user and flag.
func (s *OverrideStore) ListByUserAndFlag(ctx context.Context, userID, flagID string) ([]model.FlagOverride, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT fo.id, fo.user_id, fo.flag_id, f.key, fo.environment_id, e.key, p.key, fo.value, fo.expires_at, fo.created_at
		 FROM flag_overrides fo
		 JOIN flags f ON f.id = fo.flag_id
		 JOIN environments e ON e.id = fo.environment_id
		 JOIN projects p ON p.id = f.project_id
		 WHERE fo.user_id = $1 AND fo.flag_id = $2 AND (fo.expires_at IS NULL OR fo.expires_at > NOW())
		 ORDER BY fo.created_at DESC`,
		userID, flagID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing overrides by user and flag: %w", err)
	}
	defer rows.Close()

	var overrides []model.FlagOverride
	for rows.Next() {
		var o model.FlagOverride
		if err := rows.Scan(&o.ID, &o.UserID, &o.FlagID, &o.FlagKey, &o.EnvironmentID, &o.EnvironmentKey, &o.ProjectKey, &o.Value, &o.ExpiresAt, &o.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning override: %w", err)
		}
		overrides = append(overrides, o)
	}
	if overrides == nil {
		overrides = []model.FlagOverride{}
	}
	return overrides, rows.Err()
}

// ListByProjectEnv returns all non-expired overrides for a project+environment.
func (s *OverrideStore) ListByProjectEnv(ctx context.Context, projectKey, envKey string) ([]model.FlagOverride, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT fo.id, fo.user_id, fo.flag_id, f.key, fo.environment_id, fo.value, fo.expires_at, fo.created_at
		 FROM flag_overrides fo
		 JOIN flags f ON f.id = fo.flag_id
		 JOIN environments e ON e.id = fo.environment_id
		 JOIN projects p ON p.id = f.project_id
		 WHERE p.key = $1 AND e.key = $2
		   AND (fo.expires_at IS NULL OR fo.expires_at > NOW())`,
		projectKey, envKey,
	)
	if err != nil {
		return nil, fmt.Errorf("listing overrides by project env: %w", err)
	}
	defer rows.Close()

	var overrides []model.FlagOverride
	for rows.Next() {
		var o model.FlagOverride
		if err := rows.Scan(&o.ID, &o.UserID, &o.FlagID, &o.FlagKey, &o.EnvironmentID, &o.Value, &o.ExpiresAt, &o.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning override: %w", err)
		}
		overrides = append(overrides, o)
	}
	if overrides == nil {
		overrides = []model.FlagOverride{}
	}
	return overrides, rows.Err()
}

// ListAllOverrides loads all non-expired overrides with app_user_id resolved.
// Used for cache LoadAll at startup.
func (s *OverrideStore) ListAllOverrides(ctx context.Context) ([]OverrideCacheEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT p.key, e.key, f.key, uai.app_user_id, fo.value, fo.expires_at
		 FROM flag_overrides fo
		 JOIN flags f ON f.id = fo.flag_id
		 JOIN environments e ON e.id = fo.environment_id
		 JOIN projects p ON p.id = f.project_id
		 JOIN user_app_identities uai ON uai.user_id = fo.user_id AND uai.project_id = f.project_id
		 WHERE fo.expires_at IS NULL OR fo.expires_at > NOW()`)
	if err != nil {
		return nil, fmt.Errorf("listing all overrides: %w", err)
	}
	defer rows.Close()

	var entries []OverrideCacheEntry
	for rows.Next() {
		var e OverrideCacheEntry
		if err := rows.Scan(&e.ProjectKey, &e.EnvironmentKey, &e.FlagKey, &e.AppUserID, &e.Value, &e.ExpiresAt); err != nil {
			return nil, fmt.Errorf("scanning override cache entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *OverrideStore) DeleteExpired(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM flag_overrides WHERE expires_at IS NOT NULL AND expires_at <= NOW()`)
	if err != nil {
		return 0, fmt.Errorf("deleting expired overrides: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (s *OverrideStore) DeleteByUserAndProject(ctx context.Context, userID, projectID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM flag_overrides WHERE user_id = $1 AND flag_id IN (SELECT id FROM flags WHERE project_id = $2)`,
		userID, projectID,
	)
	if err != nil {
		return fmt.Errorf("deleting overrides for user in project: %w", err)
	}
	return nil
}

func (s *OverrideStore) DeleteAllByUser(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM flag_overrides WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("deleting all overrides for user: %w", err)
	}
	return nil
}

// OverrideCacheEntry is used for bulk-loading overrides into the cache.
type OverrideCacheEntry struct {
	ProjectKey     string
	EnvironmentKey string
	FlagKey        string
	AppUserID      string
	Value          json.RawMessage
	ExpiresAt      *time.Time
}
