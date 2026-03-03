ALTER TABLE audit_log ADD COLUMN batch_id UUID;
CREATE INDEX idx_audit_log_batch_id ON audit_log (batch_id) WHERE batch_id IS NOT NULL;
