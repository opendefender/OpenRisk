-- Reverses 0060. Dropping mfa_grace_started_at restores the pre-OR26-03
-- behaviour, where a privileged account with no authenticator was refused a
-- session outright rather than given a window.

BEGIN;

ALTER TABLE organization_members
    DROP COLUMN IF EXISTS mfa_grace_started_at;

DROP INDEX IF EXISTS uq_mfa_policies_tenant;
DROP TABLE IF EXISTS mfa_policies;

COMMIT;
