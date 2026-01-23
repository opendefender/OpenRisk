-- Migration 0010: Create User-Tenant Junction Table
-- Supports users belonging to multiple tenants with different roles

CREATE TABLE IF NOT EXISTS user_tenants (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, tenant_id),
  INDEX idx_user_tenants_tenant_id (tenant_id),
  INDEX idx_user_tenants_role_id (role_id)
);

-- Trigger for user_tenants updated_at
CREATE TRIGGER update_user_tenants_updated_at
  BEFORE UPDATE ON user_tenants
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at_column();
