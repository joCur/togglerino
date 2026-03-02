package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type ProjectSettingsStore struct {
	pool *pgxpool.Pool
}

func NewProjectSettingsStore(pool *pgxpool.Pool) *ProjectSettingsStore {
	return &ProjectSettingsStore{pool: pool}
}

// Get returns the project settings for a project. Returns nil (no error) if no settings exist yet.
func (s *ProjectSettingsStore) Get(ctx context.Context, projectID string) (*model.ProjectSettings, error) {
	var ps model.ProjectSettings
	var settingsJSON []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, settings, updated_at FROM project_settings WHERE project_id = $1`,
		projectID,
	).Scan(&ps.ID, &ps.ProjectID, &settingsJSON, &ps.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("getting project settings: %w", err)
	}

	var raw struct {
		FlagLifetimes       map[model.FlagType]*int            `json:"flag_lifetimes"`
		EnvironmentDefaults map[string]model.EnvironmentDefault `json:"environment_defaults,omitempty"`
	}
	if len(settingsJSON) > 0 {
		if err := json.Unmarshal(settingsJSON, &raw); err != nil {
			return nil, fmt.Errorf("unmarshaling project settings: %w", err)
		}
	}
	ps.FlagLifetimes = raw.FlagLifetimes
	ps.EnvironmentDefaults = raw.EnvironmentDefaults
	return &ps, nil
}

// Upsert creates or updates the flag_lifetimes portion of project settings,
// preserving other settings (e.g., environment_defaults).
func (s *ProjectSettingsStore) Upsert(ctx context.Context, projectID string, flagLifetimes map[model.FlagType]*int) (*model.ProjectSettings, error) {
	// Read existing settings to preserve other keys (environment_defaults).
	var existingJSON []byte
	err := s.pool.QueryRow(ctx,
		`SELECT settings FROM project_settings WHERE project_id = $1`,
		projectID,
	).Scan(&existingJSON)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("reading existing settings: %w", err)
	}

	var full map[string]json.RawMessage
	if len(existingJSON) > 0 {
		if err := json.Unmarshal(existingJSON, &full); err != nil {
			return nil, fmt.Errorf("unmarshaling existing settings: %w", err)
		}
	}
	if full == nil {
		full = make(map[string]json.RawMessage)
	}

	lifetimesJSON, err := json.Marshal(flagLifetimes)
	if err != nil {
		return nil, fmt.Errorf("marshaling flag lifetimes: %w", err)
	}
	full["flag_lifetimes"] = lifetimesJSON

	mergedJSON, err := json.Marshal(full)
	if err != nil {
		return nil, fmt.Errorf("marshaling merged settings: %w", err)
	}

	var ps model.ProjectSettings
	var returnedJSON []byte
	err = s.pool.QueryRow(ctx,
		`INSERT INTO project_settings (project_id, settings)
		 VALUES ($1, $2)
		 ON CONFLICT (project_id) DO UPDATE SET settings = $2, updated_at = NOW()
		 RETURNING id, project_id, settings, updated_at`,
		projectID, mergedJSON,
	).Scan(&ps.ID, &ps.ProjectID, &returnedJSON, &ps.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upserting project settings: %w", err)
	}

	var raw struct {
		FlagLifetimes       map[model.FlagType]*int            `json:"flag_lifetimes"`
		EnvironmentDefaults map[string]model.EnvironmentDefault `json:"environment_defaults,omitempty"`
	}
	if err := json.Unmarshal(returnedJSON, &raw); err != nil {
		return nil, fmt.Errorf("unmarshaling upserted settings: %w", err)
	}
	ps.FlagLifetimes = raw.FlagLifetimes
	ps.EnvironmentDefaults = raw.EnvironmentDefaults
	return &ps, nil
}

// GetAll returns all project settings keyed by project ID (for staleness checker bulk load).
func (s *ProjectSettingsStore) GetAll(ctx context.Context) (map[string]*model.ProjectSettings, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, project_id, settings, updated_at FROM project_settings`)
	if err != nil {
		return nil, fmt.Errorf("listing project settings: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*model.ProjectSettings)
	for rows.Next() {
		var ps model.ProjectSettings
		var settingsJSON []byte
		if err := rows.Scan(&ps.ID, &ps.ProjectID, &settingsJSON, &ps.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning project settings: %w", err)
		}
		var raw struct {
			FlagLifetimes       map[model.FlagType]*int            `json:"flag_lifetimes"`
			EnvironmentDefaults map[string]model.EnvironmentDefault `json:"environment_defaults,omitempty"`
		}
		if err := json.Unmarshal(settingsJSON, &raw); err != nil {
			return nil, fmt.Errorf("unmarshaling project settings row: %w", err)
		}
		ps.FlagLifetimes = raw.FlagLifetimes
		ps.EnvironmentDefaults = raw.EnvironmentDefaults
		result[ps.ProjectID] = &ps
	}
	return result, rows.Err()
}

// UpsertEnvironmentDefaults creates or updates just the environment_defaults
// portion of project settings, preserving other settings (e.g., flag_lifetimes).
func (s *ProjectSettingsStore) UpsertEnvironmentDefaults(ctx context.Context, projectID string, envDefaults map[string]model.EnvironmentDefault) (*model.ProjectSettings, error) {
	// Read existing settings to preserve other keys (flag_lifetimes).
	var existingJSON []byte
	err := s.pool.QueryRow(ctx,
		`SELECT settings FROM project_settings WHERE project_id = $1`,
		projectID,
	).Scan(&existingJSON)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("reading existing settings: %w", err)
	}

	var full map[string]json.RawMessage
	if len(existingJSON) > 0 {
		if err := json.Unmarshal(existingJSON, &full); err != nil {
			return nil, fmt.Errorf("unmarshaling existing settings: %w", err)
		}
	}
	if full == nil {
		full = make(map[string]json.RawMessage)
	}

	envJSON, err := json.Marshal(envDefaults)
	if err != nil {
		return nil, fmt.Errorf("marshaling environment defaults: %w", err)
	}
	full["environment_defaults"] = envJSON

	mergedJSON, err := json.Marshal(full)
	if err != nil {
		return nil, fmt.Errorf("marshaling merged settings: %w", err)
	}

	var ps model.ProjectSettings
	var returnedJSON []byte
	err = s.pool.QueryRow(ctx,
		`INSERT INTO project_settings (project_id, settings)
		 VALUES ($1, $2)
		 ON CONFLICT (project_id) DO UPDATE SET settings = $2, updated_at = NOW()
		 RETURNING id, project_id, settings, updated_at`,
		projectID, mergedJSON,
	).Scan(&ps.ID, &ps.ProjectID, &returnedJSON, &ps.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upserting environment defaults: %w", err)
	}

	var raw struct {
		FlagLifetimes       map[model.FlagType]*int            `json:"flag_lifetimes"`
		EnvironmentDefaults map[string]model.EnvironmentDefault `json:"environment_defaults,omitempty"`
	}
	if err := json.Unmarshal(returnedJSON, &raw); err != nil {
		return nil, fmt.Errorf("unmarshaling upserted settings: %w", err)
	}
	ps.FlagLifetimes = raw.FlagLifetimes
	ps.EnvironmentDefaults = raw.EnvironmentDefaults
	return &ps, nil
}
