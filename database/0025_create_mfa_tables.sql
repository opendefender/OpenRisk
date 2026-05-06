-- Create MFA Secrets table
CREATE TABLE mfa_secrets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL UNIQUE,
    tenant_id UUID NOT NULL,
    secret_encrypted TEXT NOT NULL,
    is_verified BOOLEAN NOT NULL DEFAULT false,
    verified_at TIMESTAMP,
    last_used_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

CREATE INDEX idx_mfa_secrets_user_id ON mfa_secrets(user_id);
CREATE INDEX idx_mfa_secrets_tenant_id ON mfa_secrets(tenant_id);
CREATE INDEX idx_mfa_secrets_tenant_user ON mfa_secrets(tenant_id, user_id);

-- Create MFA Backup Codes table
CREATE TABLE mfa_backup_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    code_hash VARCHAR(255) NOT NULL,
    used_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

CREATE INDEX idx_mfa_backup_codes_user_id ON mfa_backup_codes(user_id);
CREATE INDEX idx_mfa_backup_codes_tenant_id ON mfa_backup_codes(tenant_id);
CREATE INDEX idx_mfa_backup_codes_tenant_user ON mfa_backup_codes(tenant_id, user_id);
CREATE INDEX idx_mfa_backup_codes_code_hash ON mfa_backup_codes(code_hash);

-- Create OAuth Providers table
CREATE TABLE user_oauth_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    provider VARCHAR(50) NOT NULL, -- 'google', 'github'
    provider_user_id VARCHAR(255) NOT NULL,
    email VARCHAR(255),
    access_token TEXT,
    refresh_token TEXT,
    access_token_expires_at TIMESTAMP,
    last_login_at TIMESTAMP,
    linked_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    UNIQUE(user_id, provider)
);

CREATE INDEX idx_oauth_providers_user_id ON user_oauth_providers(user_id);
CREATE INDEX idx_oauth_providers_tenant_id ON user_oauth_providers(tenant_id);
CREATE INDEX idx_oauth_providers_tenant_user ON user_oauth_providers(tenant_id, user_id);
CREATE INDEX idx_oauth_providers_provider ON user_oauth_providers(provider);
CREATE INDEX idx_oauth_providers_email ON user_oauth_providers(email);

-- Alter Users table to add MFA fields
ALTER TABLE users 
ADD COLUMN mfa_enabled BOOLEAN NOT NULL DEFAULT false,
ADD COLUMN mfa_verified_at TIMESTAMP,
ADD COLUMN mfa_temporary_code VARCHAR(255);

CREATE INDEX idx_users_mfa_enabled ON users(mfa_enabled);
