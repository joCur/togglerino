ALTER TABLE flag_environment_configs
    DROP COLUMN locked,
    DROP COLUMN locked_by,
    DROP COLUMN locked_at,
    DROP COLUMN lock_reason;

ALTER TABLE scheduled_flag_changes
    DROP CONSTRAINT scheduled_flag_changes_status_check;
ALTER TABLE scheduled_flag_changes
    ADD CONSTRAINT scheduled_flag_changes_status_check
    CHECK (status IN ('pending', 'executed', 'cancelled'));
