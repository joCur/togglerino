-- Restore archived column
ALTER TABLE flags ADD COLUMN archived BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE flags SET archived = TRUE WHERE lifecycle_status = 'archived';

-- Drop new columns
ALTER TABLE flags DROP COLUMN lifecycle_status_changed_at;
ALTER TABLE flags DROP COLUMN lifecycle_status;
ALTER TABLE flags DROP COLUMN flag_type;

-- Drop project settings table
DROP TABLE IF EXISTS project_settings;
