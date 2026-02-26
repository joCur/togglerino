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
		FlagLifetimes map[model.FlagType]*int `json:"flag_lifetimes"`
	}
	if len(settingsJSON) > 0 {
		json.Unmarshal(settingsJSON, &raw)
	}
	ps.FlagLifetimes = raw.FlagLifetimes
	return &ps, nil
}

// Upsert creates or updates project settings.
func (s *ProjectSettingsStore) Upsert(ctx context.Context, projectID string, flagLifetimes map[model.FlagType]*int) (*model.ProjectSettings, error) {
	settings := struct {
		FlagLifetimes map[model.FlagType]*int `json:"flag_lifetimes"`
	}{FlagLifetimes: flagLifetimes}

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return nil, fmt.Errorf("marshaling settings: %w", err)
	}

	var ps model.ProjectSettings
	var returnedJSON []byte
	err = s.pool.QueryRow(ctx,
		`INSERT INTO project_settings (project_id, settings)
		 VALUES ($1, $2)
		 ON CONFLICT (project_id) DO UPDATE SET settings = $2, updated_at = NOW()
		 RETURNING id, project_id, settings, updated_at`,
		projectID, settingsJSON,
	).Scan(&ps.ID, &ps.ProjectID, &returnedJSON, &ps.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upserting project settings: %w", err)
	}

	var raw struct {
		FlagLifetimes map[model.FlagType]*int `json:"flag_lifetimes"`
	}
	json.Unmarshal(returnedJSON, &raw)
	ps.FlagLifetimes = raw.FlagLifetimes
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
			FlagLifetimes map[model.FlagType]*int `json:"flag_lifetimes"`
		}
		json.Unmarshal(settingsJSON, &raw)
		ps.FlagLifetimes = raw.FlagLifetimes
		result[ps.ProjectID] = &ps
	}
	return result, rows.Err()
}
