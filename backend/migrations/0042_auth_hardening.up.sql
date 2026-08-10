-- Copyright (c) 2026 OpenDefender Contributors
-- SPDX-License-Identifier: AGPL-3.0-only
--
-- Auth hardening: password reset tokens + session device provenance.

-- ---------------------------------------------------------------------------
-- Password reset
-- ---------------------------------------------------------------------------
--
-- A row is written for EVERY reset request, including ones naming an address
-- with no account — which is why user_id and token_hash are nullable. That is
-- what keeps the rate limiter from becoming an account-existence oracle: if only
-- real accounts were counted, a fourth request would start returning 429 for an
-- address that exists and never for one that does not.
--
-- email_hash is SHA-256 of the normalised address rather than the address
-- itself, so this table is not a harvestable list of who has an account (nor, for
-- unknown addresses, a log of what was probed for). It only ever needs equality.
CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email_hash         VARCHAR(64) NOT NULL,
    user_id            UUID NULL,
    token_hash         VARCHAR(64) NULL,
    expires_at         TIMESTAMPTZ NOT NULL,
    used_at            TIMESTAMPTZ NULL,
    request_ip         VARCHAR(64),
    request_user_agent VARCHAR(512),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Unique so a token can never be issued twice; the single-use claim is a
-- conditional UPDATE on used_at, and this index is what that lookup rides on.
CREATE UNIQUE INDEX IF NOT EXISTS idx_password_reset_tokens_token_hash
    ON password_reset_tokens (token_hash)
    WHERE token_hash IS NOT NULL;

-- The rate-limit query: count rows for an address inside the last hour.
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_email_window
    ON password_reset_tokens (email_hash, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id
    ON password_reset_tokens (user_id)
    WHERE user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_expires_at
    ON password_reset_tokens (expires_at);

-- ---------------------------------------------------------------------------
-- Session device provenance
-- ---------------------------------------------------------------------------
--
-- refresh_tokens already IS the session table. These columns make the device
-- list in Settings something a person can act on: "Chrome on macOS, 41.x.x.x,
-- last used 2 hours ago" is recognisable, an opaque UUID is not.
ALTER TABLE refresh_tokens ADD COLUMN IF NOT EXISTS ip_address VARCHAR(64);
ALTER TABLE refresh_tokens ADD COLUMN IF NOT EXISTS user_agent VARCHAR(512);

-- Supports HasSeenDevice, the lookup behind the new-device sign-in alert.
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_device
    ON refresh_tokens (user_id, device_fingerprint);
