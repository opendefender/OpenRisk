-- #349 — install the mfa_policies guarantees 0060 declared but never applied.
--
-- A server boot runs AutoMigrate before the SQL layer, and AutoMigrate already
-- creates mfa_policies from domain.MFAPolicy. 0060's CREATE TABLE IF NOT EXISTS
-- is therefore a no-op on every booted database, and three things it declares
-- only exist where the migration CLI ran before the first boot:
--   - the CHECK that keeps grace_days within 0–90, so a direct SQL edit cannot
--     express "never require MFA";
--   - the grace_days default (7), for raw inserts;
--   - the id default, for raw inserts.
-- Each is added where it is missing and left alone where it already exists.

BEGIN;

-- The application reads an out-of-range value as its clamped value
-- (MFAPolicy.EffectiveGraceDays), so storing the clamped value changes no
-- decision. Without it, the CHECK below would abort the boot on a row the
-- application already treats as legal.
UPDATE mfa_policies
   SET grace_days = LEAST(GREATEST(grace_days, 0), 90)
 WHERE grace_days NOT BETWEEN 0 AND 90;

ALTER TABLE mfa_policies ALTER COLUMN grace_days SET DEFAULT 7;
ALTER TABLE mfa_policies ALTER COLUMN id SET DEFAULT gen_random_uuid();

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'mfa_policies'::regclass
           AND conname = 'mfa_policies_grace_days_bounds'
    ) THEN
        ALTER TABLE mfa_policies
            ADD CONSTRAINT mfa_policies_grace_days_bounds CHECK (grace_days BETWEEN 0 AND 90);
    END IF;
END $$;

COMMIT;
