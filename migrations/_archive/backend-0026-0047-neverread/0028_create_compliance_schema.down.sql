-- Migration: 0028_create_compliance_schema.down.sql
-- Purpose: Rollback migration 0028 (compliance foundation)

-- Drop indices for control_evidences
DROP INDEX IF EXISTS idx_control_evidences_tenant_control;
DROP INDEX IF EXISTS idx_control_evidences_deleted_at;
DROP INDEX IF EXISTS idx_control_evidences_control_id;
DROP INDEX IF EXISTS idx_control_evidences_tenant_id;

DROP TABLE IF EXISTS control_evidences;

-- Drop indices for compliance_controls
DROP INDEX IF EXISTS idx_compliance_controls_tenant_fw_ref;
DROP INDEX IF EXISTS idx_compliance_controls_tenant_framework;
DROP INDEX IF EXISTS idx_compliance_controls_deleted_at;
DROP INDEX IF EXISTS idx_compliance_controls_status;
DROP INDEX IF EXISTS idx_compliance_controls_framework_id;
DROP INDEX IF EXISTS idx_compliance_controls_tenant_id;

DROP TABLE IF EXISTS compliance_controls;

-- Drop indices for compliance_frameworks
DROP INDEX IF EXISTS idx_compliance_frameworks_deleted_at;
DROP INDEX IF EXISTS idx_compliance_frameworks_name_version;

DROP TABLE IF EXISTS compliance_frameworks;
