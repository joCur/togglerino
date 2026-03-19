ALTER TABLE flag_environment_configs ADD COLUMN variants JSONB NOT NULL DEFAULT '[]'::jsonb;
UPDATE flag_environment_configs fec SET variants = f.variants FROM flags f WHERE fec.flag_id = f.id;
ALTER TABLE flags DROP COLUMN variants;
