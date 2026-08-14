-- Roll back the activation / onboarding tables.
--
-- Dropping activation_events destroys the activation history (it is a log, not a
-- cache — nothing else can rebuild it), so this is deliberately a full drop
-- rather than a partial revert.

DROP INDEX IF EXISTS idx_onboarding_progress_tenant;
DROP INDEX IF EXISTS idx_onboarding_progress_user;
DROP TABLE IF EXISTS onboarding_progress;

DROP INDEX IF EXISTS idx_activation_celebrations_tenant;
DROP INDEX IF EXISTS idx_activation_celebration;
DROP TABLE IF EXISTS activation_celebrations;

DROP INDEX IF EXISTS idx_activation_events_occurred_at;
DROP INDEX IF EXISTS idx_activation_events_user;
DROP INDEX IF EXISTS idx_activation_tenant_key;
DROP TABLE IF EXISTS activation_events;
