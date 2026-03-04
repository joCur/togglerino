package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type TemplateStore struct {
	pool *pgxpool.Pool
}

func NewTemplateStore(pool *pgxpool.Pool) *TemplateStore {
	return &TemplateStore{pool: pool}
}

// Create inserts a new flag template.
func (s *TemplateStore) Create(ctx context.Context, projectID *string, key, name, description string, flagType model.FlagType, valueType model.ValueType, defaultValue json.RawMessage, tags []string, environmentDefaults json.RawMessage, variantConfig json.RawMessage, isSystem bool, sortOrder int) (*model.FlagTemplate, error) {
	if tags == nil {
		tags = []string{}
	}
	var t model.FlagTemplate
	err := s.pool.QueryRow(ctx,
		`INSERT INTO flag_templates (project_id, key, name, description, flag_type, value_type, default_value, tags, environment_defaults, variant_config, is_system, sort_order)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 RETURNING id, project_id, key, name, description, flag_type, value_type, default_value, tags, environment_defaults, variant_config, is_system, sort_order, created_at, updated_at`,
		projectID, key, name, description, flagType, valueType, defaultValue, tags, environmentDefaults, variantConfig, isSystem, sortOrder,
	).Scan(&t.ID, &t.ProjectID, &t.Key, &t.Name, &t.Description, &t.FlagType, &t.ValueType, &t.DefaultValue, &t.Tags, &t.EnvironmentDefaults, &t.VariantConfig, &t.IsSystem, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating template: %w", err)
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}
	return &t, nil
}

// ListGlobal returns all global (non-project-specific) templates.
func (s *TemplateStore) ListGlobal(ctx context.Context) ([]model.FlagTemplate, error) {
	return s.list(ctx, `SELECT id, project_id, key, name, description, flag_type, value_type, default_value, tags, environment_defaults, variant_config, is_system, sort_order, created_at, updated_at FROM flag_templates WHERE project_id IS NULL ORDER BY sort_order, name`)
}

// ListByProject returns all templates belonging to a specific project.
func (s *TemplateStore) ListByProject(ctx context.Context, projectID string) ([]model.FlagTemplate, error) {
	return s.list(ctx, `SELECT id, project_id, key, name, description, flag_type, value_type, default_value, tags, environment_defaults, variant_config, is_system, sort_order, created_at, updated_at FROM flag_templates WHERE project_id = $1 ORDER BY sort_order, name`, projectID)
}

func (s *TemplateStore) GetByKey(ctx context.Context, projectID *string, key string) (*model.FlagTemplate, error) {
	var t model.FlagTemplate
	var query string
	var args []any
	if projectID == nil {
		query = `SELECT id, project_id, key, name, description, flag_type, value_type, default_value, tags, environment_defaults, variant_config, is_system, sort_order, created_at, updated_at FROM flag_templates WHERE project_id IS NULL AND key = $1`
		args = []any{key}
	} else {
		query = `SELECT id, project_id, key, name, description, flag_type, value_type, default_value, tags, environment_defaults, variant_config, is_system, sort_order, created_at, updated_at FROM flag_templates WHERE project_id = $1 AND key = $2`
		args = []any{*projectID, key}
	}
	err := s.pool.QueryRow(ctx, query, args...).Scan(&t.ID, &t.ProjectID, &t.Key, &t.Name, &t.Description, &t.FlagType, &t.ValueType, &t.DefaultValue, &t.Tags, &t.EnvironmentDefaults, &t.VariantConfig, &t.IsSystem, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting template by key: %w", err)
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}
	return &t, nil
}

func (s *TemplateStore) Update(ctx context.Context, id string, name, description string, flagType model.FlagType, valueType model.ValueType, defaultValue json.RawMessage, tags []string, environmentDefaults json.RawMessage, variantConfig json.RawMessage, sortOrder int) (*model.FlagTemplate, error) {
	if tags == nil {
		tags = []string{}
	}
	var t model.FlagTemplate
	err := s.pool.QueryRow(ctx,
		`UPDATE flag_templates SET name=$2, description=$3, flag_type=$4, value_type=$5, default_value=$6, tags=$7, environment_defaults=$8, variant_config=$9, sort_order=$10, updated_at=NOW()
		 WHERE id=$1
		 RETURNING id, project_id, key, name, description, flag_type, value_type, default_value, tags, environment_defaults, variant_config, is_system, sort_order, created_at, updated_at`,
		id, name, description, flagType, valueType, defaultValue, tags, environmentDefaults, variantConfig, sortOrder,
	).Scan(&t.ID, &t.ProjectID, &t.Key, &t.Name, &t.Description, &t.FlagType, &t.ValueType, &t.DefaultValue, &t.Tags, &t.EnvironmentDefaults, &t.VariantConfig, &t.IsSystem, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating template: %w", err)
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}
	return &t, nil
}

func (s *TemplateStore) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM flag_templates WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting template: %w", err)
	}
	return nil
}

func (s *TemplateStore) list(ctx context.Context, query string, args ...any) ([]model.FlagTemplate, error) {
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing templates: %w", err)
	}
	defer rows.Close()

	var templates []model.FlagTemplate
	for rows.Next() {
		var t model.FlagTemplate
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Key, &t.Name, &t.Description, &t.FlagType, &t.ValueType, &t.DefaultValue, &t.Tags, &t.EnvironmentDefaults, &t.VariantConfig, &t.IsSystem, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning template: %w", err)
		}
		if t.Tags == nil {
			t.Tags = []string{}
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating templates: %w", err)
	}
	if templates == nil {
		templates = []model.FlagTemplate{}
	}
	return templates, nil
}
