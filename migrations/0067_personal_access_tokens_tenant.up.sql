-- #782 — a personal access token belongs to one organization.
--
-- Before this, the PAT middleware resolved a token's tenant from its owner's
-- default organization on every request, so changing that default silently
-- moved every token the person held. The tenant is now stored on the token and
-- the middleware resolves the session for that tenant only.
--
-- The backfill copies users.default_org_id: the tenant each token already acted
-- in, so no token changes behaviour. A token whose owner has no default
-- organization never authenticated a single request, and is deleted rather
-- than pinned to a guessed tenant.
--
-- A server boot builds this table with AutoMigrate and runs the same backfill
-- first (database.backfillPersonalAccessTokenTenant). Every statement below is
-- therefore idempotent.

-- No SQL migration creates personal_access_tokens: only AutoMigrate does. Run
-- before the first boot, this migration finds no table and does nothing; the
-- boot then creates the table with the column already in place.

DO $$
BEGIN
    IF to_regclass('personal_access_tokens') IS NULL THEN
        RETURN;
    END IF;

    ALTER TABLE personal_access_tokens ADD COLUMN IF NOT EXISTS tenant_id uuid;

    UPDATE personal_access_tokens p
       SET tenant_id = u.default_org_id
      FROM users u
     WHERE p.user_id = u.id
       AND p.tenant_id IS NULL
       AND u.default_org_id IS NOT NULL;

    DELETE FROM personal_access_tokens WHERE tenant_id IS NULL;

    ALTER TABLE personal_access_tokens ALTER COLUMN tenant_id SET NOT NULL;

    CREATE INDEX IF NOT EXISTS idx_personal_access_tokens_tenant_id
        ON personal_access_tokens (tenant_id);
END $$;
