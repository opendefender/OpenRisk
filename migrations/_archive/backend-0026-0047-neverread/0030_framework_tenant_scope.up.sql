-- Migration: 0030_framework_tenant_scope.up.sql
-- Purpose: make compliance_frameworks TENANT-SCOPED. Previously frameworks were
-- global (no tenant_id), so one tenant creating/deleting a framework affected
-- every tenant. Each tenant now owns its own frameworks; uniqueness of
-- (name, version) is enforced per tenant instead of globally.

ALTER TABLE compliance_frameworks
    ADD COLUMN IF NOT EXISTS tenant_id UUID;

-- Backfill: assign each previously-global framework to the tenant that owns its
-- controls. (In practice every framework was used by a single tenant.)
UPDATE compliance_frameworks f
SET tenant_id = c.tenant_id
FROM (
    SELECT DISTINCT ON (framework_id) framework_id, tenant_id
    FROM compliance_controls
    WHERE deleted_at IS NULL
    ORDER BY framework_id, tenant_id
) c
WHERE f.tenant_id IS NULL AND f.id = c.framework_id;

-- Replace the old GLOBAL uniqueness with PER-TENANT uniqueness.
DROP INDEX IF EXISTS idx_compliance_frameworks_name_version;
CREATE UNIQUE INDEX IF NOT EXISTS idx_compliance_frameworks_tenant_name_version
    ON compliance_frameworks (tenant_id, name, version)
    WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_compliance_frameworks_tenant_id
    ON compliance_frameworks (tenant_id);

-- NOTE: NOT NULL is intentionally not forced here — a framework with no controls
-- has nothing to backfill from and would break the migration. Any such straggler
-- must be assigned a tenant (or deleted) before a follow-up migration adds the
-- NOT NULL constraint. New rows always set tenant_id (enforced in the repository).
