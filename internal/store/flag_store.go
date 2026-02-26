package store

import (
	"context"
	"encoding/json"
	"fmt"

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
// environment in the project (all disabled by default with default variants).
func (s *FlagStore) Create(ctx context.Context, projectID, key, name, description string, flagType model.FlagType, defaultValue json.RawMessage, tags []string) (*model.Flag, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var f model.Flag
	err = tx.QueryRow(ctx,
		`INSERT INTO flags (project_id, key, name, description, flag_type, default_value, tags)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, project_id, key, name, description, flag_type, default_value, tags, archived, created_at, updated_at`,
		projectID, key, name, description, flagType, defaultValue, tags,
	).Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.FlagType, &f.DefaultValue, &f.Tags, &f.Archived, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating flag: %w", err)
	}

	// Get all environments for this project
	rows, err := tx.Query(ctx, `SELECT id FROM environments WHERE project_id = $1`, projectID)
	if err != nil {
		return nil, fmt.Errorf("querying environments: %w", err)
	}
	defer rows.Close()

	var envIDs []string
	for rows.Next() {
		var envID string
		if err := rows.Scan(&envID); err != nil {
			return nil, fmt.Errorf("scanning environment id: %w", err)
		}
		envIDs = append(envIDs, envID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating environments: %w", err)
	}

	// Create a FlagEnvironmentConfig for each environment
	for _, envID := range envIDs {
		_, err := tx.Exec(ctx,
			`INSERT INTO flag_environment_configs (flag_id, environment_id) VALUES ($1, $2)`,
			f.ID, envID,
		)
		if err != nil {
			return nil, fmt.Errorf("creating flag environment config for env %s: %w", envID, err)
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

// ListByProject returns all flags for a project. Supports optional tag filter and search query.
func (s *FlagStore) ListByProject(ctx context.Context, projectID string, tag string, search string) ([]model.Flag, error) {
	query := `SELECT id, project_id, key, name, description, flag_type, default_value, tags, archived, created_at, updated_at
		FROM flags WHERE project_id = $1`
	args := []any{projectID}
	argIdx := 2

	if tag != "" {
		query += fmt.Sprintf(" AND $%d = ANY(tags)", argIdx)
		args = append(args, tag)
		argIdx++
	}

	if search != "" {
		query += fmt.Sprintf(" AND (key ILIKE '%%' || $%d || '%%' OR name ILIKE '%%' || $%d || '%%')", argIdx, argIdx)
		args = append(args, search)
		argIdx++
	}

	query += " ORDER BY created_at DESC"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing flags: %w", err)
	}
	defer rows.Close()

	var flags []model.Flag
	for rows.Next() {
		var f model.Flag
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.FlagType, &f.DefaultValue, &f.Tags, &f.Archived, &f.CreatedAt, &f.UpdatedAt); err != nil {
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

// FindByKey returns a flag by project ID and flag key.
func (s *FlagStore) FindByKey(ctx context.Context, projectID, key string) (*model.Flag, error) {
	var f model.Flag
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, key, name, description, flag_type, default_value, tags, archived, created_at, updated_at
		 FROM flags WHERE project_id = $1 AND key = $2`,
		projectID, key,
	).Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.FlagType, &f.DefaultValue, &f.Tags, &f.Archived, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("finding flag by key: %w", err)
	}
	if f.Tags == nil {
		f.Tags = []string{}
	}
	return &f, nil
}

// Update updates a flag's metadata (name, description, tags).
func (s *FlagStore) Update(ctx context.Context, flagID, name, description string, tags []string) (*model.Flag, error) {
	var f model.Flag
	err := s.pool.QueryRow(ctx,
		`UPDATE flags SET name=$2, description=$3, tags=$4, updated_at=NOW() WHERE id=$1
		 RETURNING id, project_id, key, name, description, flag_type, default_value, tags, archived, created_at, updated_at`,
		flagID, name, description, tags,
	).Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.FlagType, &f.DefaultValue, &f.Tags, &f.Archived, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating flag: %w", err)
	}
	if f.Tags == nil {
		f.Tags = []string{}
	}
	return &f, nil
}

// SetArchived sets the archived status of a flag.
func (s *FlagStore) SetArchived(ctx context.Context, flagID string, archived bool) (*model.Flag, error) {
	var f model.Flag
	err := s.pool.QueryRow(ctx,
		`UPDATE flags SET archived=$2, updated_at=NOW() WHERE id=$1
		 RETURNING id, project_id, key, name, description, flag_type, default_value, tags, archived, created_at, updated_at`,
		flagID, archived,
	).Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.FlagType, &f.DefaultValue, &f.Tags, &f.Archived, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("setting flag archived status: %w", err)
	}
	if f.Tags == nil {
		f.Tags = []string{}
	}
	return &f, nil
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
