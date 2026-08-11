-- Migration: 0045_risk_lifecycle_state.down.sql
-- Reverse 0045: drop the canonical lifecycle state.
--
-- status and lifecycle_phase are NOT restored to their pre-migration values:
-- 0045 deliberately repaired rows where the two disagreed, and re-introducing
-- that disagreement would be a regression, not a rollback. Both columns remain
-- populated and self-consistent after this runs.

DROP INDEX IF EXISTS idx_risks_lifecycle_state;

ALTER TABLE risks
    DROP COLUMN IF EXISTS lifecycle_state;
