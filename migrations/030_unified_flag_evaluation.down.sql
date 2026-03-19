-- Reverse: rename fallthrough_variant back to default_variant, drop off_variant
ALTER TABLE flag_environment_configs RENAME COLUMN fallthrough_variant TO default_variant;
ALTER TABLE flag_environment_configs DROP COLUMN IF EXISTS off_variant;

-- Reverse variant name → key
UPDATE flag_environment_configs
SET variants = (
    SELECT COALESCE(jsonb_agg(
        jsonb_set(v - 'name', '{key}', v->'name')
    ), '[]'::jsonb)
    FROM jsonb_array_elements(variants) AS v
)
WHERE variants != '[]'::jsonb;
