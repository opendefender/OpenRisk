-- Tenant-scope custom fields + templates (isolation audit).
-- These tables previously had NO tenant column at all: a global unique index on
-- (name, scope) made every custom field visible/editable/deletable across ALL
-- tenants, and a "Department" field in one tenant blocked another from creating
-- one. Add tenant_id and rebuild the unique constraint to (tenant_id, name, scope)
-- so tenants define fields independently (RULE #2). Idempotent.

-- 1. Add the column (nullable first so the backfill can run on existing rows).
ALTER TABLE custom_fields          ADD COLUMN IF NOT EXISTS tenant_id uuid;
ALTER TABLE custom_field_templates ADD COLUMN IF NOT EXISTS tenant_id uuid;

-- 2. Backfill any pre-existing rows to their creator's primary organization when
--    resolvable; otherwise leave NULL (an orphaned global field no tenant will
--    ever match — safe by default). Dev environments rarely have rows here.
UPDATE custom_fields cf
SET tenant_id = om.organization_id
FROM organization_members om
WHERE cf.tenant_id IS NULL AND cf.created_by = om.user_id;

UPDATE custom_field_templates cft
SET tenant_id = om.organization_id
FROM organization_members om
WHERE cft.tenant_id IS NULL AND cft.created_by = om.user_id;

-- 3. Replace the global unique indexes with tenant-scoped ones.
DROP INDEX IF EXISTS idx_name_scope;
CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_name_scope
    ON custom_fields (tenant_id, name, scope) WHERE deleted_at IS NULL;

-- The templates table historically had a unique index on name; scope it per tenant.
DROP INDEX IF EXISTS idx_custom_field_templates_name;
CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_template_name
    ON custom_field_templates (tenant_id, name);

CREATE INDEX IF NOT EXISTS idx_custom_fields_tenant           ON custom_fields (tenant_id);
CREATE INDEX IF NOT EXISTS idx_custom_field_templates_tenant  ON custom_field_templates (tenant_id);
