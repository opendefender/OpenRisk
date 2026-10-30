-- #612 — new organisations are Free, and the legacy 'starter' value is retired.
--
-- organizations.plan defaulted to 'starter', which the entitlement resolver
-- reads as the legacy name of Pro: every organisation created without a plan
-- (sign-up, seed admin, SSO personal org) resolved to Pro caps with no
-- subscription. No customer holds a deliberate 'starter' plan (owner,
-- 2026-10-29: still in development), so every such row becomes Free.
-- A subscription, when present, stays the source of truth over this column.
--
-- The same backfill runs at boot in PrepareForAutoMigrate, for deployments that
-- skip this SQL layer.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'organizations') THEN
        ALTER TABLE organizations ALTER COLUMN plan SET DEFAULT 'free';
        UPDATE organizations SET plan = 'free' WHERE plan = 'starter';
    END IF;
END $$;
