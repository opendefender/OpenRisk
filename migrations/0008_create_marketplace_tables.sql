-- Migration 0008: Create marketplace tables for connectors and apps
-- Date: 2025-01-22

-- Connectors table: registry of available marketplace connectors
CREATE TABLE IF NOT EXISTS connectors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    author VARCHAR(255) NOT NULL,
    version VARCHAR(50) NOT NULL,
    description TEXT NOT NULL,
    long_description TEXT,
    icon TEXT,
    category VARCHAR(100),
    status VARCHAR(50) DEFAULT 'active' CHECK(status IN ('active', 'inactive', 'deprecated', 'beta')),
    capabilities JSONB NOT NULL DEFAULT '[]',
    documentation TEXT,
    source_url VARCHAR(500),
    support_email VARCHAR(255),
    license VARCHAR(100),
    rating DECIMAL(3, 2) DEFAULT 0.0 CHECK(rating >= 0 AND rating <= 5),
    install_count BIGINT DEFAULT 0,
    downloads BIGINT DEFAULT 0,
    config_schema JSONB DEFAULT '{}',
    required_permissions JSONB DEFAULT '[]',
    supported_frameworks JSONB DEFAULT '[]',
    reviews JSONB DEFAULT '[]',
    release_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    -- Indexes for efficient querying
    CONSTRAINT connectors_name_version_unique UNIQUE(name, version)
);

CREATE INDEX IF NOT EXISTS idx_connectors_status ON connectors(status);
CREATE INDEX IF NOT EXISTS idx_connectors_category ON connectors(category);
CREATE INDEX IF NOT EXISTS idx_connectors_rating_desc ON connectors(rating DESC, install_count DESC);
CREATE INDEX IF NOT EXISTS idx_connectors_created_at ON connectors(created_at DESC);

-- Marketplace apps table: installed connectors for each tenant
CREATE TABLE IF NOT EXISTS marketplace_apps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connector_id UUID NOT NULL REFERENCES connectors(id) ON DELETE RESTRICT,
    tenant_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    version VARCHAR(50),
    status VARCHAR(50) DEFAULT 'pending' CHECK(status IN ('pending', 'installed', 'disabled', 'uninstalled', 'error')),
    configuration JSONB DEFAULT '{}',
    enabled BOOLEAN DEFAULT true,
    auto_sync BOOLEAN DEFAULT false,
    sync_interval INTEGER DEFAULT 300,
    last_sync_at TIMESTAMP,
    last_sync_status VARCHAR(50),
    last_sync_error TEXT,
    installation_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    webhook_url VARCHAR(500),
    webhook_secret VARCHAR(255),
    is_webhook_verified BOOLEAN DEFAULT false,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_marketplace_apps_tenant_id ON marketplace_apps(tenant_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_apps_connector_id ON marketplace_apps(connector_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_apps_status ON marketplace_apps(status);
CREATE INDEX IF NOT EXISTS idx_marketplace_apps_enabled ON marketplace_apps(enabled);
CREATE INDEX IF NOT EXISTS idx_marketplace_apps_user_id ON marketplace_apps(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_marketplace_apps_unique_install ON marketplace_apps(connector_id, tenant_id)
    WHERE status != 'uninstalled';

-- Connector updates table: track updates to installed connectors
CREATE TABLE IF NOT EXISTS connector_updates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES marketplace_apps(id) ON DELETE CASCADE,
    from_version VARCHAR(50) NOT NULL,
    to_version VARCHAR(50) NOT NULL,
    status VARCHAR(50) DEFAULT 'pending' CHECK(status IN ('pending', 'completed', 'failed')),
    error TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_connector_updates_app_id ON connector_updates(app_id);
CREATE INDEX IF NOT EXISTS idx_connector_updates_status ON connector_updates(status);

-- Marketplace logs table: audit trail for marketplace activities
CREATE TABLE IF NOT EXISTS marketplace_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES marketplace_apps(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL,
    action VARCHAR(100) NOT NULL,
    details JSONB DEFAULT '{}',
    status VARCHAR(50) DEFAULT 'success' CHECK(status IN ('success', 'failure')),
    error_message TEXT,
    execution_time INTEGER, -- milliseconds
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_marketplace_logs_app_id ON marketplace_logs(app_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_logs_action ON marketplace_logs(action);
CREATE INDEX IF NOT EXISTS idx_marketplace_logs_user_id ON marketplace_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_logs_created_at ON marketplace_logs(created_at DESC);

-- Trigger to update marketplace_apps updated_at timestamp
CREATE OR REPLACE FUNCTION update_marketplace_apps_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_marketplace_apps_timestamp ON marketplace_apps;
CREATE TRIGGER trigger_marketplace_apps_timestamp
BEFORE UPDATE ON marketplace_apps
FOR EACH ROW
EXECUTE FUNCTION update_marketplace_apps_timestamp();

-- Add to schema_migrations
INSERT INTO schema_migrations (version) 
VALUES ('0008_create_marketplace_tables')
ON CONFLICT (version) DO NOTHING;
