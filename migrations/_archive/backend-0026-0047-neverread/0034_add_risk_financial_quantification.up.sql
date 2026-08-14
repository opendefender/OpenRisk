-- Migration: 0034_add_risk_financial_quantification.up.sql
-- Purpose: give the Risk entity the full set of financial-quantification drivers
-- required by spec §9 "Quantification financière". The existing sle_xaf / aro
-- columns (migration set / AutoMigrate) already carry the ALE = SLE × ARO core;
-- this adds the components a composed SLE, a worst/average loss band and ROSI are
-- built from.
--
-- GORM's AutoMigrate already adds these columns (Risk is in AutoMigrate). This
-- migration exists so a migrations-only deploy is self-sufficient. All amounts
-- are XAF (FCFA); USD is derived on read via pkg/crq. Every column is nullable —
-- an unquantified risk simply falls back to the reference model.

ALTER TABLE risks
    ADD COLUMN IF NOT EXISTS downtime_hours            NUMERIC(10,2),
    ADD COLUMN IF NOT EXISTS hourly_downtime_cost_xaf  NUMERIC(16,2),
    ADD COLUMN IF NOT EXISTS data_loss_cost_xaf        NUMERIC(16,2),
    ADD COLUMN IF NOT EXISTS fines_xaf                 NUMERIC(16,2),
    ADD COLUMN IF NOT EXISTS other_direct_cost_xaf     NUMERIC(16,2),
    ADD COLUMN IF NOT EXISTS remediation_cost_xaf      NUMERIC(16,2),
    ADD COLUMN IF NOT EXISTS mitigation_effectiveness  NUMERIC(5,4);
