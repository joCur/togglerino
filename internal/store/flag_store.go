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
// envOverrides optionally sets variants, fallthrough_variant, off_variant, and targeting_rules per environment.
func (s *FlagStore) Create(ctx context.Context, projectID, key, name, description string, valueType model.ValueType, flagType model.FlagType, defaultValue json.RawMessage, tags []string, envEnabled map[string]bool, ownerID *string, envOverrides map[string]model.EnvironmentDefault) (*model.Flag, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Compute flag-level variants from environment overrides (all envs should have the same variants).
	flagVariants := json.RawMessage(`[]`)
	if envOverrides != nil {
		for _, override := range envOverrides {
			if override.Variants != nil && string(override.Variants) != "[]" {
				flagVariants = override.Variants
				break
			}
		}
	}

	var f model.Flag
	var variantsJSON json.RawMessage
	err = tx.QueryRow(ctx,
		`INSERT INTO flags (project_id, key, name, description, value_type, flag_type, default_value, tags, owner_id, variants)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id, project_id, key, name, description, value_type, flag_type, default_value, tags, variants, lifecycle_status, lifecycle_status_changed_at, created_at, updated_at, owner_id, last_evaluated_at`,
		projectID, key, name, description, valueType, flagType, defaultValue, tags, ownerID, flagVariants,
	).Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &variantsJSON, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt, &f.OwnerID, &f.LastEvaluatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating flag: %w", err)
	}
	if err := json.Unmarshal(variantsJSON, &f.Variants); err != nil {
		return nil, fmt.Errorf("unmarshalling flag variants: %w", err)
	}
	if f.Variants == nil {
		f.Variants = []model.Variant{}
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

		fallthroughVariant := ""
		offVariant := ""
		targetingRules := json.RawMessage(`[]`)

		if envOverrides != nil {
			if override, ok := envOverrides[env.Key]; ok {
				if override.FallthroughVariant != "" {
					fallthroughVariant = override.FallthroughVariant
				}
				if override.OffVariant != "" {
					offVariant = override.OffVariant
				} else {
					offVariant = fallthroughVariant
				}
				if override.TargetingRules != nil {
					targetingRules = override.TargetingRules
				}
			}
		}

		_, err := tx.Exec(ctx,
			`INSERT INTO flag_environment_configs (flag_id, environment_id, enabled, fallthrough_variant, off_variant, targeting_rules) VALUES ($1, $2, $3, $4, $5, $6)`,
			f.ID, env.ID, enabled, fallthroughVariant, offVariant, targetingRules,
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
	query := `SELECT f.id, f.project_id, f.key, f.name, f.description, f.value_type, f.flag_type, f.default_value, f.tags, f.variants, f.lifecycle_status, f.lifecycle_status_changed_at, f.created_at, f.updated_at, f.owner_id, f.last_evaluated_at,
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
		var variantsJSON json.RawMessage
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &variantsJSON, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt, &f.OwnerID, &f.LastEvaluatedAt,
			&ownerUserID, &ownerEmail, &ownerDisplayName, &totalCount); err != nil {
			return nil, 0, fmt.Errorf("scanning flag: %w", err)
		}
		if err := json.Unmarshal(variantsJSON, &f.Variants); err != nil {
			return nil, 0, fmt.Errorf("unmarshalling variants: %w", err)
		}
		if ownerUserID != nil {
			f.Owner = &model.FlagOwner{ID: *ownerUserID, Email: *ownerEmail, DisplayName: ownerDisplayName}
		}
		if f.Tags == nil {
			f.Tags = []string{}
		}
		if f.Variants == nil {
			f.Variants = []model.Variant{}
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
	var variantsJSON json.RawMessage
	err := s.pool.QueryRow(ctx,
		`SELECT f.id, f.project_id, f.key, f.name, f.description, f.value_type, f.flag_type, f.default_value, f.tags, f.variants, f.lifecycle_status, f.lifecycle_status_changed_at, f.created_at, f.updated_at, f.owner_id, f.last_evaluated_at,
		       u.id, u.email, u.display_name
		 FROM flags f
		 LEFT JOIN users u ON f.owner_id = u.id
		 WHERE f.project_id = $1 AND f.key = $2`,
		projectID, key,
	).Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &variantsJSON, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt, &f.OwnerID, &f.LastEvaluatedAt,
		&ownerUserID, &ownerEmail, &ownerDisplayName)
	if err != nil {
		return nil, fmt.Errorf("finding flag by key: %w", err)
	}
	if err := json.Unmarshal(variantsJSON, &f.Variants); err != nil {
		return nil, fmt.Errorf("unmarshalling variants: %w", err)
	}
	if ownerUserID != nil {
		f.Owner = &model.FlagOwner{ID: *ownerUserID, Email: *ownerEmail, DisplayName: ownerDisplayName}
	}
	if f.Tags == nil {
		f.Tags = []string{}
	}
	if f.Variants == nil {
		f.Variants = []model.Variant{}
	}
	return &f, nil
}

// Update updates a flag's metadata (name, description, tags, flag_type, owner_id).
func (s *FlagStore) Update(ctx context.Context, flagID, name, description string, tags []string, flagType model.FlagType, ownerID *string) (*model.Flag, error) {
	var f model.Flag
	var variantsJSON json.RawMessage
	err := s.pool.QueryRow(ctx,
		`UPDATE flags SET name=$2, description=$3, tags=$4, flag_type=$5, owner_id=$6, updated_at=NOW() WHERE id=$1
		 RETURNING id, project_id, key, name, description, value_type, flag_type, default_value, tags, variants, lifecycle_status, lifecycle_status_changed_at, created_at, updated_at, owner_id, last_evaluated_at`,
		flagID, name, description, tags, flagType, ownerID,
	).Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &variantsJSON, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt, &f.OwnerID, &f.LastEvaluatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating flag: %w", err)
	}
	if err := json.Unmarshal(variantsJSON, &f.Variants); err != nil {
		return nil, fmt.Errorf("unmarshalling variants: %w", err)
	}
	if f.Tags == nil {
		f.Tags = []string{}
	}
	if f.Variants == nil {
		f.Variants = []model.Variant{}
	}
	return &f, nil
}

// SetLifecycleStatus sets the lifecycle status of a flag.
func (s *FlagStore) SetLifecycleStatus(ctx context.Context, flagID string, status model.LifecycleStatus) (*model.Flag, error) {
	var f model.Flag
	var variantsJSON json.RawMessage
	err := s.pool.QueryRow(ctx,
		`UPDATE flags SET lifecycle_status=$2, lifecycle_status_changed_at=NOW(), updated_at=NOW() WHERE id=$1
		 RETURNING id, project_id, key, name, description, value_type, flag_type, default_value, tags, variants, lifecycle_status, lifecycle_status_changed_at, created_at, updated_at, owner_id, last_evaluated_at`,
		flagID, status,
	).Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &variantsJSON, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt, &f.OwnerID, &f.LastEvaluatedAt)
	if err != nil {
		return nil, fmt.Errorf("setting flag lifecycle status: %w", err)
	}
	if err := json.Unmarshal(variantsJSON, &f.Variants); err != nil {
		return nil, fmt.Errorf("unmarshalling variants: %w", err)
	}
	if f.Tags == nil {
		f.Tags = []string{}
	}
	if f.Variants == nil {
		f.Variants = []model.Variant{}
	}
	return &f, nil
}

// ListNonArchived returns all flags that are not archived (for cache loading and staleness checks).
func (s *FlagStore) ListNonArchived(ctx context.Context) ([]model.Flag, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, key, name, description, value_type, flag_type, default_value, tags, variants, lifecycle_status, lifecycle_status_changed_at, created_at, updated_at, owner_id, last_evaluated_at
		 FROM flags WHERE lifecycle_status != 'archived'`)
	if err != nil {
		return nil, fmt.Errorf("listing non-archived flags: %w", err)
	}
	defer rows.Close()

	var flags []model.Flag
	for rows.Next() {
		var f model.Flag
		var variantsJSON json.RawMessage
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &variantsJSON, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt, &f.OwnerID, &f.LastEvaluatedAt); err != nil {
			return nil, fmt.Errorf("scanning flag: %w", err)
		}
		if err := json.Unmarshal(variantsJSON, &f.Variants); err != nil {
			return nil, fmt.Errorf("unmarshalling variants: %w", err)
		}
		if f.Tags == nil {
			f.Tags = []string{}
		}
		if f.Variants == nil {
			f.Variants = []model.Variant{}
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
		`SELECT fec.id, fec.flag_id, fec.environment_id, fec.enabled, fec.fallthrough_variant, fec.off_variant,
		        fec.targeting_rules, fec.updated_at, fec.updated_by,
		        fec.locked, fec.locked_by, fec.locked_at, fec.lock_reason,
		        u.id, u.email, u.display_name,
		        lu.id, lu.email, lu.display_name
		 FROM flag_environment_configs fec
		 LEFT JOIN users u ON fec.updated_by = u.id
		 LEFT JOIN users lu ON fec.locked_by = lu.id
		 WHERE fec.flag_id = $1 AND fec.environment_id = $2`,
		flagID, environmentID,
	)
	return scanFlagEnvConfigWithUser(row)
}

