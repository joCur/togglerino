DROP INDEX IF EXISTS idx_audit_log_batch_id;
ALTER TABLE audit_log DROP COLUMN IF EXISTS batch_id;
