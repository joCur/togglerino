package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

// FlagData holds everything needed to evaluate a flag.
type FlagData struct {
	Flag   model.Flag
	Config model.FlagEnvironmentConfig
}

// Cache holds all flag data in memory for fast evaluation.
type Cache struct {
	mu sync.RWMutex
	// Key: "projectKey:envKey", Value: map of flagKey -> FlagData
	data map[string]map[string]FlagData
}

// NewCache creates a new empty cache.
func NewCache() *Cache {
	return &Cache{
		data: make(map[string]map[string]FlagData),
	}
}

// cacheKey builds the composite key for the cache map.
func cacheKey(projectKey, envKey string) string {
	return projectKey + ":" + envKey
}

const baseFlagQuery = `
SELECT
    p.key AS project_key,
    e.key AS env_key,
    f.id, f.project_id, f.key, f.name, f.description, f.value_type, f.flag_type, f.default_value, f.tags, f.lifecycle_status, f.lifecycle_status_changed_at, f.created_at, f.updated_at,
    fec.id, fec.flag_id, fec.environment_id, fec.enabled, fec.default_variant, fec.variants, fec.targeting_rules, fec.updated_at
FROM flags f
JOIN projects p ON p.id = f.project_id
JOIN flag_environment_configs fec ON fec.flag_id = f.id
JOIN environments e ON e.id = fec.environment_id
`

// LoadAll loads all flags and their environment configs from the database.
// Called once on startup.
func (c *Cache) LoadAll(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := pool.Query(ctx, baseFlagQuery)
	if err != nil {
		return fmt.Errorf("cache LoadAll query: %w", err)
	}
	defer rows.Close()

	newData := make(map[string]map[string]FlagData)

	for rows.Next() {
		projectKey, envKey, fd, err := scanFlagRow(rows)
		if err != nil {
			return fmt.Errorf("cache LoadAll scan: %w", err)
		}
		key := cacheKey(projectKey, envKey)
		if newData[key] == nil {
			newData[key] = make(map[string]FlagData)
		}
		newData[key][fd.Flag.Key] = fd
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("cache LoadAll rows: %w", err)
	}

	c.mu.Lock()
	c.data = newData
	c.mu.Unlock()

	return nil
}

// Refresh reloads flag data for a specific project/environment from the database.
// Called after a flag is updated.
func (c *Cache) Refresh(ctx context.Context, pool *pgxpool.Pool, projectKey, envKey string) error {
	query := baseFlagQuery + " WHERE p.key = $1 AND e.key = $2"
	rows, err := pool.Query(ctx, query, projectKey, envKey)
	if err != nil {
		return fmt.Errorf("cache Refresh query: %w", err)
	}
	defer rows.Close()

	flags := make(map[string]FlagData)

	for rows.Next() {
		_, _, fd, err := scanFlagRow(rows)
		if err != nil {
			return fmt.Errorf("cache Refresh scan: %w", err)
		}
		flags[fd.Flag.Key] = fd
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("cache Refresh rows: %w", err)
	}

	key := cacheKey(projectKey, envKey)
	c.mu.Lock()
	c.data[key] = flags
	c.mu.Unlock()

	return nil
}

// GetFlags returns all flag data for a project/environment.
// Returns nil if the project/environment combination is not found.
func (c *Cache) GetFlags(projectKey, envKey string) map[string]FlagData {
	key := cacheKey(projectKey, envKey)
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data[key]
}

// GetFlag returns a single flag's data for a project/environment.
func (c *Cache) GetFlag(projectKey, envKey, flagKey string) (FlagData, bool) {
	key := cacheKey(projectKey, envKey)
	c.mu.RLock()
	defer c.mu.RUnlock()
	flags := c.data[key]
	if flags == nil {
		return FlagData{}, false
	}
	fd, ok := flags[flagKey]
	return fd, ok
}

// Set directly sets flag data for a project/environment (useful for testing).
func (c *Cache) Set(projectKey, envKey string, flags map[string]FlagData) {
	key := cacheKey(projectKey, envKey)
	c.mu.Lock()
	c.data[key] = flags
	c.mu.Unlock()
}

// rowScanner is an interface satisfied by pgx.Rows for scanning a single row.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanFlagRow scans a single row from the flag query into FlagData.
func scanFlagRow(row rowScanner) (projectKey, envKey string, fd FlagData, err error) {
	var (
		variantsJSON       []byte
		targetingRulesJSON []byte
		fecUpdatedAt       time.Time
	)

	err = row.Scan(
		&projectKey,
		&envKey,
		// Flag fields
		&fd.Flag.ID,
		&fd.Flag.ProjectID,
		&fd.Flag.Key,
		&fd.Flag.Name,
		&fd.Flag.Description,
		&fd.Flag.ValueType,
		&fd.Flag.FlagType,
		&fd.Flag.DefaultValue,
		&fd.Flag.Tags,
		&fd.Flag.LifecycleStatus,
		&fd.Flag.LifecycleStatusChangedAt,
		&fd.Flag.CreatedAt,
		&fd.Flag.UpdatedAt,
		// FlagEnvironmentConfig fields
		&fd.Config.ID,
		&fd.Config.FlagID,
		&fd.Config.EnvironmentID,
		&fd.Config.Enabled,
		&fd.Config.DefaultVariant,
		&variantsJSON,
		&targetingRulesJSON,
		&fecUpdatedAt,
	)
	if err != nil {
		return "", "", FlagData{}, err
	}

	fd.Config.UpdatedAt = fecUpdatedAt

	if len(variantsJSON) > 0 {
		if err := json.Unmarshal(variantsJSON, &fd.Config.Variants); err != nil {
			return "", "", FlagData{}, fmt.Errorf("unmarshal variants: %w", err)
		}
	}

	if len(targetingRulesJSON) > 0 {
		if err := json.Unmarshal(targetingRulesJSON, &fd.Config.TargetingRules); err != nil {
			return "", "", FlagData{}, fmt.Errorf("unmarshal targeting_rules: %w", err)
		}
	}

	return projectKey, envKey, fd, nil
}
