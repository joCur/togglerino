CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT UNIQUE NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    permissions TEXT[] NOT NULL,
    is_built_in BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO roles (name, description, permissions, is_built_in) VALUES
    ('admin', 'Full access to all project resources and settings', ARRAY['flags:read', 'flags:write', 'environments:read', 'environments:write', 'sdk_keys:manage', 'segments:write', 'templates:manage', 'project:settings'], true),
    ('editor', 'Can manage flags, environments, segments, and SDK keys', ARRAY['flags:read', 'flags:write', 'environments:read', 'environments:write', 'sdk_keys:manage', 'segments:write', 'templates:manage'], true),
    ('viewer', 'Read-only access to flags and environments', ARRAY['flags:read', 'environments:read'], true);

ALTER TABLE project_members DROP CONSTRAINT IF EXISTS project_members_role_check;

ALTER TABLE project_members ADD CONSTRAINT project_members_role_fkey FOREIGN KEY (role) REFERENCES roles(name) ON UPDATE CASCADE;
