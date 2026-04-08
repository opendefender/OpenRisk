-- Migration 0014: Organizations & SaaS Feature System
-- Implements multi-organization support with user management and subscription tiers
-- Supports both on-premise and SaaS deployments with local-to-SaaS migration

-- Organizations Table
CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    website VARCHAR(255),
    logo_url VARCHAR(500),
    
    -- Organization settings
    country VARCHAR(2),
    industry VARCHAR(100),
    company_size VARCHAR(50),
    timezone VARCHAR(50) DEFAULT 'UTC',
    
    -- Subscription information
    subscription_tier VARCHAR(50) NOT NULL DEFAULT 'freemium',
    subscription_status VARCHAR(50) NOT NULL DEFAULT 'active',
    subscription_start_date TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    subscription_end_date TIMESTAMP,
    subscription_renewal_date TIMESTAMP,
    
    -- Billing information
    billing_email VARCHAR(255),
    billing_address JSONB,
    vat_number VARCHAR(50),
    
    -- Feature flags for SaaS tiers
    features JSONB DEFAULT '{
        "max_users": 1,
        "max_risks": 10,
        "advanced_analytics": false,
        "custom_reports": false,
        "api_access": false,
        "sso_enabled": false,
        "audit_logs": true,
        "data_export": false,
        "advanced_compliance": false,
        "custom_fields": false,
        "webhooks": false,
        "max_api_calls_per_month": 0
    }'::jsonb,
    
    -- Quotas
    current_user_count INT DEFAULT 0,
    current_risk_count INT DEFAULT 0,
    current_api_calls_month INT DEFAULT 0,
    
    -- Created/Updated metadata
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    
    -- Constraints
    CONSTRAINT chk_subscription_tier CHECK (subscription_tier IN ('freemium', 'pro', 'enterprise')),
    CONSTRAINT chk_subscription_status CHECK (subscription_status IN ('active', 'suspended', 'cancelled', 'trial')),
    
    -- Indexes
    INDEX idx_org_slug (slug),
    INDEX idx_org_subscription_tier (subscription_tier),
    INDEX idx_org_subscription_status (subscription_status),
    INDEX idx_org_created_at (created_at DESC)
);

-- Organization Members/Users Table
CREATE TABLE IF NOT EXISTS organization_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- Membership information
    role VARCHAR(50) NOT NULL DEFAULT 'member',
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    invitation_token VARCHAR(255),
    invitation_accepted_at TIMESTAMP,
    invitation_expires_at TIMESTAMP,
    
    -- Permissions override (can override role permissions)
    permissions_override JSONB,
    
    -- Metadata
    joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    
    -- Constraints
    CONSTRAINT chk_role CHECK (role IN ('owner', 'admin', 'manager', 'member', 'viewer')),
    CONSTRAINT chk_status CHECK (status IN ('active', 'pending', 'suspended', 'removed')),
    UNIQUE(organization_id, user_id),
    
    -- Indexes
    INDEX idx_org_members_org_id (organization_id),
    INDEX idx_org_members_user_id (user_id),
    INDEX idx_org_members_role (role),
    INDEX idx_org_members_status (status)
);

-- Organization Teams Table
CREATE TABLE IF NOT EXISTS organization_teams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    
    name VARCHAR(255) NOT NULL,
    description TEXT,
    
    -- Team settings
    max_members INT,
    default_role VARCHAR(50) DEFAULT 'viewer',
    
    -- Permissions
    permissions JSONB DEFAULT '[]'::jsonb,
    
    -- Metadata
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID NOT NULL REFERENCES users(id),
    deleted_at TIMESTAMP,
    
    -- Constraints
    CONSTRAINT chk_default_role CHECK (default_role IN ('owner', 'admin', 'manager', 'member', 'viewer')),
    
    -- Indexes
    INDEX idx_team_org_id (organization_id),
    INDEX idx_team_created_at (created_at DESC)
);

-- Team Members Table
CREATE TABLE IF NOT EXISTS team_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID NOT NULL REFERENCES organization_teams(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    role VARCHAR(50) NOT NULL DEFAULT 'member',
    added_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    added_by UUID NOT NULL REFERENCES users(id),
    
    CONSTRAINT chk_team_role CHECK (role IN ('lead', 'member', 'viewer')),
    UNIQUE(team_id, user_id),
    
    INDEX idx_team_member_team_id (team_id),
    INDEX idx_team_member_user_id (user_id)
);

