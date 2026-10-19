-- Migration: Create notification templates table
-- Version: 0017
-- Date: 2026-03-10

CREATE TABLE IF NOT EXISTS notification_templates (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL, -- mitigation_deadline, critical_risk, action_assigned, etc.
    subject VARCHAR(500) NOT NULL,
    message TEXT NOT NULL,
    description TEXT,
    variables JSONB DEFAULT '[]', -- List of variables like {name, severity, etc}
    is_default BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,
    created_by UUID NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL,
    INDEX idx_tenant_type (tenant_id, type),
    INDEX idx_is_default (is_default),
    INDEX idx_is_active (is_active),
    UNIQUE KEY unique_tenant_name (tenant_id, name, deleted_at)
);

-- Create index for soft deletes
CREATE INDEX idx_templates_deleted_at ON notification_templates(deleted_at) WHERE deleted_at IS NULL;

-- Create index for active templates
CREATE INDEX idx_active_templates ON notification_templates(is_active) WHERE is_active = TRUE;
