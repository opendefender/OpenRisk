-- Migration 0008: Create Tenants Table
-- Adds multi-tenant support to the application

CREATE TABLE IF NOT EXISTS tenants (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name VARCHAR(255) NOT NULL,
  slug VARCHAR(255) NOT NULL UNIQUE,
  owner_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  status VARCHAR(50) NOT NULL DEFAULT 'active',
  is_active BOOLEAN NOT NULL DEFAULT true,
  metadata JSONB,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMP,
  INDEX idx_tenants_slug (slug),
  INDEX idx_tenants_status (status),
  INDEX idx_tenants_deleted_at (deleted_at)
);

-- Trigger to update updated_at
CREATE TRIGGER update_tenants_updated_at
  BEFORE UPDATE ON tenants
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at_column();

-- Add tenant_id to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS tenant_id UUID;
ALTER TABLE users ADD CONSTRAINT fk_users_tenant_id FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_users_tenant_id ON users(tenant_id);
