-- Migration: Add refresh tokens and JWT blacklist tables for RS256 auth
-- Date: 2026-04-20
-- Purpose: Support token rotation, revocation, and secure refresh token storage

-- ============================================================================
-- UP: Create tables
-- ============================================================================

-- Table: refresh_tokens
-- Stores opaque refresh tokens (32-byte random hex) with rotation support
-- Allows invalidating old tokens when new pair is generated
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id     UUID NOT NULL,
    token_hash          VARCHAR(64) NOT NULL UNIQUE,  -- SHA256 hex of token
    device_fingerprint  VARCHAR(255),                  -- Optional: user-agent hash
    expires_at          TIMESTAMPTZ NOT NULL,
    last_used_at        TIMESTAMPTZ,
    revoked_at          TIMESTAMPTZ,                   -- Soft delete for refresh token
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index: Find non-revoked tokens by user
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_refresh_tokens_user_active
    ON refresh_tokens(user_id) 
    WHERE revoked_at IS NULL AND expires_at > NOW();

-- Index: Find token by hash for validation
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_refresh_tokens_hash
    ON refresh_tokens(token_hash);

-- Index: Find expired tokens for cleanup
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_refresh_tokens_expires
    ON refresh_tokens(expires_at);

-- Index: Find by organization (multi-tenancy)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_refresh_tokens_org
    ON refresh_tokens(organization_id);

-- Table: token_blacklist
-- Stores revoked JWT JTIs (JWT ID) to prevent token reuse after logout
-- Redis is primary store, this is backup for cross-instance consistency
CREATE TABLE IF NOT EXISTS token_blacklist (
    jti                 VARCHAR(255) PRIMARY KEY,      -- JWT ID claim
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at          TIMESTAMPTZ NOT NULL,          -- When JTI naturally expires
    revoked_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reason              VARCHAR(255)                   -- "logout", "password_change", etc.
);

-- Index: Find by user (audit trail)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_token_blacklist_user
    ON token_blacklist(user_id);

-- Index: Cleanup expired entries
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_token_blacklist_expires
    ON token_blacklist(expires_at);

-- ============================================================================
-- DOWN: Drop tables (rollback)
-- ============================================================================
-- DROP TABLE IF EXISTS token_blacklist CASCADE;
-- DROP TABLE IF EXISTS refresh_tokens CASCADE;
-- DROP INDEX IF EXISTS idx_token_blacklist_expires CASCADE;
-- DROP INDEX IF EXISTS idx_token_blacklist_user CASCADE;
-- DROP INDEX IF EXISTS idx_refresh_tokens_org CASCADE;
-- DROP INDEX IF EXISTS idx_refresh_tokens_expires CASCADE;
-- DROP INDEX IF EXISTS idx_refresh_tokens_hash CASCADE;
-- DROP INDEX IF EXISTS idx_refresh_tokens_user_active CASCADE;
