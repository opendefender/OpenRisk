-- W0-04 — Organization member management.
--
-- Two changes:
--   1. organization_members gains an explicit lifecycle (status + the two
--      timestamps that say when it changed), backfilled from the is_active
--      boolean that used to be the only signal.
--   2. invitations is rebuilt around a hashed bearer token. The previous shape
--      stored a UUID token in clear; it was never in AutoMigrate, so no
--      deployment has rows in it, and it is dropped rather than migrated —
--      there is no safe way to convert a plaintext credential into a hashed one.

BEGIN;

-- 1. Membership lifecycle -----------------------------------------------------

ALTER TABLE organization_members
    ADD COLUMN IF NOT EXISTS status          VARCHAR(16),
    ADD COLUMN IF NOT EXISTS deactivated_at  TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS revoked_at      TIMESTAMPTZ;

-- Backfill from the boolean that was the only state until now. Rows are read
-- through EffectiveStatus() which falls back to is_active, so this is about
-- making the column queryable, not about correctness of existing rows.
UPDATE organization_members
   SET status = CASE WHEN is_active THEN 'active' ELSE 'deactivated' END
 WHERE status IS NULL;

ALTER TABLE organization_members
    ALTER COLUMN status SET DEFAULT 'active';

CREATE INDEX IF NOT EXISTS idx_org_members_status
    ON organization_members (organization_id, status);

-- A user has at most one membership per organization. Without this, two
-- concurrent invitation acceptances both insert and the member ends up with
-- two rows and two roles.
CREATE UNIQUE INDEX IF NOT EXISTS uq_org_members_org_user
    ON organization_members (organization_id, user_id);

-- 2. Invitations --------------------------------------------------------------

DROP TABLE IF EXISTS invitations;

CREATE TABLE invitations (
    id               UUID PRIMARY KEY,
    organization_id  UUID        NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    email            VARCHAR(320) NOT NULL,
    role             VARCHAR(16) NOT NULL,
    business_role    VARCHAR(64),
    -- SHA-256 of the bearer token, hex. The plaintext is never stored: it exists
    -- once, in the response to the admin who created it and in the invitee's
    -- email, and a database dump cannot be replayed into memberships.
    token_hash       VARCHAR(64) NOT NULL,
    status           VARCHAR(16) NOT NULL DEFAULT 'pending',
    expires_at       TIMESTAMPTZ NOT NULL,
    invited_by_id    UUID        NOT NULL,
    accepted_at      TIMESTAMPTZ,
    accepted_by_id   UUID,
    revoked_at       TIMESTAMPTZ,
    revoked_by_id    UUID,
    last_sent_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    send_count       INTEGER     NOT NULL DEFAULT 1,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX uq_invitations_token_hash ON invitations (token_hash);
CREATE INDEX idx_invitations_org_status       ON invitations (organization_id, status);
CREATE INDEX idx_invitations_email            ON invitations (organization_id, email);
CREATE INDEX idx_invitations_expires          ON invitations (expires_at);

-- At most one PENDING invitation per (organization, email). This is the
-- database half of the duplicate check: two admins inviting the same person at
-- the same moment both pass the application-level lookup, and one of them has
-- to lose here.
CREATE UNIQUE INDEX uq_invitations_pending_email
    ON invitations (organization_id, email)
 WHERE status = 'pending';

COMMIT;
