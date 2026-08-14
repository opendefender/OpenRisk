-- Migration: 0031_add_risk_lifecycle_phase.down.sql
-- Revert the ISO 31000 lifecycle phase column on risks.

DROP INDEX IF EXISTS idx_risks_lifecycle_phase;

ALTER TABLE risks
    DROP COLUMN IF EXISTS lifecycle_phase;
