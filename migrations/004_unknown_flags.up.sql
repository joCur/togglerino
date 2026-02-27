CREATE TABLE unknown_flags (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment_id  UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    flag_key        TEXT NOT NULL,
    request_count   BIGINT NOT NULL DEFAULT 1,
    first_seen_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    dismissed_at    TIMESTAMPTZ,
    UNIQUE (project_id, environment_id, flag_key)
);

CREATE INDEX idx_unknown_flags_project ON unknown_flags(project_id) WHERE dismissed_at IS NULL;
