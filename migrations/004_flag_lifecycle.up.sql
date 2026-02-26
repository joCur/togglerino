-- Add flag purpose type
ALTER TABLE flags ADD COLUMN flag_type TEXT NOT NULL DEFAULT 'release'
    CHECK (flag_type IN ('release', 'experiment', 'operational', 'kill-switch', 'permission'));

-- Add lifecycle status (replaces archived boolean)
ALTER TABLE flags ADD COLUMN lifecycle_status TEXT NOT NULL DEFAULT 'active'
    CHECK (lifecycle_status IN ('active', 'potentially_stale', 'stale', 'archived'));
ALTER TABLE flags ADD COLUMN lifecycle_status_changed_at TIMESTAMPTZ;

-- Migrate archived flags to lifecycle_status
UPDATE flags SET lifecycle_status = 'archived', lifecycle_status_changed_at = updated_at WHERE archived = TRUE;

-- Drop archived column
ALTER TABLE flags DROP COLUMN archived;

-- Project settings table
CREATE TABLE project_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL UNIQUE REFERENCES projects(id) ON DELETE CASCADE,
    settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
