ALTER TABLE flag_environment_configs
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL;
