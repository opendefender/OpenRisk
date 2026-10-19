-- +migrate Up
-- Create auth_audit_logs table for authentication audit trail
-- APPEND-ONLY table - NO UPDATES OR DELETES allowed
CREATE TABLE IF NOT EXISTS auth_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID,
    tenant_id UUID,
    action VARCHAR(255) NOT NULL,
    ip VARCHAR(45), -- IPv4/IPv6
    user_agent TEXT,
    geo_country VARCHAR(2),
    success BOOLEAN DEFAULT true,
    failure_reason TEXT,
    device_fingerprint VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for auth_audit_logs
CREATE INDEX IF NOT EXISTS idx_auth_audit_logs_user_id ON auth_audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_auth_audit_logs_tenant_id ON auth_audit_logs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_auth_audit_logs_action ON auth_audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_auth_audit_logs_success ON auth_audit_logs(success);
CREATE INDEX IF NOT EXISTS idx_auth_audit_logs_created_at ON auth_audit_logs(created_at);

-- +migrate Down
-- WARNING: This will delete audit logs - use with caution
DROP TABLE IF EXISTS auth_audit_logs;