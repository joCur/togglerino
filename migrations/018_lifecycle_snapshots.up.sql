CREATE TABLE lifecycle_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    active_count INTEGER NOT NULL DEFAULT 0,
    potentially_stale_count INTEGER NOT NULL DEFAULT 0,
    stale_count INTEGER NOT NULL DEFAULT 0,
    archived_count INTEGER NOT NULL DEFAULT 0,
    recorded_at DATE NOT NULL DEFAULT CURRENT_DATE,
    UNIQUE (project_id, recorded_at)
);

CREATE INDEX idx_lifecycle_snapshots_project_date ON lifecycle_snapshots (project_id, recorded_at);
