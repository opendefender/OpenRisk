-- Reverses 0067's constraint and index only. The column is NOT dropped: the
-- application of this release reads it on every PAT request, and dropping it
-- would make every token unattributable on a later re-upgrade.

DO $$
BEGIN
    IF to_regclass('personal_access_tokens') IS NOT NULL THEN
        ALTER TABLE personal_access_tokens ALTER COLUMN tenant_id DROP NOT NULL;
    END IF;
END $$;

DROP INDEX IF EXISTS idx_personal_access_tokens_tenant_id;
