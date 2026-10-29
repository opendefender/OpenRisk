-- #1164 — what an agent scan looks for: a fast network discovery or the full
-- vulnerability scan. Every scan before this ran the vulnerability command, so
-- existing rows take that value. Additive.
--
-- No SQL migration creates scan_jobs: only AutoMigrate does. Run before the
-- first boot, this migration finds no table and does nothing; the boot then
-- creates the table with the column in place. A boot without DATABASE_URL adds
-- the column in database.prepareScanJobType instead.

DO $$
BEGIN
    IF to_regclass('scan_jobs') IS NULL THEN
        RETURN;
    END IF;

    ALTER TABLE scan_jobs ADD COLUMN IF NOT EXISTS scan_type VARCHAR(16) NOT NULL DEFAULT 'vulnerability';
END $$;
