DROP INDEX IF EXISTS idx_audit_log_flag_history;
ALTER TABLE audit_log DROP COLUMN IF EXISTS user_email;
ALTER TABLE audit_log DROP COLUMN IF EXISTS environment_id;
