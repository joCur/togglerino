CREATE TABLE personal_access_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (length(name) <= 100),
    token_hash TEXT NOT NULL,
    token_prefix VARCHAR(12) NOT NULL,
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_personal_access_tokens_hash ON personal_access_tokens (token_hash);
CREATE INDEX idx_personal_access_tokens_user ON personal_access_tokens (user_id);
