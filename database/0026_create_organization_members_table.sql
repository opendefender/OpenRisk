-- +migrate Up
-- Create organization_members table for multi-tenant RBAC
CREATE TABLE IF NOT EXISTS organization_members (
    user_id UUID NOT NULL,
    organization_id UUID NOT NULL,
    role VARCHAR(255) NOT NULL DEFAULT 'viewer',
    permissions_override JSONB DEFAULT '{}'::jsonb,
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (user_id, organization_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE
);

-- Create indexes for organization_members
CREATE INDEX IF NOT EXISTS idx_organization_members_user_id ON organization_members(user_id);
CREATE INDEX IF NOT EXISTS idx_organization_members_organization_id ON organization_members(organization_id);
CREATE INDEX IF NOT EXISTS idx_organization_members_role ON organization_members(role);

-- +migrate Down
DROP TABLE IF EXISTS organization_members;