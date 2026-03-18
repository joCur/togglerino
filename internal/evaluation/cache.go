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

// OverrideEntry holds a cached personal override value.
type OverrideEntry struct {
	Value     json.RawMessage
	ExpiresAt *time.Time
}

// overrideKey is a composite map key that avoids the collision risk of string
// concatenation (e.g. appUserID containing ":").
type overrideKey struct {
	appUserID string
	flagKey   string
}

// Cache holds all flag data in memory for fast evaluation.
type Cache struct {
	mu sync.RWMutex
	// Key: "projectKey:envKey", Value: map of flagKey -> FlagData
	data map[string]map[string]FlagData
	// Key: projectKey, Value: map of segmentKey -> Segment
	segments map[string]map[string]model.Segment
	// Key: "projectKey:envKey", Value: map of overrideKey -> OverrideEntry
	overrides map[string]map[overrideKey]OverrideEntry
}

// NewCache creates a new empty cache.
func NewCache() *Cache {
	return &Cache{
		data:      make(map[string]map[string]FlagData),
		segments:  make(map[string]map[string]model.Segment),
		overrides: make(map[string]map[overrideKey]OverrideEntry),
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
    fec.id, fec.flag_id, fec.environment_id, fec.enabled, fec.fallthrough_variant, fec.off_variant, fec.variants, fec.targeting_rules, fec.updated_at
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

	// Load segments (project-scoped)
	segRows, err := pool.Query(ctx,
		`SELECT p.key AS project_key, s.id, s.project_id, s.key, s.name, s.description, s.conditions, s.created_at, s.updated_at
		 FROM segments s
		 JOIN projects p ON p.id = s.project_id`)
	if err != nil {
		return fmt.Errorf("cache LoadAll segments query: %w", err)
	}
	defer segRows.Close()

	newSegments := make(map[string]map[string]model.Segment)
	for segRows.Next() {
		var projectKey string
		var seg model.Segment
		var condJSON []byte
		if err := segRows.Scan(&projectKey, &seg.ID, &seg.ProjectID, &seg.Key, &seg.Name, &seg.Description, &condJSON, &seg.CreatedAt, &seg.UpdatedAt); err != nil {
			return fmt.Errorf("cache LoadAll segment scan: %w", err)
		}
		if err := json.Unmarshal(condJSON, &seg.Conditions); err != nil {
			return fmt.Errorf("cache LoadAll segment unmarshal: %w", err)
		}
		if seg.Conditions == nil {
			seg.Conditions = []model.Condition{}
		}
		if newSegments[projectKey] == nil {
			newSegments[projectKey] = make(map[string]model.Segment)
		}
		newSegments[projectKey][seg.Key] = seg
	}
	if err := segRows.Err(); err != nil {
		return fmt.Errorf("cache LoadAll segments rows: %w", err)
	}

	c.mu.Lock()
	c.data = newData
	c.segments = newSegments
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

// RefreshSegments reloads all segments for a specific project from the database.
// Called after a segment is created, updated, or deleted.
func (c *Cache) RefreshSegments(ctx context.Context, pool *pgxpool.Pool, projectKey string) error {
	rows, err := pool.Query(ctx,
		`SELECT s.id, s.project_id, s.key, s.name, s.description, s.conditions, s.created_at, s.updated_at
		 FROM segments s
		 JOIN projects p ON p.id = s.project_id
		 WHERE p.key = $1`, projectKey)
	if err != nil {
		return fmt.Errorf("cache RefreshSegments query: %w", err)
	}
	defer rows.Close()

	segs := make(map[string]model.Segment)
	for rows.Next() {
		var seg model.Segment
		var condJSON []byte
		if err := rows.Scan(&seg.ID, &seg.ProjectID, &seg.Key, &seg.Name, &seg.Description, &condJSON, &seg.CreatedAt, &seg.UpdatedAt); err != nil {
			return fmt.Errorf("cache RefreshSegments scan: %w", err)
		}
		if err := json.Unmarshal(condJSON, &seg.Conditions); err != nil {
			return fmt.Errorf("cache RefreshSegments unmarshal: %w", err)
		}
		if seg.Conditions == nil {
			seg.Conditions = []model.Condition{}
		}
		segs[seg.Key] = seg
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("cache RefreshSegments rows: %w", err)
	}

	c.mu.Lock()
	c.segments[projectKey] = segs
	c.mu.Unlock()
	return nil
}

// FlagCount returns the total number of cached flags across all project/environments.
func (c *Cache) FlagCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	count := 0
	for _, flags := range c.data {
		count += len(flags)
	}
	return count
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

// SetFlag directly sets a single flag's data for a project/environment (useful for testing).
func (c *Cache) SetFlag(projectKey, envKey, flagKey string, data FlagData) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := cacheKey(projectKey, envKey)
	if c.data[key] == nil {
		c.data[key] = make(map[string]FlagData)
	}
	c.data[key][flagKey] = data
}

// GetSegments returns all segments for a project, keyed by segment key.
// Returns nil if the project has no segments cached.
func (c *Cache) GetSegments(projectKey string) map[string]model.Segment {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.segments[projectKey]
}

// SetSegments stores segments for a project, keyed by segment key.
func (c *Cache) SetSegments(projectKey string, segments map[string]model.Segment) {
	c.mu.Lock()
	c.segments[projectKey] = segments
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
		&fd.Config.FallthroughVariant,
		&fd.Config.OffVariant,
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

func (c *Cache) SetOverride(projectKey, envKey, appUserID, flagKey string, value json.RawMessage, expiresAt *time.Time) {
	key := cacheKey(projectKey, envKey)
	oKey := overrideKey{appUserID: appUserID, flagKey: flagKey}
	c.mu.Lock()
	if c.overrides[key] == nil {
		c.overrides[key] = make(map[overrideKey]OverrideEntry)
	}
	c.overrides[key][oKey] = OverrideEntry{Value: value, ExpiresAt: expiresAt}
	c.mu.Unlock()
}

func (c *Cache) GetOverride(projectKey, envKey, appUserID, flagKey string) (json.RawMessage, bool) {
	key := cacheKey(projectKey, envKey)
	oKey := overrideKey{appUserID: appUserID, flagKey: flagKey}
	c.mu.RLock()
	defer c.mu.RUnlock()
	overrides := c.overrides[key]
	if overrides == nil {
		return nil, false
	}
	entry, ok := overrides[oKey]
	if !ok {
		return nil, false
	}
	if entry.ExpiresAt != nil && entry.ExpiresAt.Before(time.Now()) {
		return nil, false
	}
	return entry.Value, true
}

func (c *Cache) DeleteOverride(projectKey, envKey, appUserID, flagKey string) {
	key := cacheKey(projectKey, envKey)
	oKey := overrideKey{appUserID: appUserID, flagKey: flagKey}
	c.mu.Lock()
	if c.overrides[key] != nil {
		delete(c.overrides[key], oKey)
	}
	c.mu.Unlock()
}

func (c *Cache) DeleteOverridesForUser(projectKey, envKey, appUserID string) {
	key := cacheKey(projectKey, envKey)
	c.mu.Lock()
	if c.overrides[key] != nil {
		for k := range c.overrides[key] {
			if k.appUserID == appUserID {
				delete(c.overrides[key], k)
			}
		}
	}
	c.mu.Unlock()
}

// Evict removes all cached data and overrides for a specific project/environment.
// Called when an environment is deleted.
func (c *Cache) Evict(projectKey, envKey string) {
	key := cacheKey(projectKey, envKey)
	c.mu.Lock()
	delete(c.data, key)
	delete(c.overrides, key)
	c.mu.Unlock()
}

// PurgeExpiredOverrides removes all expired override entries from the cache.
func (c *Cache) PurgeExpiredOverrides() {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	for envKey, overrides := range c.overrides {
		for oKey, entry := range overrides {
			if entry.ExpiresAt != nil && entry.ExpiresAt.Before(now) {
				delete(overrides, oKey)
			}
		}
		if len(overrides) == 0 {
			delete(c.overrides, envKey)
		}
	}
}

// LoadOverrides bulk-loads override entries into the cache.
func (c *Cache) LoadOverrides(entries []model.OverrideCacheEntry) {
	newOverrides := make(map[string]map[overrideKey]OverrideEntry)
	for _, e := range entries {
		key := cacheKey(e.ProjectKey, e.EnvironmentKey)
		oKey := overrideKey{appUserID: e.AppUserID, flagKey: e.FlagKey}
		if newOverrides[key] == nil {
			newOverrides[key] = make(map[overrideKey]OverrideEntry)
		}
		newOverrides[key][oKey] = OverrideEntry{Value: e.Value, ExpiresAt: e.ExpiresAt}
	}
	c.mu.Lock()
	c.overrides = newOverrides
	c.mu.Unlock()
}
