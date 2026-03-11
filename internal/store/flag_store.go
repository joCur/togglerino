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
// envOverrides optionally sets variants, default_variant, and targeting_rules per environment.
func (s *FlagStore) Create(ctx context.Context, projectID, key, name, description string, valueType model.ValueType, flagType model.FlagType, defaultValue json.RawMessage, tags []string, envEnabled map[string]bool, ownerID *string, envOverrides map[string]model.EnvironmentDefault) (*model.Flag, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var f model.Flag
	err = tx.QueryRow(ctx,
		`INSERT INTO flags (project_id, key, name, description, value_type, flag_type, default_value, tags, owner_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, project_id, key, name, description, value_type, flag_type, default_value, tags, lifecycle_status, lifecycle_status_changed_at, created_at, updated_at, owner_id, last_evaluated_at`,
		projectID, key, name, description, valueType, flagType, defaultValue, tags, ownerID,
	).Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt, &f.OwnerID, &f.LastEvaluatedAt)
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

		defaultVariant := ""
		variants := json.RawMessage(`[]`)
		targetingRules := json.RawMessage(`[]`)

		if envOverrides != nil {
			if override, ok := envOverrides[env.Key]; ok {
				if override.DefaultVariant != "" {
					defaultVariant = override.DefaultVariant
				}
				if override.Variants != nil {
					variants = override.Variants
				}
				if override.TargetingRules != nil {
					targetingRules = override.TargetingRules
				}
			}
		}

		_, err := tx.Exec(ctx,
			`INSERT INTO flag_environment_configs (flag_id, environment_id, enabled, default_variant, variants, targeting_rules) VALUES ($1, $2, $3, $4, $5, $6)`,
			f.ID, env.ID, enabled, defaultVariant, variants, targetingRules,
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

// ListByProject returns flags for a project with pagination. Supports optional tag filter,
// search query, lifecycle status filter, flag type filter, owner filter, and unevaluated days filter.
// Returns the flags, total count (before pagination), and any error.
func (s *FlagStore) ListByProject(ctx context.Context, projectID string, tag string, search string, lifecycleStatus string, flagType string, owner string, unevaluatedDays string, limit, offset int) ([]model.Flag, int, error) {
	query := `SELECT f.id, f.project_id, f.key, f.name, f.description, f.value_type, f.flag_type, f.default_value, f.tags, f.lifecycle_status, f.lifecycle_status_changed_at, f.created_at, f.updated_at, f.owner_id, f.last_evaluated_at,
	       u.id, u.email, u.display_name,
	       COUNT(*) OVER() AS total_count
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

	if unevaluatedDays != "" {
		if unevaluatedDays == "never" {
			query += " AND f.last_evaluated_at IS NULL"
		} else {
			query += fmt.Sprintf(" AND (f.last_evaluated_at IS NULL OR f.last_evaluated_at < NOW() - ($%d || ' days')::INTERVAL)", argIdx)
			args = append(args, unevaluatedDays)
			argIdx++
		}
	}

	query += fmt.Sprintf(" ORDER BY f.created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing flags: %w", err)
	}
	defer rows.Close()

	var flags []model.Flag
	totalCount := 0
	for rows.Next() {
		var f model.Flag
		var ownerUserID, ownerEmail *string
		var ownerDisplayName *string
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt, &f.OwnerID, &f.LastEvaluatedAt,
			&ownerUserID, &ownerEmail, &ownerDisplayName, &totalCount); err != nil {
			return nil, 0, fmt.Errorf("scanning flag: %w", err)
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
		return nil, 0, fmt.Errorf("iterating flags: %w", err)
	}
	return flags, totalCount, nil
}

// ListAllByProject returns all flags for a project without pagination.
// Used by callers that need the complete set.
func (s *FlagStore) ListAllByProject(ctx context.Context, projectID string, tag string, search string, lifecycleStatus string, flagType string, owner string, unevaluatedDays string) ([]model.Flag, error) {
	// Use a very high limit to get all results
	flags, _, err := s.ListByProject(ctx, projectID, tag, search, lifecycleStatus, flagType, owner, unevaluatedDays, 2147483647, 0)
	if err != nil {
		return nil, err
	}
	return flags, nil
}

// FindByKey returns a flag by project ID and flag key.
func (s *FlagStore) FindByKey(ctx context.Context, projectID, key string) (*model.Flag, error) {
	var f model.Flag
	var ownerUserID, ownerEmail *string
	var ownerDisplayName *string
	err := s.pool.QueryRow(ctx,
		`SELECT f.id, f.project_id, f.key, f.name, f.description, f.value_type, f.flag_type, f.default_value, f.tags, f.lifecycle_status, f.lifecycle_status_changed_at, f.created_at, f.updated_at, f.owner_id, f.last_evaluated_at,
		       u.id, u.email, u.display_name
		 FROM flags f
		 LEFT JOIN users u ON f.owner_id = u.id
		 WHERE f.project_id = $1 AND f.key = $2`,
		projectID, key,
	).Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt, &f.OwnerID, &f.LastEvaluatedAt,
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
		 RETURNING id, project_id, key, name, description, value_type, flag_type, default_value, tags, lifecycle_status, lifecycle_status_changed_at, created_at, updated_at, owner_id, last_evaluated_at`,
		flagID, name, description, tags, flagType, ownerID,
	).Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt, &f.OwnerID, &f.LastEvaluatedAt)
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
		 RETURNING id, project_id, key, name, description, value_type, flag_type, default_value, tags, lifecycle_status, lifecycle_status_changed_at, created_at, updated_at, owner_id, last_evaluated_at`,
		flagID, status,
	).Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt, &f.OwnerID, &f.LastEvaluatedAt)
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
		`SELECT id, project_id, key, name, description, value_type, flag_type, default_value, tags, lifecycle_status, lifecycle_status_changed_at, created_at, updated_at, owner_id, last_evaluated_at
		 FROM flags WHERE lifecycle_status != 'archived'`)
	if err != nil {
		return nil, fmt.Errorf("listing non-archived flags: %w", err)
	}
	defer rows.Close()

	var flags []model.Flag
	for rows.Next() {
		var f model.Flag
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt, &f.OwnerID, &f.LastEvaluatedAt); err != nil {
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

// UpdateLastEvaluatedAt batch-updates the last_evaluated_at timestamp for the given flag IDs.
func (s *FlagStore) UpdateLastEvaluatedAt(ctx context.Context, flagIDs []string) error {
	if len(flagIDs) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE flags SET last_evaluated_at = NOW() WHERE id = ANY($1)`,
		flagIDs,
	)
	if err != nil {
		return fmt.Errorf("updating last_evaluated_at: %w", err)
	}
	return nil
}

