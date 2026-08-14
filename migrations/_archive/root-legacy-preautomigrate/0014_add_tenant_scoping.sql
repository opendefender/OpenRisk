-- Migration 0011: Add Tenant Scoping to Existing Tables
-- Ensures all major entities are scoped to tenants

-- Add tenant_id to risks table
ALTER TABLE risks ADD COLUMN IF NOT EXISTS tenant_id UUID;
ALTER TABLE risks ADD CONSTRAINT fk_risks_tenant_id FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_risks_tenant_id ON risks(tenant_id);

-- Add tenant_id to mitigations table
ALTER TABLE mitigations ADD COLUMN IF NOT EXISTS tenant_id UUID;
ALTER TABLE mitigations ADD CONSTRAINT fk_mitigations_tenant_id FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_mitigations_tenant_id ON mitigations(tenant_id);

-- Add tenant_id to assets table
ALTER TABLE assets ADD COLUMN IF NOT EXISTS tenant_id UUID;
ALTER TABLE assets ADD CONSTRAINT fk_assets_tenant_id FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_assets_tenant_id ON assets(tenant_id);

-- Add tenant_id to custom_fields table
ALTER TABLE custom_fields ADD COLUMN IF NOT EXISTS tenant_id UUID;
ALTER TABLE custom_fields ADD CONSTRAINT fk_custom_fields_tenant_id FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_custom_fields_tenant_id ON custom_fields(tenant_id);

-- Add tenant_id to audit_logs table (if exists)
ALTER TABLE IF EXISTS audit_logs ADD COLUMN IF NOT EXISTS tenant_id UUID;
ALTER TABLE IF EXISTS audit_logs ADD CONSTRAINT fk_audit_logs_tenant_id FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_audit_logs_tenant_id ON audit_logs(tenant_id);

-- Ensure backward compatibility: set deleted_at trigger for soft deletes
CREATE OR REPLACE FUNCTION soft_delete_check()
RETURNS TRIGGER AS $$
BEGIN
  IF NEW.deleted_at IS NOT NULL THEN
    NEW.is_deleted = true;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
