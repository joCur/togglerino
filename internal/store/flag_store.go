package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type FlagStore struct {
	pool *pgxpool.Pool
}

func NewFlagStore(pool *pgxpool.Pool) *FlagStore {
	return &FlagStore{pool: pool}
}

// Create inserts a new flag and creates a FlagEnvironmentConfig row for each
// environment in the project. The envEnabled map controls the initial enabled
// state per environment key; environments not in the map default to disabled.
func (s *FlagStore) Create(ctx context.Context, projectID, key, name, description string, valueType model.ValueType, flagType model.FlagType, defaultValue json.RawMessage, tags []string, envEnabled map[string]bool, ownerID *string) (*model.Flag, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var f model.Flag
	err = tx.QueryRow(ctx,
		`INSERT INTO flags (project_id, key, name, description, value_type, flag_type, default_value, tags, owner_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, project_id, key, name, description, value_type, flag_type, default_value, tags, lifecycle_status, lifecycle_status_changed_at, created_at, updated_at, owner_id`,
		projectID, key, name, description, valueType, flagType, defaultValue, tags, ownerID,
	).Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt, &f.OwnerID)
	if err != nil {
		return nil, fmt.Errorf("creating flag: %w", err)
	}

	// Get all environments for this project
	rows, err := tx.Query(ctx, `SELECT id, key FROM environments WHERE project_id = $1`, projectID)
	if err != nil {
		return nil, fmt.Errorf("querying environments: %w", err)
	}
	defer rows.Close()

	type envInfo struct {
		ID  string
		Key string
	}
	var envs []envInfo
	for rows.Next() {
		var e envInfo
		if err := rows.Scan(&e.ID, &e.Key); err != nil {
			return nil, fmt.Errorf("scanning environment: %w", err)
		}
		envs = append(envs, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating environments: %w", err)
	}

	// Create a FlagEnvironmentConfig for each environment
	for _, env := range envs {
		enabled := false
		if envEnabled != nil {
			if v, ok := envEnabled[env.Key]; ok {
				enabled = v
			}
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO flag_environment_configs (flag_id, environment_id, enabled) VALUES ($1, $2, $3)`,
			f.ID, env.ID, enabled,
		)
		if err != nil {
			return nil, fmt.Errorf("creating flag environment config for env %s: %w", env.ID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	if f.Tags == nil {
		f.Tags = []string{}
	}

	return &f, nil
}

// ListByProject returns all flags for a project. Supports optional tag filter, search query,
// lifecycle status filter, flag type filter, and owner filter.
func (s *FlagStore) ListByProject(ctx context.Context, projectID string, tag string, search string, lifecycleStatus string, flagType string, owner string) ([]model.Flag, error) {
	query := `SELECT f.id, f.project_id, f.key, f.name, f.description, f.value_type, f.flag_type, f.default_value, f.tags, f.lifecycle_status, f.lifecycle_status_changed_at, f.created_at, f.updated_at, f.owner_id,
	       u.id, u.email, u.display_name
		FROM flags f
		LEFT JOIN users u ON f.owner_id = u.id
		WHERE f.project_id = $1`
	args := []any{projectID}
	argIdx := 2

	if tag != "" {
		query += fmt.Sprintf(" AND $%d = ANY(f.tags)", argIdx)
		args = append(args, tag)
		argIdx++
	}

	if search != "" {
		query += fmt.Sprintf(" AND (f.key ILIKE '%%' || $%d || '%%' OR f.name ILIKE '%%' || $%d || '%%')", argIdx, argIdx)
		args = append(args, search)
		argIdx++
	}

	if lifecycleStatus != "" {
		values := strings.Split(lifecycleStatus, ",")
		query += fmt.Sprintf(" AND f.lifecycle_status = ANY($%d)", argIdx)
		args = append(args, values)
		argIdx++
	}

	if flagType != "" {
		values := strings.Split(flagType, ",")
		query += fmt.Sprintf(" AND f.flag_type = ANY($%d)", argIdx)
		args = append(args, values)
		argIdx++
	}

	if owner != "" {
		query += fmt.Sprintf(" AND f.owner_id = $%d", argIdx)
		args = append(args, owner)
		argIdx++
	}

	query += " ORDER BY f.created_at DESC"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing flags: %w", err)
	}
	defer rows.Close()

	var flags []model.Flag
	for rows.Next() {
		var f model.Flag
		var ownerUserID, ownerEmail *string
		var ownerDisplayName *string
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt, &f.OwnerID,
			&ownerUserID, &ownerEmail, &ownerDisplayName); err != nil {
			return nil, fmt.Errorf("scanning flag: %w", err)
		}
		if ownerUserID != nil {
			f.Owner = &model.FlagOwner{ID: *ownerUserID, Email: *ownerEmail, DisplayName: ownerDisplayName}
		}
		if f.Tags == nil {
			f.Tags = []string{}
		}
		flags = append(flags, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating flags: %w", err)
	}
	return flags, nil
}

// FindByKey returns a flag by project ID and flag key.
func (s *FlagStore) FindByKey(ctx context.Context, projectID, key string) (*model.Flag, error) {
	var f model.Flag
	var ownerUserID, ownerEmail *string
	var ownerDisplayName *string
	err := s.pool.QueryRow(ctx,
		`SELECT f.id, f.project_id, f.key, f.name, f.description, f.value_type, f.flag_type, f.default_value, f.tags, f.lifecycle_status, f.lifecycle_status_changed_at, f.created_at, f.updated_at, f.owner_id,
		       u.id, u.email, u.display_name
		 FROM flags f
		 LEFT JOIN users u ON f.owner_id = u.id
		 WHERE f.project_id = $1 AND f.key = $2`,
		projectID, key,
	).Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt, &f.OwnerID,
		&ownerUserID, &ownerEmail, &ownerDisplayName)
	if err != nil {
		return nil, fmt.Errorf("finding flag by key: %w", err)
	}
	if ownerUserID != nil {
		f.Owner = &model.FlagOwner{ID: *ownerUserID, Email: *ownerEmail, DisplayName: ownerDisplayName}
	}
	if f.Tags == nil {
		f.Tags = []string{}
	}
	return &f, nil
}

// Update updates a flag's metadata (name, description, tags, flag_type, owner_id).
func (s *FlagStore) Update(ctx context.Context, flagID, name, description string, tags []string, flagType model.FlagType, ownerID *string) (*model.Flag, error) {
	var f model.Flag
	err := s.pool.QueryRow(ctx,
		`UPDATE flags SET name=$2, description=$3, tags=$4, flag_type=$5, owner_id=$6, updated_at=NOW() WHERE id=$1
		 RETURNING id, project_id, key, name, description, value_type, flag_type, default_value, tags, lifecycle_status, lifecycle_status_changed_at, created_at, updated_at, owner_id`,
		flagID, name, description, tags, flagType, ownerID,
	).Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt, &f.OwnerID)
	if err != nil {
		return nil, fmt.Errorf("updating flag: %w", err)
	}
	if f.Tags == nil {
		f.Tags = []string{}
	}
	return &f, nil
}

// SetLifecycleStatus sets the lifecycle status of a flag.
func (s *FlagStore) SetLifecycleStatus(ctx context.Context, flagID string, status model.LifecycleStatus) (*model.Flag, error) {
	var f model.Flag
	err := s.pool.QueryRow(ctx,
		`UPDATE flags SET lifecycle_status=$2, lifecycle_status_changed_at=NOW(), updated_at=NOW() WHERE id=$1
		 RETURNING id, project_id, key, name, description, value_type, flag_type, default_value, tags, lifecycle_status, lifecycle_status_changed_at, created_at, updated_at, owner_id`,
		flagID, status,
	).Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt, &f.OwnerID)
	if err != nil {
		return nil, fmt.Errorf("setting flag lifecycle status: %w", err)
	}
	if f.Tags == nil {
		f.Tags = []string{}
	}
	return &f, nil
}

// ListNonArchived returns all flags that are not archived (for cache loading and staleness checks).
func (s *FlagStore) ListNonArchived(ctx context.Context) ([]model.Flag, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, key, name, description, value_type, flag_type, default_value, tags, lifecycle_status, lifecycle_status_changed_at, created_at, updated_at, owner_id
		 FROM flags WHERE lifecycle_status != 'archived'`)
	if err != nil {
		return nil, fmt.Errorf("listing non-archived flags: %w", err)
	}
	defer rows.Close()

	var flags []model.Flag
	for rows.Next() {
		var f model.Flag
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt, &f.OwnerID); err != nil {
			return nil, fmt.Errorf("scanning flag: %w", err)
		}
		if f.Tags == nil {
			f.Tags = []string{}
		}
		flags = append(flags, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating flags: %w", err)
	}
	return flags, nil
}

// Delete deletes a flag by ID (cascades to environment configs).
func (s *FlagStore) Delete(ctx context.Context, flagID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM flags WHERE id = $1`, flagID)
	if err != nil {
		return fmt.Errorf("deleting flag: %w", err)
	}
	return nil
}

// GetEnvironmentConfig returns the flag config for a specific environment.
func (s *FlagStore) GetEnvironmentConfig(ctx context.Context, flagID, environmentID string) (*model.FlagEnvironmentConfig, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, flag_id, environment_id, enabled, default_variant, variants, targeting_rules, updated_at
		 FROM flag_environment_configs WHERE flag_id = $1 AND environment_id = $2`,
		flagID, environmentID,
	)
	return scanFlagEnvConfig(row)
}

