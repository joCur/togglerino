ALTER TABLE flag_environment_configs
    ADD COLUMN locked BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN locked_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN locked_at TIMESTAMPTZ,
    ADD COLUMN lock_reason TEXT CHECK (length(lock_reason) <= 255);

ALTER TABLE scheduled_flag_changes
    DROP CONSTRAINT scheduled_flag_changes_status_check;
ALTER TABLE scheduled_flag_changes
    ADD CONSTRAINT scheduled_flag_changes_status_check
    CHECK (status IN ('pending', 'executed', 'cancelled', 'failed'));
