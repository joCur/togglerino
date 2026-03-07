ALTER TABLE environments ADD COLUMN sort_order integer NOT NULL DEFAULT 0;

WITH ranked AS (
  SELECT id, ROW_NUMBER() OVER (PARTITION BY project_id ORDER BY created_at) - 1 AS rn
  FROM environments
)
UPDATE environments SET sort_order = ranked.rn FROM ranked WHERE environments.id = ranked.id;
