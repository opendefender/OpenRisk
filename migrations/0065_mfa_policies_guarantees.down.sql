-- Reverses 0065: back to the mfa_policies a booted server had after 0060,
-- with no CHECK and no column defaults. Values clamped by the up migration
-- stay clamped; the application read them that way already.

BEGIN;

ALTER TABLE mfa_policies DROP CONSTRAINT IF EXISTS mfa_policies_grace_days_bounds;
ALTER TABLE mfa_policies ALTER COLUMN grace_days DROP DEFAULT;
ALTER TABLE mfa_policies ALTER COLUMN id DROP DEFAULT;

COMMIT;