// GetAllEnvironmentConfigs returns all environment configs for a flag.
func (s *FlagStore) GetAllEnvironmentConfigs(ctx context.Context, flagID string) ([]model.FlagEnvironmentConfig, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT fec.id, fec.flag_id, fec.environment_id, fec.enabled, fec.fallthrough_variant, fec.off_variant,
		        fec.targeting_rules, fec.updated_at, fec.updated_by,
		        fec.locked, fec.locked_by, fec.locked_at, fec.lock_reason,
		        u.id, u.email, u.display_name,
		        lu.id, lu.email, lu.display_name
		 FROM flag_environment_configs fec
		 LEFT JOIN users u ON fec.updated_by = u.id
		 LEFT JOIN users lu ON fec.locked_by = lu.id
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
		`SELECT fec.id, fec.flag_id, fec.environment_id, fec.enabled, fec.fallthrough_variant, fec.off_variant,
		        fec.targeting_rules, fec.updated_at, fec.updated_by,
		        fec.locked, fec.locked_by, fec.locked_at, fec.lock_reason,
		        u.id, u.email, u.display_name,
		        lu.id, lu.email, lu.display_name
		 FROM flag_environment_configs fec
		 LEFT JOIN users u ON fec.updated_by = u.id
		 LEFT JOIN users lu ON fec.locked_by = lu.id
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
// This includes enabled, fallthrough_variant, off_variant, variants (JSON), and targeting_rules (JSON).
// updatedBy optionally records which user made the change.
func (s *FlagStore) UpdateEnvironmentConfig(ctx context.Context, flagID, environmentID string, enabled bool, fallthroughVariant, offVariant string, targetingRules json.RawMessage, updatedBy *string) (*model.FlagEnvironmentConfig, error) {
	row := s.pool.QueryRow(ctx,
		`UPDATE flag_environment_configs
		 SET enabled=$3, fallthrough_variant=$4, off_variant=$5, targeting_rules=$6, updated_at=NOW(), updated_by=$7
		 WHERE flag_id=$1 AND environment_id=$2
		 RETURNING id, flag_id, environment_id, enabled, fallthrough_variant, off_variant, targeting_rules, updated_at, updated_by,
		           locked, locked_by, locked_at, lock_reason`,
		flagID, environmentID, enabled, fallthroughVariant, offVariant, targetingRules, updatedBy,
	)
	return scanFlagEnvConfig(row)
}

// UpdateFlagVariants updates the variants for a flag (flag-level, not per-environment).
func (s *FlagStore) UpdateFlagVariants(ctx context.Context, flagID string, variants json.RawMessage) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE flags SET variants = $2, updated_at = NOW() WHERE id = $1`,
		flagID, variants,
	)
	if err != nil {
		return fmt.Errorf("updating flag variants: %w", err)
	}
	return nil
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

