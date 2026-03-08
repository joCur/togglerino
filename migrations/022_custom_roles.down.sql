ALTER TABLE project_members DROP CONSTRAINT IF EXISTS project_members_role_fkey;

UPDATE project_members SET role = 'viewer' WHERE role NOT IN ('admin', 'editor', 'viewer');

ALTER TABLE project_members ADD CONSTRAINT project_members_role_check CHECK (role IN ('admin', 'editor', 'viewer'));

DROP TABLE IF EXISTS roles;
