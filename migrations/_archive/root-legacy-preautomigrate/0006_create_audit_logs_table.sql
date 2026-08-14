-- Migration: Create audit logs table for authentication event tracking
-- Purpose: Track all authentication events (login, register, logout, token refresh, etc.)
-- Date: 2025-12-06

CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL, -- 'login', 'register', 'logout', 'token_refresh', 'role_change', 'user_delete', etc.
    resource VARCHAR(100), -- 'auth', 'user', 'role', etc.
    resource_id UUID, -- ID of the affected resource (if applicable)
    result VARCHAR(20) NOT NULL, -- 'success', 'failure'
    error_message TEXT, -- Description of failure (if result = 'failure')
    ip_address INET, -- Client IP address
    user_agent TEXT, -- Client user agent string
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for common queries
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_timestamp ON audit_logs(timestamp DESC);
CREATE INDEX idx_audit_logs_user_timestamp ON audit_logs(user_id, timestamp DESC);
CREATE INDEX idx_audit_logs_result ON audit_logs(result);

-- Create index for IP-based analysis
CREATE INDEX idx_audit_logs_ip_address ON audit_logs(ip_address);

-- Add comments for clarity
COMMENT ON TABLE audit_logs IS 'Audit trail for all authentication and authorization events';
COMMENT ON COLUMN audit_logs.user_id IS 'Reference to user (NULL for pre-auth events like register)';
COMMENT ON COLUMN audit_logs.action IS 'Type of action: login, register, logout, token_refresh, role_change, user_delete, etc.';
COMMENT ON COLUMN audit_logs.resource IS 'Resource type affected (auth, user, role, etc.)';
COMMENT ON COLUMN audit_logs.result IS 'Outcome of the action (success or failure)';
COMMENT ON COLUMN audit_logs.ip_address IS 'IPv4 or IPv6 address of the client';
