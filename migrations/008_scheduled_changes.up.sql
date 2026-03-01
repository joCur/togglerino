CREATE TABLE scheduled_flag_changes (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    flag_id        UUID NOT NULL REFERENCES flags(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    scheduled_at   TIMESTAMPTZ NOT NULL,
    status         TEXT NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending', 'executed', 'cancelled')),
    config_snapshot JSONB NOT NULL,
    created_by     UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    executed_at    TIMESTAMPTZ,
    cancelled_at   TIMESTAMPTZ,
    cancel_reason  TEXT
);

CREATE INDEX idx_scheduled_flag_changes_pending
    ON scheduled_flag_changes (scheduled_at)
    WHERE status = 'pending';

CREATE INDEX idx_scheduled_flag_changes_flag_env
    ON scheduled_flag_changes (flag_id, environment_id);
