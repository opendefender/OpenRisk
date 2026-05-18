-- Migration: 0027_create_mitigations_comprehensive.down.sql
-- Purpose: Rollback migration 0027

DROP INDEX IF EXISTS idx_subactions_mitigation_order;
DROP INDEX IF EXISTS idx_subactions_mitigation_completed;
DROP INDEX IF EXISTS idx_subactions_completed_by;
DROP INDEX IF EXISTS idx_subactions_deleted_at;
DROP INDEX IF EXISTS idx_subactions_depends_on;
DROP INDEX IF EXISTS idx_subactions_completed;
DROP INDEX IF EXISTS idx_subactions_mitigation_id;

DROP TABLE IF EXISTS mitigation_subactions;

DROP INDEX IF EXISTS idx_mitigations_risk_status;
DROP INDEX IF EXISTS idx_mitigations_tenant_priority;
DROP INDEX IF EXISTS idx_mitigations_tenant_status;
DROP INDEX IF EXISTS idx_mitigations_deleted_at;
DROP INDEX IF EXISTS idx_mitigations_source;
DROP INDEX IF EXISTS idx_mitigations_created_by;
DROP INDEX IF EXISTS idx_mitigations_priority;
DROP INDEX IF EXISTS idx_mitigations_status;
DROP INDEX IF EXISTS idx_mitigations_risk_id;
DROP INDEX IF EXISTS idx_mitigations_tenant_id;

DROP TABLE IF EXISTS mitigations;
