-- App identity: maps dashboard user to their application user ID per project
CREATE TABLE user_app_identities (
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id    UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    app_user_id   TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, project_id),
    UNIQUE (project_id, app_user_id)
);

CREATE INDEX idx_user_app_identities_project_app_user
    ON user_app_identities(project_id, app_user_id);

-- Personal flag overrides
CREATE TABLE flag_overrides (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    flag_id         UUID NOT NULL REFERENCES flags(id) ON DELETE CASCADE,
    environment_id  UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    value           JSONB NOT NULL,
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, flag_id, environment_id)
);

CREATE INDEX idx_flag_overrides_expires_at
    ON flag_overrides(expires_at) WHERE expires_at IS NOT NULL;
