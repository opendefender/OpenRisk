-- Migration: 0034_add_risk_financial_quantification.down.sql
-- Reverts 0034 by dropping the financial-quantification driver columns.

ALTER TABLE risks
    DROP COLUMN IF EXISTS downtime_hours,
    DROP COLUMN IF EXISTS hourly_downtime_cost_xaf,
    DROP COLUMN IF EXISTS data_loss_cost_xaf,
    DROP COLUMN IF EXISTS fines_xaf,
    DROP COLUMN IF EXISTS other_direct_cost_xaf,
    DROP COLUMN IF EXISTS remediation_cost_xaf,
    DROP COLUMN IF EXISTS mitigation_effectiveness;
