ALTER TABLE flags ADD COLUMN owner_id UUID REFERENCES users(id) ON DELETE SET NULL;
CREATE INDEX idx_flags_owner_id ON flags(owner_id);
