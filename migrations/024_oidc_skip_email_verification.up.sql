ALTER TABLE oidc_providers ADD COLUMN skip_email_verification BOOLEAN NOT NULL DEFAULT FALSE;
