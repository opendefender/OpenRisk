-- Migration 0012: Seed Default Roles and Permissions
-- Populates the RBAC system with standard roles and permissions

-- Insert system permissions
INSERT INTO permissions (resource, action, description, is_system) VALUES
-- Risk permissions
('risk', 'read', 'View risks', true),
('risk', 'create', 'Create new risks', true),
('risk', 'update', 'Update existing risks', true),
('risk', 'delete', 'Delete risks', true),
('risk', 'export', 'Export risk data', true),
('risk', 'admin', 'Administer risk settings', true),

-- Mitigation permissions
('mitigation', 'read', 'View mitigations', true),
('mitigation', 'create', 'Create mitigations', true),
('mitigation', 'update', 'Update mitigations', true),
('mitigation', 'delete', 'Delete mitigations', true),
('mitigation', 'export', 'Export mitigation data', true),
('mitigation', 'admin', 'Administer mitigation settings', true),

-- User permissions
('user', 'read', 'View user information', true),
('user', 'create', 'Create new users', true),
('user', 'update', 'Update user information', true),
('user', 'delete', 'Delete users', true),
('user', 'admin', 'Administer users', true),

-- Report permissions
('report', 'read', 'View reports', true),
('report', 'create', 'Create reports', true),
('report', 'update', 'Update reports', true),
('report', 'delete', 'Delete reports', true),
('report', 'export', 'Export reports', true),
('report', 'admin', 'Administer reports', true),

-- Integration permissions
('integration', 'read', 'View integrations', true),
('integration', 'create', 'Configure integrations', true),
('integration', 'update', 'Update integrations', true),
('integration', 'delete', 'Remove integrations', true),
('integration', 'admin', 'Administer integrations', true),

-- Audit permissions
('audit', 'read', 'View audit logs', true),
('audit', 'admin', 'Administer audit logs', true),

-- Asset permissions
('asset', 'read', 'View assets', true),
('asset', 'create', 'Create assets', true),
('asset', 'update', 'Update assets', true),
('asset', 'delete', 'Delete assets', true),
('asset', 'admin', 'Administer assets', true),

-- Connector permissions
('connector', 'read', 'View connectors', true),
('connector', 'create', 'Install connectors', true),
('connector', 'update', 'Update connectors', true),
('connector', 'delete', 'Uninstall connectors', true),
('connector', 'admin', 'Administer connectors', true)
ON CONFLICT DO NOTHING;

-- Create default system tenant (for backward compatibility)
INSERT INTO tenants (id, name, slug, owner_id, status)
SELECT
  '00000000-0000-0000-0000-000000000001'::uuid,
  'Default Tenant',
  'default',
  id,
  'active'
FROM users LIMIT 1
ON CONFLICT DO NOTHING;

-- Function to create predefined roles (will be called in code)
-- This demonstrates the role structure
/*
ADMIN (Level 9):
- All risk permissions
- All mitigation permissions
- All user permissions
- All report permissions
- All integration permissions
- All audit permissions
- All asset permissions
- All connector permissions

MANAGER (Level 6):
- Risk: read, create, update, export
- Mitigation: read, create, update, export
- Report: read, create, update, export
- Audit: read
- Asset: read, create, update
- Connector: read, update
- User: read

ANALYST (Level 3):
- Risk: read, create, update, export
- Mitigation: read, create, update, export
- Report: read, create, export
- Audit: read
- Asset: read, create, update
- Connector: read

VIEWER (Level 0):
- Risk: read
- Mitigation: read
- Report: read
- Audit: read
- Asset: read
- Connector: read
*/

-- Ensure backward compatibility marker
INSERT INTO custom_fields (id, name, type, is_system)
VALUES ('00000000-0000-0000-0000-000000000002'::uuid, '_migration_0012_completed', 'system', true)
ON CONFLICT DO NOTHING;
