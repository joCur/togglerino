-- 1. Rename default_variant → fallthrough_variant
ALTER TABLE flag_environment_configs RENAME COLUMN default_variant TO fallthrough_variant;

-- 2. Add off_variant column
ALTER TABLE flag_environment_configs ADD COLUMN off_variant TEXT NOT NULL DEFAULT '';

-- 3. Rename variant JSON field key → name in variants JSONB
UPDATE flag_environment_configs
SET variants = (
    SELECT COALESCE(jsonb_agg(
        jsonb_set(v - 'key', '{name}', v->'key')
    ), '[]'::jsonb)
    FROM jsonb_array_elements(variants) AS v
)
WHERE variants != '[]'::jsonb;

-- 4. Populate off_variant for existing flags:
--    Boolean flags: "false"
--    Non-boolean flags: copy current fallthrough_variant
UPDATE flag_environment_configs fec
SET off_variant = CASE
    WHEN f.value_type = 'boolean' THEN 'false'
    ELSE fec.fallthrough_variant
END
FROM flags f
WHERE fec.flag_id = f.id;

-- 5. Populate variants for existing boolean flags
UPDATE flag_environment_configs fec
SET variants = '[{"name": "true", "value": true}, {"name": "false", "value": false}]'::jsonb,
    fallthrough_variant = 'true'
FROM flags f
WHERE fec.flag_id = f.id
  AND f.value_type = 'boolean';

-- 6. Transform scheduled_flag_changes config snapshots
UPDATE scheduled_flag_changes
SET config_snapshot = jsonb_set(
    jsonb_set(
        config_snapshot - 'default_variant',
        '{fallthrough_variant}',
        COALESCE(config_snapshot->'default_variant', '""'::jsonb)
    ),
    '{off_variant}',
    COALESCE(config_snapshot->'default_variant', '""'::jsonb)
)
WHERE config_snapshot ? 'default_variant';

-- Also rename variant key → name in scheduled config snapshot variants
UPDATE scheduled_flag_changes
SET config_snapshot = jsonb_set(
    config_snapshot,
    '{variants}',
    (
        SELECT COALESCE(jsonb_agg(
            jsonb_set(v - 'key', '{name}', v->'key')
        ), '[]'::jsonb)
        FROM jsonb_array_elements(config_snapshot->'variants') AS v
    )
)
WHERE config_snapshot->'variants' IS NOT NULL
  AND config_snapshot->'variants' != '[]'::jsonb;

-- 7. Transform flag_templates variant_config
UPDATE flag_templates
SET variant_config = jsonb_set(
    jsonb_set(
        variant_config - 'default_variant',
        '{fallthrough_variant}',
        COALESCE(variant_config->'default_variant', '""'::jsonb)
    ),
    '{off_variant}',
    COALESCE(variant_config->'default_variant', '""'::jsonb)
)
WHERE variant_config ? 'default_variant';

UPDATE flag_templates
SET variant_config = jsonb_set(
    variant_config,
    '{variants}',
    (
        SELECT COALESCE(jsonb_agg(
            jsonb_set(v - 'key', '{name}', v->'key')
        ), '[]'::jsonb)
        FROM jsonb_array_elements(variant_config->'variants') AS v
    )
)
WHERE variant_config->'variants' IS NOT NULL
  AND variant_config->'variants' != '[]'::jsonb;

-- 8. Transform project_settings environment_defaults (nested inside settings JSONB column)
UPDATE project_settings
SET settings = jsonb_set(
    settings,
    '{environment_defaults}',
    (
        SELECT jsonb_object_agg(
            key,
            CASE
                WHEN value ? 'default_variant' THEN
                    jsonb_set(
                        jsonb_set(
                            value - 'default_variant',
                            '{fallthrough_variant}',
                            COALESCE(value->'default_variant', '""'::jsonb)
                        ),
                        '{off_variant}',
                        COALESCE(value->'default_variant', '""'::jsonb)
                    )
                ELSE value
            END
        )
        FROM jsonb_each(settings->'environment_defaults')
    )
)
WHERE settings->'environment_defaults' IS NOT NULL
  AND settings->'environment_defaults' != '{}'::jsonb;