// GetAllEnvironmentConfigs returns all environment configs for a flag.
func (s *FlagStore) GetAllEnvironmentConfigs(ctx context.Context, flagID string) ([]model.FlagEnvironmentConfig, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, flag_id, environment_id, enabled, default_variant, variants, targeting_rules, updated_at
		 FROM flag_environment_configs WHERE flag_id = $1 ORDER BY updated_at`,
		flagID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing environment configs: %w", err)
	}
	defer rows.Close()

	var configs []model.FlagEnvironmentConfig
	for rows.Next() {
		var cfg model.FlagEnvironmentConfig
		var variantsJSON, rulesJSON json.RawMessage
		if err := rows.Scan(&cfg.ID, &cfg.FlagID, &cfg.EnvironmentID, &cfg.Enabled,
			&cfg.DefaultVariant, &variantsJSON, &rulesJSON, &cfg.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning environment config: %w", err)
		}
		json.Unmarshal(variantsJSON, &cfg.Variants)
		json.Unmarshal(rulesJSON, &cfg.TargetingRules)
		if cfg.Variants == nil {
			cfg.Variants = []model.Variant{}
		}
		if cfg.TargetingRules == nil {
			cfg.TargetingRules = []model.TargetingRule{}
		}
		configs = append(configs, cfg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating environment configs: %w", err)
	}
	return configs, nil
}

// UpdateEnvironmentConfig updates the flag config for a specific environment.
// This includes enabled, default_variant, variants (JSON), and targeting_rules (JSON).
func (s *FlagStore) UpdateEnvironmentConfig(ctx context.Context, flagID, environmentID string, enabled bool, defaultVariant string, variants json.RawMessage, targetingRules json.RawMessage) (*model.FlagEnvironmentConfig, error) {
	row := s.pool.QueryRow(ctx,
		`UPDATE flag_environment_configs
		 SET enabled=$3, default_variant=$4, variants=$5, targeting_rules=$6, updated_at=NOW()
		 WHERE flag_id=$1 AND environment_id=$2
		 RETURNING id, flag_id, environment_id, enabled, default_variant, variants, targeting_rules, updated_at`,
		flagID, environmentID, enabled, defaultVariant, variants, targetingRules,
	)
	return scanFlagEnvConfig(row)
}

// ProjectKeyByFlagID returns the project key for a flag (used by schedule checker).
func (s *FlagStore) ProjectKeyByFlagID(ctx context.Context, flagID string) (string, error) {
	var key string
	err := s.pool.QueryRow(ctx,
		`SELECT p.key FROM projects p JOIN flags f ON f.project_id = p.id WHERE f.id = $1`,
		flagID,
	).Scan(&key)
	if err != nil {
		return "", fmt.Errorf("looking up project key by flag ID: %w", err)
	}
	return key, nil
}

// ProjectIDByFlagID returns the project ID for a flag (used by schedule checker).
func (s *FlagStore) ProjectIDByFlagID(ctx context.Context, flagID string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`SELECT project_id FROM flags WHERE id = $1`,
		flagID,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("looking up project ID by flag ID: %w", err)
	}
	return id, nil
}

// FlagKeyByID returns the flag key for a flag ID (used by schedule checker).
func (s *FlagStore) FlagKeyByID(ctx context.Context, flagID string) (string, error) {
	var key string
	err := s.pool.QueryRow(ctx,
		`SELECT key FROM flags WHERE id = $1`,
		flagID,
	).Scan(&key)
	if err != nil {
		return "", fmt.Errorf("looking up flag key by ID: %w", err)
	}
	return key, nil
}

func scanFlagEnvConfig(row pgx.Row) (*model.FlagEnvironmentConfig, error) {
	var cfg model.FlagEnvironmentConfig
	var variantsJSON, rulesJSON json.RawMessage
	err := row.Scan(&cfg.ID, &cfg.FlagID, &cfg.EnvironmentID, &cfg.Enabled,
		&cfg.DefaultVariant, &variantsJSON, &rulesJSON, &cfg.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("scanning flag environment config: %w", err)
	}
	json.Unmarshal(variantsJSON, &cfg.Variants)
	json.Unmarshal(rulesJSON, &cfg.TargetingRules)
	if cfg.Variants == nil {
		cfg.Variants = []model.Variant{}
	}
	if cfg.TargetingRules == nil {
		cfg.TargetingRules = []model.TargetingRule{}
	}
	return &cfg, nil
}
