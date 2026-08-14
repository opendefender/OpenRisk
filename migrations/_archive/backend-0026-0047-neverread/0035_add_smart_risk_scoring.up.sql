-- Migration: 0035_add_smart_risk_scoring.up.sql
-- Purpose: back the Smart Risk Calculation (spec §8 "Calcul de risque
-- intelligent"). Two changes:
--   1. A per-tenant weights table for the eight factors of the multifactor model,
--      seeded with the engine defaults. Exactly one row per tenant (unique index).
--   2. The persisted smart-score columns on `risks` (the 0–100 multifactor score,
--      its criticality band, the frozen per-factor breakdown, and when it was
--      last computed).
--
-- GORM's AutoMigrate already creates both (RiskScoringWeights + Risk are in
-- AutoMigrate); this migration exists so a migrations-only deploy is self-sufficient.
-- The classic Score Engine columns (probability/impact/score/criticality) are
-- untouched — the smart score is additive.

CREATE TABLE IF NOT EXISTS risk_scoring_weights (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id             UUID NOT NULL,
    business_criticality  NUMERIC(6,4) NOT NULL DEFAULT 0.15,
    internet_exposure     NUMERIC(6,4) NOT NULL DEFAULT 0.10,
    vulnerabilities       NUMERIC(6,4) NOT NULL DEFAULT 0.20,
    control_maturity      NUMERIC(6,4) NOT NULL DEFAULT 0.10,
    incident_history      NUMERIC(6,4) NOT NULL DEFAULT 0.10,
    exploitability        NUMERIC(6,4) NOT NULL DEFAULT 0.15,
    financial_value       NUMERIC(6,4) NOT NULL DEFAULT 0.10,
    threat_intel          NUMERIC(6,4) NOT NULL DEFAULT 0.10,
    updated_by            UUID,
    created_at            TIMESTAMPTZ,
    updated_at            TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_risk_scoring_weights_tenant
    ON risk_scoring_weights (tenant_id);

ALTER TABLE risks
    ADD COLUMN IF NOT EXISTS smart_score       NUMERIC(5,2) DEFAULT 0,
    ADD COLUMN IF NOT EXISTS smart_level       VARCHAR(20),
    ADD COLUMN IF NOT EXISTS smart_factors     JSONB,
    ADD COLUMN IF NOT EXISTS smart_computed_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_risks_smart_score ON risks (smart_score);