-- SaaS Subscription Plans
CREATE TABLE IF NOT EXISTS subscription_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    plan_name VARCHAR(100) NOT NULL UNIQUE,
    tier VARCHAR(50) NOT NULL UNIQUE,
    description TEXT,
    
    -- Pricing
    monthly_price DECIMAL(10, 2) NOT NULL DEFAULT 0,
    annual_price DECIMAL(10, 2),
    currency VARCHAR(3) DEFAULT 'USD',
    
    -- Features
    features JSONB NOT NULL DEFAULT '{
        "max_users": 1,
        "max_risks": 10,
        "advanced_analytics": false,
        "custom_reports": false,
        "api_access": false,
        "sso_enabled": false,
        "audit_logs": true,
        "data_export": false,
        "advanced_compliance": false,
        "webhooks": false,
        "max_api_calls_per_month": 0
    }'::jsonb,
    
    -- Support
    support_level VARCHAR(50),
    support_email VARCHAR(255),
    support_hours VARCHAR(100),
    
    -- Terms
    trial_days INT DEFAULT 14,
    max_users INT,
    max_risks INT,
    max_api_calls_per_month INT,
    
    -- Status
    is_active BOOLEAN DEFAULT true,
    is_popular BOOLEAN DEFAULT false,
    
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT chk_tier CHECK (tier IN ('freemium', 'pro', 'enterprise')),
    
    INDEX idx_plan_tier (tier),
    INDEX idx_plan_active (is_active)
);

-- Organization Subscription History
CREATE TABLE IF NOT EXISTS organization_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    plan_id UUID NOT NULL REFERENCES subscription_plans(id),
    
    -- Subscription details
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    start_date TIMESTAMP NOT NULL,
    end_date TIMESTAMP,
    renewal_date TIMESTAMP,
    
    -- Billing details
    billing_cycle VARCHAR(50) NOT NULL DEFAULT 'monthly',
    auto_renew BOOLEAN DEFAULT true,
    
    -- Pricing (can differ from plan due to discounts)
    actual_price DECIMAL(10, 2),
    discount_applied DECIMAL(5, 2),
    discount_reason VARCHAR(255),
    
    -- Metadata
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT chk_sub_status CHECK (status IN ('active', 'suspended', 'cancelled', 'trial', 'expired')),
    CONSTRAINT chk_billing_cycle CHECK (billing_cycle IN ('monthly', 'annual', 'custom')),
    
    INDEX idx_sub_org_id (organization_id),
    INDEX idx_sub_plan_id (plan_id),
    INDEX idx_sub_status (status)
);

-- Data Migration Records (Local to SaaS)
CREATE TABLE IF NOT EXISTS data_migration_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Source & destination
    source_deployment_type VARCHAR(50) NOT NULL,  -- 'self-hosted', 'docker', 'kubernetes'
    source_database_version VARCHAR(50),
    source_data_size_bytes BIGINT,
    
    target_organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    target_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- Migration details
    migration_type VARCHAR(50) NOT NULL,  -- 'full', 'partial', 'backup'
    status VARCHAR(50) NOT NULL DEFAULT 'pending',  -- pending, in_progress, completed, failed, cancelled
    
    -- Progress tracking
    total_items INT,
    migrated_items INT DEFAULT 0,
    failed_items INT DEFAULT 0,
    skipped_items INT DEFAULT 0,
    
    -- Audit
    migration_log JSONB,
    error_details JSONB,
    validation_results JSONB,
    
    -- Metadata
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    estimated_completion TIMESTAMP,
    
    CONSTRAINT chk_migration_type CHECK (migration_type IN ('full', 'partial', 'backup')),
    CONSTRAINT chk_migration_status CHECK (status IN ('pending', 'in_progress', 'completed', 'failed', 'cancelled')),
    
    INDEX idx_migration_org_id (target_organization_id),
    INDEX idx_migration_status (status),
    INDEX idx_migration_created_at (created_at DESC)
);

-- Data Migration Items (Individual risks, assets, etc.)
CREATE TABLE IF NOT EXISTS migration_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    migration_job_id UUID NOT NULL REFERENCES data_migration_jobs(id) ON DELETE CASCADE,
    
    -- Item details
    item_type VARCHAR(50) NOT NULL,  -- 'risk', 'asset', 'mitigation', 'user', etc.
    source_id VARCHAR(255) NOT NULL,
    target_id UUID,
    
    -- Item data
    item_data JSONB NOT NULL,
    
    -- Migration status
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    error_message TEXT,
    attempted_at TIMESTAMP,
    
    CONSTRAINT chk_item_status CHECK (status IN ('pending', 'migrated', 'failed', 'skipped')),
    
    INDEX idx_item_migration_id (migration_job_id),
    INDEX idx_item_type (item_type),
    INDEX idx_item_status (status)
);

-- API Keys for Organizations
CREATE TABLE IF NOT EXISTS organization_api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) NOT NULL UNIQUE,
    
    -- Permissions
    scopes TEXT[] DEFAULT '{"read"}'::text[],
    allowed_ips INET[],
    
    -- Rate limiting
    rate_limit_per_minute INT DEFAULT 100,
    rate_limit_per_hour INT DEFAULT 10000,
    
    -- Usage tracking
    last_used_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP,
    revoked_at TIMESTAMP,
    
    INDEX idx_org_api_keys (organization_id),
    INDEX idx_api_key_hash (key_hash)
);

