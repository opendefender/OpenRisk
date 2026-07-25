-- Revert tenant scoping of custom fields + templates.
DROP INDEX IF EXISTS idx_tenant_name_scope;
DROP INDEX IF EXISTS idx_tenant_template_name;
DROP INDEX IF EXISTS idx_custom_fields_tenant;
DROP INDEX IF EXISTS idx_custom_field_templates_tenant;

CREATE UNIQUE INDEX IF NOT EXISTS idx_name_scope ON custom_fields (name, scope);

ALTER TABLE custom_fields          DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE custom_field_templates DROP COLUMN IF EXISTS tenant_id;
