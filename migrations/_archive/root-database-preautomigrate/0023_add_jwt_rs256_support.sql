-- Migration 0023: Add JWT RS256 refresh tokens and blacklist tables
-- Supports JWT token rotation, revocation, and device fingerprinting

-- Table for storing opaque refresh tokens (not JWT)
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    
    -- Token hash (SHA256 hex) for secure storage
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    
    -- Device fingerprint for security (optional)
    device_fingerprint VARCHAR(255),
    
    -- Expiration and usage tracking
    expires_at TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for efficient queries
CREATE INDEX CONCURRENTLY idx_refresh_tokens_user 
    ON refresh_tokens(user_id) WHERE revoked_at IS NULL;
CREATE INDEX CONCURRENTLY idx_refresh_tokens_tenant 
    ON refresh_tokens(tenant_id) WHERE revoked_at IS NULL;
CREATE INDEX CONCURRENTLY idx_refresh_tokens_hash 
    ON refresh_tokens(token_hash);
CREATE INDEX CONCURRENTLY idx_refresh_tokens_expires 
    ON refresh_tokens(expires_at) WHERE revoked_at IS NULL;

-- Table for JWT token blacklist (JTI = JWT ID)
-- Used to revoke access tokens before natural expiration
-- Redis is primary storage, this is backup/audit trail
CREATE TABLE IF NOT EXISTS token_blacklist (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    jti VARCHAR(255) NOT NULL UNIQUE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    
    -- Why the token was blacklisted
    reason VARCHAR(100) NOT NULL DEFAULT 'logout',
    
    -- When it was blacklisted
    blacklisted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- When the token was supposed to expire
    token_expires_at TIMESTAMPTZ NOT NULL
);

-- Indexes for efficient lookups and cleanup
CREATE INDEX CONCURRENTLY idx_token_blacklist_jti 
    ON token_blacklist(jti);
CREATE INDEX CONCURRENTLY idx_token_blacklist_user 
    ON token_blacklist(user_id);
CREATE INDEX CONCURRENTLY idx_token_blacklist_tenant 
    ON token_blacklist(tenant_id);
CREATE INDEX CONCURRENTLY idx_token_blacklist_expires 
    ON token_blacklist(token_expires_at);

-- Function to clean up expired tokens from blacklist (cron job)
CREATE OR REPLACE FUNCTION cleanup_expired_token_blacklist()
RETURNS void AS $$
BEGIN
    DELETE FROM token_blacklist 
    WHERE token_expires_at < NOW();
END;
$$ LANGUAGE plpgsql;

-- Table for login attempt tracking (brute force protection)
CREATE TABLE IF NOT EXISTS login_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    ip_address INET NOT NULL,
    
    success BOOLEAN NOT NULL DEFAULT FALSE,
    reason VARCHAR(100),
    
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for efficient brute force detection
CREATE INDEX CONCURRENTLY idx_login_attempts_ip_time 
    ON login_attempts(ip_address, attempted_at);
CREATE INDEX CONCURRENTLY idx_login_attempts_email_time 
    ON login_attempts(email, attempted_at);
CREATE INDEX CONCURRENTLY idx_login_attempts_tenant 
    ON login_attempts(tenant_id);

-- Function to cleanup old login attempts (older than 30 days)
CREATE OR REPLACE FUNCTION cleanup_old_login_attempts()
RETURNS void AS $$
BEGIN
    DELETE FROM login_attempts 
    WHERE attempted_at < NOW() - INTERVAL '30 days';
END;
$$ LANGUAGE plpgsql;
