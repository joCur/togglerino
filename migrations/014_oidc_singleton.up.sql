-- Enforce single OIDC provider at the database level.
-- The singleton column is always TRUE; the UNIQUE constraint prevents a second row.
ALTER TABLE oidc_providers ADD COLUMN singleton BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE oidc_providers ADD CONSTRAINT oidc_providers_singleton_unique UNIQUE (singleton);
ALTER TABLE oidc_providers ADD CONSTRAINT oidc_providers_singleton_check CHECK (singleton = TRUE);
