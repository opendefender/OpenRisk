-- Migration: Create notification logs table
-- Version: 0018
-- Date: 2026-03-10

CREATE TABLE IF NOT EXISTS notification_logs (
    id UUID PRIMARY KEY,
    notification_id UUID NOT NULL,
    user_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    channel VARCHAR(50) NOT NULL, -- email, slack, webhook, in_app
    provider VARCHAR(100), -- SendGrid, Slack, Custom Webhook, etc.
    status VARCHAR(50) NOT NULL, -- sent, delivered, failed, pending
    error_message TEXT,
    error_code VARCHAR(50),
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 3,
    next_retry_at TIMESTAMP,
    sent_at TIMESTAMP,
    delivered_at TIMESTAMP,
    failed_at TIMESTAMP,
    metadata JSONB DEFAULT '{}', -- Additional context about send/delivery
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (notification_id) REFERENCES notifications(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    INDEX idx_notification_id (notification_id),
    INDEX idx_user_tenant (user_id, tenant_id),
    INDEX idx_channel (channel),
    INDEX idx_status (status),
    INDEX idx_created_at (created_at),
    INDEX idx_next_retry (next_retry_at) WHERE status = 'pending'
);

-- Create index for failed notifications needing retry
CREATE INDEX idx_notification_retry ON notification_logs(next_retry_at) WHERE status = 'pending' AND next_retry_at IS NOT NULL AND next_retry_at <= CURRENT_TIMESTAMP;

-- Create index for analytics (notifications per channel)
CREATE INDEX idx_logs_analytics ON notification_logs(channel, created_at);
