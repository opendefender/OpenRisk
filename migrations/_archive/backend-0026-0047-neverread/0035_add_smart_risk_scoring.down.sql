-- Migration: 0035_add_smart_risk_scoring.down.sql
-- Reverts 0035 by dropping the smart-score columns from `risks` and the
-- per-tenant factor-weights table.

DROP INDEX IF EXISTS idx_risks_smart_score;

ALTER TABLE risks
    DROP COLUMN IF EXISTS smart_score,
    DROP COLUMN IF EXISTS smart_level,
    DROP COLUMN IF EXISTS smart_factors,
    DROP COLUMN IF EXISTS smart_computed_at;

DROP INDEX IF EXISTS idx_risk_scoring_weights_tenant;
DROP TABLE IF EXISTS risk_scoring_weights;
