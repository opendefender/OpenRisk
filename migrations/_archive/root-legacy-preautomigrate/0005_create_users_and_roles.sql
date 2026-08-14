-- Migration: Create users and roles tables for RBAC
-- Purpose: Enable role-based access control with simple role assignments
-- Date: 2025-12-06

CREATE TABLE IF NOT EXISTS roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) NOT NULL UNIQUE,
    description TEXT,
    permissions TEXT[], -- JSON array of permission strings
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Pre-populate standard roles
INSERT INTO roles (name, description, permissions) VALUES
    ('admin', 'Full system access', ARRAY['*']),
    ('analyst', 'Can create, read, update risks and mitigations', ARRAY['risk:read', 'risk:create', 'risk:update', 'mitigation:read', 'mitigation:create', 'mitigation:update', 'asset:read']),
    ('viewer', 'Read-only access to risks and mitigations', ARRAY['risk:read', 'mitigation:read', 'asset:read'])
ON CONFLICT (name) DO NOTHING;

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    username VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255),
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    last_login TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Create indexes for common queries
CREATE INDEX idx_users_email ON users(email) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_username ON users(username) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_role_id ON users(role_id);
CREATE INDEX idx_roles_name ON roles(name);

-- Add comment for clarity
COMMENT ON TABLE roles IS 'Role definitions with permission sets for RBAC';
COMMENT ON TABLE users IS 'User accounts with assigned roles and authentication data';
COMMENT ON COLUMN users.password_hash IS 'Bcrypt hash of user password';
COMMENT ON COLUMN users.role_id IS 'Reference to user role for authorization checks';
