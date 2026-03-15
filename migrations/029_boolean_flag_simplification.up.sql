-- Clear variants and default_variant for boolean flags.
-- Also migrate targeting rule variant values from "on"/"off" to "true"/"false".
UPDATE flag_environment_configs fec
SET
    variants = '[]'::jsonb,
    default_variant = '',
    targeting_rules = (
        SELECT COALESCE(jsonb_agg(
            CASE
                WHEN rule->>'variant' = 'on' THEN jsonb_set(rule, '{variant}', '"true"')
                WHEN rule->>'variant' = 'off' THEN jsonb_set(rule, '{variant}', '"false"')
                ELSE rule
            END
        ), '[]'::jsonb)
        FROM jsonb_array_elements(fec.targeting_rules) AS rule
    )
FROM flags f
WHERE fec.flag_id = f.id
  AND f.value_type = 'boolean';
