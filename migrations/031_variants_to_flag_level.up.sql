-- Add variants column to flags table
ALTER TABLE flags ADD COLUMN variants JSONB NOT NULL DEFAULT '[]'::jsonb;

-- Populate from first environment config (they should all be the same)
UPDATE flags f
SET variants = COALESCE(
    (SELECT fec.variants
     FROM flag_environment_configs fec
     WHERE fec.flag_id = f.id
     AND fec.variants != '[]'::jsonb
     LIMIT 1),
    '[]'::jsonb
);

-- Drop variants from flag_environment_configs
ALTER TABLE flag_environment_configs DROP COLUMN variants;
