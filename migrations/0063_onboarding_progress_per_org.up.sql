-- #735 — onboarding progress is one row per (organization, user), not per user.
--
-- The table arrives through AutoMigrate, the declared schema authority
-- (internal/migrations/migrator.go), which adds the composite index from the
-- model tag but never drops an index the model no longer declares. This file
-- drops the old per-user unique index; internal/infrastructure/database/
-- prepare.go does the same at boot for deployments whose chain is behind.
--
-- WHY IT MATTERS. An account belongs to every organization that invited it.
-- With user_id unique, its single progress row moved to whichever organization
-- last wrote it, so a member of two organizations was sent back to the signup
-- wizard on every switch, and finishing it in one re-opened it in the other.
--
-- No row is rewritten: rows unique on user_id are unique on (tenant_id, user_id).

BEGIN;

DROP INDEX IF EXISTS idx_onboarding_progress_user_id;

CREATE UNIQUE INDEX IF NOT EXISTS uq_onboarding_progress_tenant_user
    ON onboarding_progress (tenant_id, user_id);

COMMIT;
