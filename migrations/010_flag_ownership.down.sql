DROP INDEX IF EXISTS idx_flags_owner_id;
ALTER TABLE flags DROP COLUMN IF EXISTS owner_id;