-- Organization Usage Tracking
CREATE TABLE IF NOT EXISTS organization_usage_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    
    -- Usage metrics
    api_calls_count INT DEFAULT 0,
    risks_created INT DEFAULT 0,
    risks_updated INT DEFAULT 0,
    users_invited INT DEFAULT 0,
    data_exported_bytes BIGINT DEFAULT 0,
    
    -- Storage
    storage_used_bytes BIGINT DEFAULT 0,
    backup_size_bytes BIGINT DEFAULT 0,
    
    -- Month/Year
    usage_month INT NOT NULL,
    usage_year INT NOT NULL,
    
    recorded_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(organization_id, usage_year, usage_month),
    
    INDEX idx_usage_org_id (organization_id),
    INDEX idx_usage_month_year (usage_year, usage_month)
);

-- Organization Audit Log
CREATE TABLE IF NOT EXISTS organization_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id),
    
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id VARCHAR(255),
    
    old_values JSONB,
    new_values JSONB,
    
    ip_address INET,
    user_agent TEXT,
    
    status VARCHAR(50) DEFAULT 'success',
    error_message TEXT,
    
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    INDEX idx_audit_org_id (organization_id),
    INDEX idx_audit_user_id (user_id),
    INDEX idx_audit_created_at (created_at DESC),
    INDEX idx_audit_action (action)
);

-- Organization Invitations
CREATE TABLE IF NOT EXISTS organization_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    
    invitee_email VARCHAR(255) NOT NULL,
    inviter_id UUID NOT NULL REFERENCES users(id),
    
    role VARCHAR(50) NOT NULL DEFAULT 'member',
    team_id UUID REFERENCES organization_teams(id),
    
    invitation_token VARCHAR(255) NOT NULL UNIQUE,
    expires_at TIMESTAMP NOT NULL,
    accepted_at TIMESTAMP,
    declined_at TIMESTAMP,
    
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT chk_invite_role CHECK (role IN ('owner', 'admin', 'manager', 'member', 'viewer')),
    
    INDEX idx_invite_org_id (organization_id),
    INDEX idx_invite_email (invitee_email),
    INDEX idx_invite_token (invitation_token),
    INDEX idx_invite_expires_at (expires_at)
);

-- Default Subscription Plans (Initial data)
INSERT INTO subscription_plans (plan_name, tier, description, monthly_price, annual_price, features, support_level, max_users, max_risks, max_api_calls_per_month)
VALUES
    ('Freemium', 'freemium', 'Free plan for individual users and small teams', 0, 0, 
     '{"max_users": 1, "max_risks": 10, "advanced_analytics": false, "custom_reports": false, "api_access": false, "sso_enabled": false, "audit_logs": true, "data_export": false, "advanced_compliance": false, "custom_fields": false, "webhooks": false, "max_api_calls_per_month": 100}'::jsonb,
     'community', 1, 10, 100),
     
    ('Professional', 'pro', 'Professional plan for growing teams', 29.99, 299.99,
     '{"max_users": 10, "max_risks": 1000, "advanced_analytics": true, "custom_reports": true, "api_access": true, "sso_enabled": false, "audit_logs": true, "data_export": true, "advanced_compliance": true, "custom_fields": true, "webhooks": true, "max_api_calls_per_month": 100000}'::jsonb,
     'email', 10, 1000, 100000),
     
    ('Enterprise', 'enterprise', 'Enterprise plan with dedicated support', 99.99, 999.99,
     '{"max_users": 1000, "max_risks": 100000, "advanced_analytics": true, "custom_reports": true, "api_access": true, "sso_enabled": true, "audit_logs": true, "data_export": true, "advanced_compliance": true, "custom_fields": true, "webhooks": true, "max_api_calls_per_month": 10000000}'::jsonb,
     'dedicated', 1000, 100000, 10000000);

-- Tenants table migration: Link tenants to organizations (backward compatibility)
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS subscription_tier VARCHAR(50) DEFAULT 'freemium';

-- Users table migration: Link to organization
ALTER TABLE users ADD COLUMN IF NOT EXISTS primary_organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMP;

-- Risks table migration: Link to organization
ALTER TABLE risks ADD COLUMN IF NOT EXISTS organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE;

-- Create index for efficient queries
CREATE INDEX IF NOT EXISTS idx_risks_org_id ON risks(organization_id);
CREATE INDEX IF NOT EXISTS idx_users_primary_org ON users(primary_organization_id);
CREATE INDEX IF NOT EXISTS idx_tenants_org_id ON tenants(organization_id);

-- Update existing records to reference organization via tenant
UPDATE risks SET organization_id = t.organization_id FROM tenants t WHERE risks.tenant_id = t.id;
UPDATE tenants SET organization_id = id WHERE organization_id IS NULL;

COMMIT;
