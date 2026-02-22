-- Migration: 0009_add_performance_indexes.sql
-- Purpose: Add indexes for frequently queried columns to optimize query performance

-- Risk table indexes
CREATE INDEX IF NOT EXISTS idx_risks_status ON risks(status);
CREATE INDEX IF NOT EXISTS idx_risks_score ON risks(score DESC);
CREATE INDEX IF NOT EXISTS idx_risks_created_at ON risks(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_risks_updated_at ON risks(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_risks_impact ON risks(impact);
CREATE INDEX IF NOT EXISTS idx_risks_probability ON risks(probability);
CREATE INDEX IF NOT EXISTS idx_risks_title_search ON risks USING GIN(to_tsvector('english', title));
CREATE INDEX IF NOT EXISTS idx_risks_tags ON risks USING GIN(tags);

-- Composite indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_risks_status_score ON risks(status, score DESC);
CREATE INDEX IF NOT EXISTS idx_risks_created_status ON risks(created_at DESC, status);

-- Mitigation table indexes
CREATE INDEX IF NOT EXISTS idx_mitigations_risk_id ON mitigations(risk_id);
CREATE INDEX IF NOT EXISTS idx_mitigations_status ON mitigations(status);
CREATE INDEX IF NOT EXISTS idx_mitigations_due_date ON mitigations(due_date DESC);
CREATE INDEX IF NOT EXISTS idx_mitigations_created_at ON mitigations(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_mitigations_risk_status ON mitigations(risk_id, status);

-- Mitigation SubActions table indexes
CREATE INDEX IF NOT EXISTS idx_mitigation_subactions_mitigation_id ON mitigation_subactions(mitigation_id);
CREATE INDEX IF NOT EXISTS idx_mitigation_subactions_completed ON mitigation_subactions(is_completed);
CREATE INDEX IF NOT EXISTS idx_mitigation_subactions_created_at ON mitigation_subactions(created_at DESC);

-- Risk Assets (Many-to-Many) table indexes
CREATE INDEX IF NOT EXISTS idx_risk_assets_risk_id ON risk_assets(risk_id);
CREATE INDEX IF NOT EXISTS idx_risk_assets_asset_id ON risk_assets(asset_id);
CREATE INDEX IF NOT EXISTS idx_risk_assets_composite ON risk_assets(risk_id, asset_id);

-- Assets table indexes
CREATE INDEX IF NOT EXISTS idx_assets_name ON assets(name);
CREATE INDEX IF NOT EXISTS idx_assets_type ON assets(type);
CREATE INDEX IF NOT EXISTS idx_assets_created_at ON assets(created_at DESC);

-- Users table indexes
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_tenant_id ON users(tenant_id);

-- API Tokens table indexes
CREATE INDEX IF NOT EXISTS idx_tokens_user_id ON api_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_tokens_token_hash ON api_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_tokens_status ON api_tokens(status);
CREATE INDEX IF NOT EXISTS idx_tokens_expires_at ON api_tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_tokens_user_status ON api_tokens(user_id, status);

-- Audit logs table indexes
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_action ON audit_logs(user_id, action);

-- Bulk operations table indexes
CREATE INDEX IF NOT EXISTS idx_bulk_operations_status ON bulk_operations(status);
CREATE INDEX IF NOT EXISTS idx_bulk_operations_created_at ON bulk_operations(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_bulk_operations_user_id ON bulk_operations(user_id);

-- Custom fields table indexes
CREATE INDEX IF NOT EXISTS idx_custom_fields_scope ON custom_fields(scope);
CREATE INDEX IF NOT EXISTS idx_custom_fields_created_at ON custom_fields(created_at DESC);

-- Marketplace indexes
CREATE INDEX IF NOT EXISTS idx_marketplace_connectors_status ON marketplace_connectors(status);
CREATE INDEX IF NOT EXISTS idx_marketplace_installations_connector_id ON marketplace_installations(connector_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_installations_user_id ON marketplace_installations(user_id);
