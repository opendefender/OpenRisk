-- Migration: 0030_framework_tenant_scope.down.sql
-- Revert compliance_frameworks to a global (non-tenant-scoped) entity.

DROP INDEX IF EXISTS idx_compliance_frameworks_tenant_name_version;
DROP INDEX IF EXISTS idx_compliance_frameworks_tenant_id;

CREATE UNIQUE INDEX IF NOT EXISTS idx_compliance_frameworks_name_version
    ON compliance_frameworks (name, version)
    WHERE deleted_at IS NULL;

ALTER TABLE compliance_frameworks
    DROP COLUMN IF EXISTS tenant_id;