// LifecycleCountsByProject returns flag counts grouped by project and lifecycle
// status using a single aggregate query. Projects with zero flags are included
// via a LEFT JOIN to the projects table — they produce a single row with
// COUNT(f.id) = 0. The COALESCE maps the NULL lifecycle_status to 'active' for
// these zero-flag rows; the status value is irrelevant since the count is 0.
func (s *FlagStore) LifecycleCountsByProject(ctx context.Context) ([]model.LifecycleCountRow, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT p.id, COALESCE(f.lifecycle_status, 'active'), COUNT(f.id)
		 FROM projects p
		 LEFT JOIN flags f ON f.project_id = p.id
		 GROUP BY p.id, f.lifecycle_status`)
	if err != nil {
		return nil, fmt.Errorf("querying lifecycle counts: %w", err)
	}
	defer rows.Close()

	var result []model.LifecycleCountRow
	for rows.Next() {
		var r model.LifecycleCountRow
		if err := rows.Scan(&r.ProjectID, &r.Status, &r.Count); err != nil {
			return nil, fmt.Errorf("scanning lifecycle count row: %w", err)
		}
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating lifecycle counts: %w", err)
	}
	return result, nil
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
		`SELECT fec.id, fec.flag_id, fec.environment_id, fec.enabled, fec.default_variant,
		        fec.variants, fec.targeting_rules, fec.updated_at, fec.updated_by,
		        u.id, u.email, u.display_name
		 FROM flag_environment_configs fec
		 LEFT JOIN users u ON fec.updated_by = u.id
		 WHERE fec.flag_id = $1 AND fec.environment_id = $2`,
		flagID, environmentID,
	)
	return scanFlagEnvConfigWithUser(row)
}

// GetAllEnvironmentConfigs returns all environment configs for a flag.
func (s *FlagStore) GetAllEnvironmentConfigs(ctx context.Context, flagID string) ([]model.FlagEnvironmentConfig, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT fec.id, fec.flag_id, fec.environment_id, fec.enabled, fec.default_variant,
		        fec.variants, fec.targeting_rules, fec.updated_at, fec.updated_by,
		        u.id, u.email, u.display_name
		 FROM flag_environment_configs fec
		 LEFT JOIN users u ON fec.updated_by = u.id
		 WHERE fec.flag_id = $1 ORDER BY fec.updated_at`,
		flagID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing environment configs: %w", err)
	}
	defer rows.Close()

	var configs []model.FlagEnvironmentConfig
	for rows.Next() {
		cfg, err := scanEnvironmentConfigRowWithUser(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, cfg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating environment configs: %w", err)
	}
	return configs, nil
}

