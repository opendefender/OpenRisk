-- Migration: Create notifications table
-- Version: 0015
-- Date: 2026-03-10

CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    type VARCHAR(50) NOT NULL, -- mitigation_deadline, critical_risk, action_assigned, risk_update, risk_resolved
    channel VARCHAR(50) NOT NULL, -- email, slack, webhook, in_app
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, sent, delivered, failed, read
    subject TEXT NOT NULL,
    message TEXT NOT NULL,
    description TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    INDEX idx_user_tenant (user_id, tenant_id),
    INDEX idx_created_at (created_at),
    INDEX idx_status (status),
    INDEX idx_type (type),
    INDEX idx_channel (channel)
);

-- Create index for soft deletes
CREATE INDEX idx_notifications_deleted_at ON notifications(deleted_at) WHERE deleted_at IS NULL;

-- Create index for unread notifications (status = 'pending')
CREATE INDEX idx_notifications_unread ON notifications(user_id, status) WHERE status = 'pending';
