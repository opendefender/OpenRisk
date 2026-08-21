-- Reverses 0058. Invitations are dropped, not restored to their previous
-- shape: the old table stored plaintext tokens and a hashed token cannot be
-- turned back into one.

BEGIN;

DROP TABLE IF EXISTS invitations;

DROP INDEX IF EXISTS uq_org_members_org_user;
DROP INDEX IF EXISTS idx_org_members_status;

ALTER TABLE organization_members
    DROP COLUMN IF EXISTS revoked_at,
    DROP COLUMN IF EXISTS deactivated_at,
    DROP COLUMN IF EXISTS status;

COMMIT;
