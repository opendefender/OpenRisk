-- Migration: Create notification preferences table
-- Version: 0016
-- Date: 2026-03-10

CREATE TABLE IF NOT EXISTS notification_preferences (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL UNIQUE,
    tenant_id UUID NOT NULL,
    
    -- Email preferences
    email_on_mitigation_deadline BOOLEAN DEFAULT TRUE,
    email_on_critical_risk BOOLEAN DEFAULT TRUE,
    email_on_action_assigned BOOLEAN DEFAULT TRUE,
    email_on_risk_update BOOLEAN DEFAULT FALSE,
    email_on_risk_resolved BOOLEAN DEFAULT FALSE,
    email_deadline_advance_days INTEGER DEFAULT 3,
    
    -- Slack preferences
    slack_enabled BOOLEAN DEFAULT FALSE,
    slack_on_mitigation_deadline BOOLEAN DEFAULT TRUE,
    slack_on_critical_risk BOOLEAN DEFAULT TRUE,
    slack_on_action_assigned BOOLEAN DEFAULT TRUE,
    slack_on_risk_update BOOLEAN DEFAULT FALSE,
    slack_on_risk_resolved BOOLEAN DEFAULT FALSE,
    slack_webhook_url VARCHAR(500),
    
    -- Webhook preferences
    webhook_enabled BOOLEAN DEFAULT FALSE,
    webhook_on_mitigation_deadline BOOLEAN DEFAULT TRUE,
    webhook_on_critical_risk BOOLEAN DEFAULT TRUE,
    webhook_on_action_assigned BOOLEAN DEFAULT TRUE,
    webhook_on_risk_update BOOLEAN DEFAULT FALSE,
    webhook_on_risk_resolved BOOLEAN DEFAULT FALSE,
    webhook_url VARCHAR(500),
    webhook_secret VARCHAR(255),
    
    -- Global preferences
    disable_all_notifications BOOLEAN DEFAULT FALSE,
    enable_sound_notifications BOOLEAN DEFAULT TRUE,
    enable_desktop_notifications BOOLEAN DEFAULT TRUE,
    mute_until TIMESTAMP,
    
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    INDEX idx_user_preferences (user_id, tenant_id),
    INDEX idx_disabled_notifications (disable_all_notifications)
);

-- Create index for muted notifications
CREATE INDEX idx_notification_mute ON notification_preferences(mute_until) WHERE mute_until > CURRENT_TIMESTAMP;
