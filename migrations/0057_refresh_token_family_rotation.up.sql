-- W0-03 authentication hardening: refresh-token reuse detection.
--
-- Refresh tokens are single-use and rotated. To detect REUSE (a rotated token
-- replayed, or two requests racing to rotate the same token) we stop hard-
-- deleting a token on rotation and instead:
--   * group every rotation of one login into a family_id lineage, and
--   * stamp rotated_at when a token is consumed.
-- A replay of a token whose rotated_at is set means the lineage leaked, so the
-- whole family is revoked (see internal/auth.RefreshTokenPair).
--
-- Existing rows are live sessions: give them a fresh family each (their own id)
-- and leave rotated_at NULL so they keep working.

ALTER TABLE refresh_tokens ADD COLUMN IF NOT EXISTS family_id uuid;
ALTER TABLE refresh_tokens ADD COLUMN IF NOT EXISTS rotated_at timestamptz;

UPDATE refresh_tokens SET family_id = id WHERE family_id IS NULL;

ALTER TABLE refresh_tokens ALTER COLUMN family_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family_id ON refresh_tokens (family_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_rotated_at ON refresh_tokens (rotated_at);
