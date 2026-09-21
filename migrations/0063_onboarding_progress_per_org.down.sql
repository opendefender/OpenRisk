-- Reverse of 0063. LOSSY: once a user has a row in two organizations, the
-- per-user unique index cannot be rebuilt without deleting all but one. The most
-- recently updated row is kept; the others are removed, and those members will
-- see the wizard again in the organizations whose row was dropped — the
-- behaviour before 0063.

BEGIN;

DELETE FROM onboarding_progress op
 WHERE EXISTS (
       SELECT 1 FROM onboarding_progress keep
        WHERE keep.user_id = op.user_id
          AND (keep.updated_at, keep.id) > (op.updated_at, op.id));

DROP INDEX IF EXISTS uq_onboarding_progress_tenant_user;

CREATE UNIQUE INDEX IF NOT EXISTS idx_onboarding_progress_user_id
    ON onboarding_progress (user_id);

COMMIT;
