CREATE TABLE flag_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    flag_type TEXT NOT NULL DEFAULT 'release',
    value_type TEXT NOT NULL DEFAULT 'boolean',
    default_value JSONB NOT NULL DEFAULT 'false',
    tags TEXT[] NOT NULL DEFAULT '{}',
    environment_defaults JSONB NOT NULL DEFAULT '{}',
    variant_config JSONB NOT NULL DEFAULT '{}',
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_flag_templates_unique_key ON flag_templates(COALESCE(project_id, '00000000-0000-0000-0000-000000000000'), key);
CREATE INDEX idx_flag_templates_project_id ON flag_templates(project_id);
