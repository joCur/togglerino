ALTER TABLE oidc_providers DROP CONSTRAINT oidc_providers_singleton_check;
ALTER TABLE oidc_providers DROP CONSTRAINT oidc_providers_singleton_unique;
ALTER TABLE oidc_providers DROP COLUMN singleton;
