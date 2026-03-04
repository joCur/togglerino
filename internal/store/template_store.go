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

// Update modifies a template's mutable fields. The key and is_system columns are
// intentionally excluded from the SET clause to prevent mutation after creation.
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
	result, err := s.pool.Exec(ctx, `DELETE FROM flag_templates WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting template: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("template not found")
	}
	return nil
}

func (s *TemplateStore) SeedSystemTemplates(ctx context.Context) error {
	templates := []struct {
		key, name, description string
		flagType               model.FlagType
		valueType              model.ValueType
		defaultValue           string
		tags                   []string
		envDefaults            string
		variantConfig          string
		sortOrder              int
	}{
		{
			key: "gradual-rollout", name: "Gradual Rollout",
			description: "Boolean flag with gradual percentage rollout",
			flagType: model.FlagTypeRelease, valueType: model.ValueTypeBoolean,
			defaultValue:  `false`,
			tags:          []string{},
			envDefaults:   `{"development":{"enabled":true},"production":{"enabled":false}}`,
			variantConfig: `{"variants":[{"key":"on","value":true},{"key":"off","value":false}],"default_variant":"off","targeting_rules":[{"conditions":[],"variant":"on","percentage_rollout":0}]}`,
			sortOrder:     0,
		},
		{
			key: "kill-switch", name: "Kill Switch",
			description: "Emergency shutoff switch, enabled everywhere",
			flagType: model.FlagTypeKillSwitch, valueType: model.ValueTypeBoolean,
			defaultValue:  `true`,
			tags:          []string{},
			envDefaults:   `{"development":{"enabled":true},"staging":{"enabled":true},"production":{"enabled":true}}`,
			variantConfig: `{"variants":[{"key":"on","value":true}],"default_variant":"on"}`,
			sortOrder:     1,
		},
		{
			key: "ab-test", name: "A/B Test",
			description: "Experiment with two variants split 50/50",
			flagType: model.FlagTypeExperiment, valueType: model.ValueTypeString,
			defaultValue:  `"control"`,
			tags:          []string{},
			envDefaults:   `{"development":{"enabled":true},"production":{"enabled":false}}`,
			variantConfig: `{"variants":[{"key":"control","value":"control"},{"key":"treatment","value":"treatment"}],"default_variant":"control","targeting_rules":[{"conditions":[],"variant":"treatment","percentage_rollout":50}]}`,
			sortOrder:     2,
		},
		{
			key: "permission-gate", name: "Permission Gate",
			description: "Permission flag, disabled by default with targeting rules",
			flagType: model.FlagTypePermission, valueType: model.ValueTypeBoolean,
			defaultValue:  `false`,
			tags:          []string{},
			envDefaults:   `{"development":{"enabled":false},"staging":{"enabled":false},"production":{"enabled":false}}`,
			variantConfig: `{"variants":[{"key":"on","value":true},{"key":"off","value":false}],"default_variant":"off"}`,
			sortOrder:     3,
		},
	}

	for _, tmpl := range templates {
		// ON CONFLICT DO NOTHING: existing system templates are not overwritten on upgrade.
		// To push updated definitions to existing installs, use a migration instead.
		_, err := s.pool.Exec(ctx,
			`INSERT INTO flag_templates (project_id, key, name, description, flag_type, value_type, default_value, tags, environment_defaults, variant_config, is_system, sort_order)
			 VALUES (NULL, $1, $2, $3, $4, $5, $6, $7, $8, $9, TRUE, $10)
			 ON CONFLICT (COALESCE(project_id, '00000000-0000-0000-0000-000000000000'), key) DO NOTHING`,
			tmpl.key, tmpl.name, tmpl.description, tmpl.flagType, tmpl.valueType,
			json.RawMessage(tmpl.defaultValue), tmpl.tags,
			json.RawMessage(tmpl.envDefaults), json.RawMessage(tmpl.variantConfig), tmpl.sortOrder,
		)
		if err != nil {
			return fmt.Errorf("seeding template %q: %w", tmpl.key, err)
		}
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
