ALTER TABLE flag_environment_configs
    DROP COLUMN locked,
    DROP COLUMN locked_by,
    DROP COLUMN locked_at,
    DROP COLUMN lock_reason;
