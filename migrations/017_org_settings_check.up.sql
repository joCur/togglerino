ALTER TABLE org_settings ADD CONSTRAINT chk_base_project_role
  CHECK (key != 'base_project_role' OR value IN ('admin', 'editor', 'viewer', 'none'));