// GetEnvironmentConfigsByFlagIDs returns environment configs for multiple flags in a single query.
// The returned map is keyed by flag ID.
func (s *FlagStore) GetEnvironmentConfigsByFlagIDs(ctx context.Context, flagIDs []string) (map[string][]model.FlagEnvironmentConfig, error) {
	result := make(map[string][]model.FlagEnvironmentConfig)
	if len(flagIDs) == 0 {
		return result, nil
	}

	rows, err := s.pool.Query(ctx,
		`SELECT fec.id, fec.flag_id, fec.environment_id, fec.enabled, fec.default_variant,
		        fec.variants, fec.targeting_rules, fec.updated_at, fec.updated_by,
		        u.id, u.email, u.display_name
		 FROM flag_environment_configs fec
		 LEFT JOIN users u ON fec.updated_by = u.id
		 WHERE fec.flag_id = ANY($1)
		 ORDER BY fec.flag_id, fec.updated_at`,
		flagIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("querying environment configs by flag IDs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		cfg, err := scanEnvironmentConfigRowWithUser(rows)
		if err != nil {
			return nil, err
		}
		result[cfg.FlagID] = append(result[cfg.FlagID], cfg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating environment configs: %w", err)
	}
	return result, nil
}

// UpdateEnvironmentConfig updates the flag config for a specific environment.
// This includes enabled, default_variant, variants (JSON), and targeting_rules (JSON).
// updatedBy optionally records which user made the change.
func (s *FlagStore) UpdateEnvironmentConfig(ctx context.Context, flagID, environmentID string, enabled bool, defaultVariant string, variants json.RawMessage, targetingRules json.RawMessage, updatedBy *string) (*model.FlagEnvironmentConfig, error) {
	row := s.pool.QueryRow(ctx,
		`UPDATE flag_environment_configs
		 SET enabled=$3, default_variant=$4, variants=$5, targeting_rules=$6, updated_at=NOW(), updated_by=$7
		 WHERE flag_id=$1 AND environment_id=$2
		 RETURNING id, flag_id, environment_id, enabled, default_variant, variants, targeting_rules, updated_at, updated_by`,
		flagID, environmentID, enabled, defaultVariant, variants, targetingRules, updatedBy,
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
		&cfg.DefaultVariant, &variantsJSON, &rulesJSON, &cfg.UpdatedAt, &cfg.UpdatedBy)
	if err != nil {
		return nil, fmt.Errorf("scanning flag environment config: %w", err)
	}
	if err := json.Unmarshal(variantsJSON, &cfg.Variants); err != nil {
		return nil, fmt.Errorf("unmarshalling variants: %w", err)
	}
	if err := json.Unmarshal(rulesJSON, &cfg.TargetingRules); err != nil {
		return nil, fmt.Errorf("unmarshalling targeting rules: %w", err)
	}
	if cfg.Variants == nil {
		cfg.Variants = []model.Variant{}
	}
	if cfg.TargetingRules == nil {
		cfg.TargetingRules = []model.TargetingRule{}
	}
	return &cfg, nil
}

func scanFlagEnvConfigWithUser(row pgx.Row) (*model.FlagEnvironmentConfig, error) {
	var cfg model.FlagEnvironmentConfig
	var variantsJSON, rulesJSON json.RawMessage
	var updatedByUserID, updatedByEmail *string
	var updatedByDisplayName *string
	err := row.Scan(&cfg.ID, &cfg.FlagID, &cfg.EnvironmentID, &cfg.Enabled,
		&cfg.DefaultVariant, &variantsJSON, &rulesJSON, &cfg.UpdatedAt, &cfg.UpdatedBy,
		&updatedByUserID, &updatedByEmail, &updatedByDisplayName)
	if err != nil {
		return nil, fmt.Errorf("scanning flag environment config: %w", err)
	}
	if err := json.Unmarshal(variantsJSON, &cfg.Variants); err != nil {
		return nil, fmt.Errorf("unmarshalling variants: %w", err)
	}
	if err := json.Unmarshal(rulesJSON, &cfg.TargetingRules); err != nil {
		return nil, fmt.Errorf("unmarshalling targeting rules: %w", err)
	}
	if cfg.Variants == nil {
		cfg.Variants = []model.Variant{}
	}
	if cfg.TargetingRules == nil {
		cfg.TargetingRules = []model.TargetingRule{}
	}
	if updatedByUserID != nil {
		cfg.UpdatedByUser = &model.FlagOwner{ID: *updatedByUserID, Email: *updatedByEmail, DisplayName: updatedByDisplayName}
	}
	return &cfg, nil
}

// scanEnvironmentConfigRowWithUser scans a single row from a multi-row query
// that includes the LEFT JOIN on users for the updated_by field.
func scanEnvironmentConfigRowWithUser(rows pgx.Rows) (model.FlagEnvironmentConfig, error) {
	var cfg model.FlagEnvironmentConfig
	var variantsJSON, rulesJSON json.RawMessage
	var updatedByUserID, updatedByEmail *string
	var updatedByDisplayName *string
	if err := rows.Scan(&cfg.ID, &cfg.FlagID, &cfg.EnvironmentID, &cfg.Enabled,
		&cfg.DefaultVariant, &variantsJSON, &rulesJSON, &cfg.UpdatedAt, &cfg.UpdatedBy,
		&updatedByUserID, &updatedByEmail, &updatedByDisplayName); err != nil {
		return cfg, fmt.Errorf("scanning environment config: %w", err)
	}
	if err := json.Unmarshal(variantsJSON, &cfg.Variants); err != nil {
		return cfg, fmt.Errorf("unmarshalling variants: %w", err)
	}
	if err := json.Unmarshal(rulesJSON, &cfg.TargetingRules); err != nil {
		return cfg, fmt.Errorf("unmarshalling targeting rules: %w", err)
	}
	if cfg.Variants == nil {
		cfg.Variants = []model.Variant{}
	}
	if cfg.TargetingRules == nil {
		cfg.TargetingRules = []model.TargetingRule{}
	}
	if updatedByUserID != nil {
		cfg.UpdatedByUser = &model.FlagOwner{ID: *updatedByUserID, Email: *updatedByEmail, DisplayName: updatedByDisplayName}
	}
	return cfg, nil
}

// LifecycleSummary returns flag counts grouped by lifecycle status for a
// project, plus a health score (percentage of non-archived flags that are
// active).
func (s *FlagStore) LifecycleSummary(ctx context.Context, projectID string) (*model.LifecycleSummary, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT lifecycle_status, COUNT(*)
		 FROM flags WHERE project_id = $1
		 GROUP BY lifecycle_status`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying lifecycle summary: %w", err)
	}
	defer rows.Close()

	summary := &model.LifecycleSummary{}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scanning lifecycle count: %w", err)
		}
		switch status {
		case "active":
			summary.Active = count
		case "potentially_stale":
			summary.PotentiallyStale = count
		case "stale":
			summary.Stale = count
		case "archived":
			summary.Archived = count
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating lifecycle counts: %w", err)
	}

	nonArchived := summary.Active + summary.PotentiallyStale + summary.Stale
	if nonArchived > 0 {
		summary.HealthScore = float64(summary.Active) / float64(nonArchived) * 100
	} else {
		summary.HealthScore = 100
	}

	return summary, nil
}
