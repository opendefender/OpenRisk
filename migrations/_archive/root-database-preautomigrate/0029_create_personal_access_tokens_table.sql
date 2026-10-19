-- +migrate Up
-- Create personal_access_tokens table for PAT functionality
CREATE TABLE IF NOT EXISTS personal_access_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    token_hash VARCHAR(64) NOT NULL UNIQUE, -- SHA256 hash
    token_prefix VARCHAR(8) NOT NULL, -- First 8 chars for display
    scopes JSONB DEFAULT '[]'::jsonb, -- permission scopes
    expires_at TIMESTAMP WITH TIME ZONE,
    last_used_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Create indexes for personal_access_tokens
CREATE INDEX IF NOT EXISTS idx_personal_access_tokens_user_id ON personal_access_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_personal_access_tokens_token_hash ON personal_access_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_personal_access_tokens_token_prefix ON personal_access_tokens(token_prefix);
CREATE INDEX IF NOT EXISTS idx_personal_access_tokens_expires_at ON personal_access_tokens(expires_at);

-- +migrate Down
DROP TABLE IF EXISTS personal_access_tokens;