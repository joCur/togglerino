ALTER TABLE flags ADD COLUMN last_evaluated_at TIMESTAMPTZ NULL;
CREATE INDEX idx_flags_last_evaluated_at ON flags (last_evaluated_at) WHERE lifecycle_status != 'archived';
