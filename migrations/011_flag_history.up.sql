ALTER TABLE audit_log ADD COLUMN environment_id UUID REFERENCES environments(id) ON DELETE SET NULL;
ALTER TABLE audit_log ADD COLUMN user_email TEXT;
CREATE INDEX idx_audit_log_flag_history ON audit_log(project_id, entity_id, entity_type, created_at DESC);