// LockEnvironmentConfig locks a flag's config in a specific environment.
func (s *FlagStore) LockEnvironmentConfig(ctx context.Context, flagID, environmentID, userID string, reason *string) (*model.FlagEnvironmentConfig, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE flag_environment_configs
		 SET locked = true, locked_by = $3, locked_at = NOW(), lock_reason = $4
		 WHERE flag_id = $1 AND environment_id = $2`,
		flagID, environmentID, userID, reason,
	)
	if err != nil {
		return nil, fmt.Errorf("locking environment config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return s.GetEnvironmentConfig(ctx, flagID, environmentID)
}

// UnlockEnvironmentConfig unlocks a flag's config in a specific environment.
func (s *FlagStore) UnlockEnvironmentConfig(ctx context.Context, flagID, environmentID string) (*model.FlagEnvironmentConfig, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE flag_environment_configs
		 SET locked = false, locked_by = NULL, locked_at = NULL, lock_reason = NULL
		 WHERE flag_id = $1 AND environment_id = $2`,
		flagID, environmentID,
	)
	if err != nil {
		return nil, fmt.Errorf("unlocking environment config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return s.GetEnvironmentConfig(ctx, flagID, environmentID)
}

// IsLockedInAnyEnvironment returns true if the flag is locked in any environment.
func (s *FlagStore) IsLockedInAnyEnvironment(ctx context.Context, flagID string) (bool, error) {
	var locked bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM flag_environment_configs
			WHERE flag_id = $1 AND locked = true
		)`,
		flagID,
	).Scan(&locked)
	if err != nil {
		return false, fmt.Errorf("checking lock status: %w", err)
	}
	return locked, nil
}

// IsEnvironmentConfigLocked returns true if a flag is locked in a specific environment.
func (s *FlagStore) IsEnvironmentConfigLocked(ctx context.Context, flagID, environmentID string) (bool, error) {
	var locked bool
	err := s.pool.QueryRow(ctx,
		`SELECT locked FROM flag_environment_configs WHERE flag_id = $1 AND environment_id = $2`,
		flagID, environmentID,
	).Scan(&locked)
	if err != nil {
		return false, fmt.Errorf("checking lock status: %w", err)
	}
	return locked, nil
}

func scanFlagEnvConfig(row pgx.Row) (*model.FlagEnvironmentConfig, error) {
	var cfg model.FlagEnvironmentConfig
	var rulesJSON json.RawMessage
	err := row.Scan(&cfg.ID, &cfg.FlagID, &cfg.EnvironmentID, &cfg.Enabled,
		&cfg.FallthroughVariant, &cfg.OffVariant, &rulesJSON, &cfg.UpdatedAt, &cfg.UpdatedBy,
		&cfg.Locked, &cfg.LockedBy, &cfg.LockedAt, &cfg.LockReason)
	if err != nil {
		return nil, fmt.Errorf("scanning flag environment config: %w", err)
	}
	if err := json.Unmarshal(rulesJSON, &cfg.TargetingRules); err != nil {
		return nil, fmt.Errorf("unmarshalling targeting rules: %w", err)
	}
	if cfg.TargetingRules == nil {
		cfg.TargetingRules = []model.TargetingRule{}
	}
	return &cfg, nil
}

func scanFlagEnvConfigWithUser(row pgx.Row) (*model.FlagEnvironmentConfig, error) {
	var cfg model.FlagEnvironmentConfig
	var rulesJSON json.RawMessage
	var updatedByUserID, updatedByEmail *string
	var updatedByDisplayName *string
	var lockedByUserID, lockedByEmail *string
	var lockedByDisplayName *string
	err := row.Scan(&cfg.ID, &cfg.FlagID, &cfg.EnvironmentID, &cfg.Enabled,
		&cfg.FallthroughVariant, &cfg.OffVariant, &rulesJSON, &cfg.UpdatedAt, &cfg.UpdatedBy,
		&cfg.Locked, &cfg.LockedBy, &cfg.LockedAt, &cfg.LockReason,
		&updatedByUserID, &updatedByEmail, &updatedByDisplayName,
		&lockedByUserID, &lockedByEmail, &lockedByDisplayName)
	if err != nil {
		return nil, fmt.Errorf("scanning flag environment config: %w", err)
	}
	if err := json.Unmarshal(rulesJSON, &cfg.TargetingRules); err != nil {
		return nil, fmt.Errorf("unmarshalling targeting rules: %w", err)
	}
	if cfg.TargetingRules == nil {
		cfg.TargetingRules = []model.TargetingRule{}
	}
	if updatedByUserID != nil {
		cfg.UpdatedByUser = &model.FlagOwner{ID: *updatedByUserID, Email: *updatedByEmail, DisplayName: updatedByDisplayName}
	}
	if lockedByUserID != nil {
		cfg.LockedByUser = &model.FlagOwner{ID: *lockedByUserID, Email: *lockedByEmail, DisplayName: lockedByDisplayName}
	}
	return &cfg, nil
}

// scanEnvironmentConfigRowWithUser scans a single row from a multi-row query
// that includes the LEFT JOIN on users for the updated_by field.
func scanEnvironmentConfigRowWithUser(rows pgx.Rows) (model.FlagEnvironmentConfig, error) {
	var cfg model.FlagEnvironmentConfig
	var rulesJSON json.RawMessage
	var updatedByUserID, updatedByEmail *string
	var updatedByDisplayName *string
	var lockedByUserID, lockedByEmail *string
	var lockedByDisplayName *string
	if err := rows.Scan(&cfg.ID, &cfg.FlagID, &cfg.EnvironmentID, &cfg.Enabled,
		&cfg.FallthroughVariant, &cfg.OffVariant, &rulesJSON, &cfg.UpdatedAt, &cfg.UpdatedBy,
		&cfg.Locked, &cfg.LockedBy, &cfg.LockedAt, &cfg.LockReason,
		&updatedByUserID, &updatedByEmail, &updatedByDisplayName,
		&lockedByUserID, &lockedByEmail, &lockedByDisplayName); err != nil {
		return cfg, fmt.Errorf("scanning environment config: %w", err)
	}
	if err := json.Unmarshal(rulesJSON, &cfg.TargetingRules); err != nil {
		return cfg, fmt.Errorf("unmarshalling targeting rules: %w", err)
	}
	if cfg.TargetingRules == nil {
		cfg.TargetingRules = []model.TargetingRule{}
	}
	if updatedByUserID != nil {
		cfg.UpdatedByUser = &model.FlagOwner{ID: *updatedByUserID, Email: *updatedByEmail, DisplayName: updatedByDisplayName}
	}
	if lockedByUserID != nil {
		cfg.LockedByUser = &model.FlagOwner{ID: *lockedByUserID, Email: *lockedByEmail, DisplayName: lockedByDisplayName}
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
