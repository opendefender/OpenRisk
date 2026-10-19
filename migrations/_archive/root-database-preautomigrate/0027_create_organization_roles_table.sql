-- +migrate Up
-- Create organization_roles table for custom IAM roles
CREATE TABLE IF NOT EXISTS organization_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    permissions JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
    UNIQUE (organization_id, name)
);

-- Create indexes for organization_roles
CREATE INDEX IF NOT EXISTS idx_organization_roles_organization_id ON organization_roles(organization_id);
CREATE INDEX IF NOT EXISTS idx_organization_roles_name ON organization_roles(name);
CREATE INDEX IF NOT EXISTS idx_organization_roles_is_active ON organization_roles(is_active);

-- +migrate Down
DROP TABLE IF EXISTS organization_roles;