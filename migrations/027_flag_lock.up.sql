ALTER TABLE flag_environment_configs
    ADD COLUMN locked BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN locked_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN locked_at TIMESTAMPTZ,
    ADD COLUMN lock_reason TEXT;
