-- Migration: Create API tokens table for service account and token-based authentication
-- Purpose: Manage API tokens for service accounts, integrations, and programmatic access
-- Date: 2025-12-07

CREATE TABLE IF NOT EXISTS api_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL, -- User-friendly name for the token
    description TEXT, -- What this token is for
    token_hash VARCHAR(64) NOT NULL UNIQUE, -- SHA256 hash of actual token (for secure storage)
    token_prefix VARCHAR(16) NOT NULL, -- First 8 chars + random suffix of token (for display)
    type VARCHAR(50) NOT NULL DEFAULT 'bearer', -- 'bearer', 'basic', 'oauth'
    status VARCHAR(50) NOT NULL DEFAULT 'active', -- 'active', 'revoked', 'expired', 'disabled'
    permissions JSONB, -- Specific permissions (overrides user's default role permissions)
    last_used_at TIMESTAMP, -- When token was last used
    expires_at TIMESTAMP, -- When token expires (NULL = never expires)
    revoked_at TIMESTAMP, -- When token was revoked
    revoke_reason VARCHAR(255), -- Why it was revoked
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT, -- Which user/admin created this
    ip_whitelist JSONB, -- Array of IP addresses or CIDR ranges for access control
    scopes JSONB, -- Array of scopes (OAuth scopes or permission scopes)
    metadata JSONB -- Custom metadata (key-value pairs)
);

-- Create indexes for efficient querying
CREATE INDEX idx_api_tokens_user_id ON api_tokens(user_id);
CREATE INDEX idx_api_tokens_token_hash ON api_tokens(token_hash);
CREATE INDEX idx_api_tokens_token_prefix ON api_tokens(token_prefix);
CREATE INDEX idx_api_tokens_status ON api_tokens(status);
CREATE INDEX idx_api_tokens_created_by_id ON api_tokens(created_by_id);
CREATE INDEX idx_api_tokens_expires_at ON api_tokens(expires_at) WHERE status = 'active';
CREATE INDEX idx_api_tokens_created_at ON api_tokens(created_at DESC);
CREATE INDEX idx_api_tokens_user_status ON api_tokens(user_id, status);
CREATE INDEX idx_api_tokens_last_used ON api_tokens(last_used_at DESC);

-- Add comments for clarity
COMMENT ON TABLE api_tokens IS 'API tokens for service accounts, integrations, and programmatic access to OpenRisk API';
COMMENT ON COLUMN api_tokens.user_id IS 'Reference to the user who owns this token';
COMMENT ON COLUMN api_tokens.name IS 'Human-readable name for token management';
COMMENT ON COLUMN api_tokens.token_hash IS 'SHA256 hash of actual token (never store plaintext tokens)';
COMMENT ON COLUMN api_tokens.token_prefix IS 'First 8 characters of token for display purposes (prefixed with orsk_)';
COMMENT ON COLUMN api_tokens.type IS 'Token authentication type: bearer (standard), basic (username:password), oauth (OAuth 2.0)';
COMMENT ON COLUMN api_tokens.status IS 'Token status: active (usable), revoked (intentionally disabled), expired (past expiry), disabled (temporary)';
COMMENT ON COLUMN api_tokens.permissions IS 'JSON array of permissions (e.g., ["risk:read:any", "mitigation:update:own"])';
COMMENT ON COLUMN api_tokens.last_used_at IS 'Timestamp of last successful API call using this token';
COMMENT ON COLUMN api_tokens.expires_at IS 'Expiration timestamp (NULL means token never expires, default 90 days)';
COMMENT ON COLUMN api_tokens.revoked_at IS 'Timestamp when token was revoked';
COMMENT ON COLUMN api_tokens.revoke_reason IS 'Optional reason for revocation (e.g., "security breach", "no longer needed")';
COMMENT ON COLUMN api_tokens.created_by_id IS 'User ID of who created this token (for audit trail)';
COMMENT ON COLUMN api_tokens.ip_whitelist IS 'JSON array of allowed IP addresses or CIDR ranges';
COMMENT ON COLUMN api_tokens.scopes IS 'JSON array of permission scopes (e.g., ["risk", "mitigation", "export"])';
COMMENT ON COLUMN api_tokens.metadata IS 'JSON object for extensible metadata (integration info, tags, etc.)';

-- Create trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_api_tokens_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER api_tokens_updated_at_trigger
BEFORE UPDATE ON api_tokens
FOR EACH ROW
EXECUTE FUNCTION update_api_tokens_updated_at();
