-- Migration 0018: Risk Management System Enhancements
-- Purpose: Add new fields to risks table, create indices for performance, add audit trail
-- Features: Probability/Impact as float, Criticality level, Audit trail, Performance indices
-- Date: 2026-05-11

-- ============================================================================
-- 1. Add new columns to risks table (if not exist)
-- ============================================================================
ALTER TABLE risks ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE;
ALTER TABLE risks ADD COLUMN IF NOT EXISTS probability NUMERIC(5,3) CHECK (probability >= 0 AND probability <= 1);
ALTER TABLE risks ADD COLUMN IF NOT EXISTS impact NUMERIC(5,1) CHECK (impact >= 0 AND impact <= 10);
ALTER TABLE risks ADD COLUMN IF NOT EXISTS score NUMERIC(8,3);
ALTER TABLE risks ADD COLUMN IF NOT EXISTS criticality VARCHAR(20) DEFAULT 'low';
ALTER TABLE risks ADD COLUMN IF NOT EXISTS assigned_to UUID REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE risks ADD COLUMN IF NOT EXISTS reviewer_id UUID REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE risks ADD COLUMN IF NOT EXISTS asset_id UUID;
ALTER TABLE risks ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE risks ADD COLUMN IF NOT EXISTS source VARCHAR(20) DEFAULT 'manual';
ALTER TABLE risks ADD COLUMN IF NOT EXISTS source_cve_id VARCHAR(50);
ALTER TABLE risks ADD COLUMN IF NOT EXISTS treatment_plan VARCHAR(20) DEFAULT 'mitigate';
ALTER TABLE risks ADD COLUMN IF NOT EXISTS residual_risk NUMERIC(8,3);
ALTER TABLE risks ADD COLUMN IF NOT EXISTS last_mitigated_at TIMESTAMP;
ALTER TABLE risks ADD COLUMN IF NOT EXISTS frameworks TEXT[] DEFAULT '{}';
ALTER TABLE risks ADD COLUMN IF NOT EXISTS control_ids TEXT[] DEFAULT '{}';

-- Rename/alias for legacy compatibility
-- (assuming organization_id already exists)
-- ALTER TABLE risks ADD COLUMN IF NOT EXISTS tenant_id UUID;
-- UPDATE risks SET tenant_id = organization_id WHERE tenant_id IS NULL AND organization_id IS NOT NULL;

-- ============================================================================
-- 2. Create Risk Audit Trail Table (APPEND-ONLY, NEVER DELETE)
-- ============================================================================
-- ABSOLUTE RULE: This table is APPEND-ONLY. No UPDATE or DELETE allowed,
-- even in migrations. Only INSERT and SELECT operations are permitted.
CREATE TABLE IF NOT EXISTS risk_audit_trail (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    risk_id UUID NOT NULL REFERENCES risks(id) ON DELETE CASCADE,
    
    -- Action details
    action VARCHAR(50) NOT NULL, -- 'create', 'update', 'accept', 'mitigate', 'delete', etc.
    changed_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    changed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Field-level tracking
    field_name VARCHAR(255), -- NULL for bulk operations, field name for specific changes
    old_value JSONB, -- Previous value (serialized)
    new_value JSONB, -- New value (serialized)
    
    -- Reason tracking
    reason TEXT, -- Why was this change made?
    
    -- Metadata
    ip_address INET, -- IP of the user making the change
    user_agent TEXT,
    
    -- ABSOLUTE: No soft delete on this table
    -- Created timestamps only, never modified
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT audit_trail_immutable CHECK (created_at IS NOT NULL)
);

-- Index for queries
CREATE INDEX IF NOT EXISTS idx_risk_audit_risk_id ON risk_audit_trail(risk_id);
CREATE INDEX IF NOT EXISTS idx_risk_audit_tenant_id ON risk_audit_trail(tenant_id);
CREATE INDEX IF NOT EXISTS idx_risk_audit_changed_at ON risk_audit_trail(changed_at DESC);
CREATE INDEX IF NOT EXISTS idx_risk_audit_changed_by ON risk_audit_trail(changed_by);
CREATE INDEX IF NOT EXISTS idx_risk_audit_action ON risk_audit_trail(action);
CREATE INDEX IF NOT EXISTS idx_risk_audit_tenant_risk ON risk_audit_trail(tenant_id, risk_id);

-- ============================================================================
-- 3. Add Performance Indices to Risks Table
-- ============================================================================
-- Multi-column indices for common queries
CREATE INDEX IF NOT EXISTS idx_risks_tenant_status ON risks(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_risks_tenant_criticality ON risks(tenant_id, criticality);
CREATE INDEX IF NOT EXISTS idx_risks_tenant_source ON risks(tenant_id, source);
CREATE INDEX IF NOT EXISTS idx_risks_tenant_assigned_to ON risks(tenant_id, assigned_to);
CREATE INDEX IF NOT EXISTS idx_risks_tenant_created_by ON risks(tenant_id, created_by);

-- Full-text search index (French language)
CREATE INDEX IF NOT EXISTS idx_risks_fts_french ON risks USING GIN(
    to_tsvector('french'::regconfig, 
        COALESCE(name, '') || ' ' || COALESCE(title, '') || ' ' || COALESCE(description, '')
    )
);

-- GIN index for array fields (tags, frameworks, control_ids)
CREATE INDEX IF NOT EXISTS idx_risks_tags ON risks USING GIN(tags);
CREATE INDEX IF NOT EXISTS idx_risks_frameworks ON risks USING GIN(frameworks);
CREATE INDEX IF NOT EXISTS idx_risks_control_ids ON risks USING GIN(control_ids);

-- Individual field indices for common filters
CREATE INDEX IF NOT EXISTS idx_risks_criticality ON risks(criticality);
CREATE INDEX IF NOT EXISTS idx_risks_status ON risks(status);
CREATE INDEX IF NOT EXISTS idx_risks_source ON risks(source);
CREATE INDEX IF NOT EXISTS idx_risks_created_at ON risks(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_risks_score ON risks(score DESC);
CREATE INDEX IF NOT EXISTS idx_risks_source_cve_id ON risks(source_cve_id);

-- ============================================================================
-- 4. Add Comments for Clarity
-- ============================================================================
COMMENT ON TABLE risk_audit_trail IS 'Append-only audit trail for risk changes. NEVER DELETE OR UPDATE records.';
COMMENT ON COLUMN risk_audit_trail.action IS 'Type of change: create, update, accept, mitigate, delete, bulk_action, import, etc.';
COMMENT ON COLUMN risk_audit_trail.field_name IS 'Specific field changed (NULL for bulk operations)';
COMMENT ON COLUMN risk_audit_trail.old_value IS 'Previous value (JSONB) before the change';
COMMENT ON COLUMN risk_audit_trail.new_value IS 'New value (JSONB) after the change';
