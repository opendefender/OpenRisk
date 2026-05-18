-- Migration: 0027_create_mitigations_comprehensive.up.sql
-- Purpose: Create comprehensive mitigations + subactions tables with full tracing (source, scanner, approvals, dependencies)

-- Create mitigations table (plan de mitigation)
CREATE TABLE IF NOT EXISTS mitigations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    risk_id UUID NOT NULL,
    
    -- Core fields
    title VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'PLANNED', -- PLANNED|IN_PROGRESS|REVIEW|DONE|CANCELLED
    priority VARCHAR(20) NOT NULL DEFAULT 'medium', -- low|medium|high|critical
    
    -- Multi-user assignment (JSONB array of UUIDs)
    assigned_to JSONB DEFAULT '[]'::jsonb,
    
    -- Progress tracking (0-100)
    progress INTEGER DEFAULT 0 CHECK (progress >= 0 AND progress <= 100),
    
    -- Lifecycle tracking
    created_by UUID NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    approved_by UUID,
    approved_at TIMESTAMP,
    
    -- Source tracking: manual|scanner|cti|ai
    source VARCHAR(20) NOT NULL DEFAULT 'manual',
    auto_detected_at TIMESTAMP,
    
    -- Link to scanner config if auto-detected
    scanner_config_id UUID,
    
    -- Soft delete
    deleted_at TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Indices for multi-tenancy + common queries
    CONSTRAINT fk_mitigations_tenant FOREIGN KEY (tenant_id) REFERENCES organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_mitigations_risk FOREIGN KEY (risk_id) REFERENCES risks(id) ON DELETE CASCADE,
    CONSTRAINT fk_mitigations_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT fk_mitigations_approved_by FOREIGN KEY (approved_by) REFERENCES users(id) ON DELETE SET NULL
);

-- Create indices for performance
CREATE INDEX IF NOT EXISTS idx_mitigations_tenant_id ON mitigations(tenant_id);
CREATE INDEX IF NOT EXISTS idx_mitigations_risk_id ON mitigations(risk_id);
CREATE INDEX IF NOT EXISTS idx_mitigations_status ON mitigations(status);
CREATE INDEX IF NOT EXISTS idx_mitigations_priority ON mitigations(priority);
CREATE INDEX IF NOT EXISTS idx_mitigations_created_by ON mitigations(created_by);
CREATE INDEX IF NOT EXISTS idx_mitigations_source ON mitigations(source);
CREATE INDEX IF NOT EXISTS idx_mitigations_deleted_at ON mitigations(deleted_at);
-- Composite indices for common queries
CREATE INDEX IF NOT EXISTS idx_mitigations_tenant_status ON mitigations(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_mitigations_tenant_priority ON mitigations(tenant_id, priority DESC);
CREATE INDEX IF NOT EXISTS idx_mitigations_risk_status ON mitigations(risk_id, status);

-- Create mitigation_subactions table (sous-actions / checklist)
CREATE TABLE IF NOT EXISTS mitigation_subactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mitigation_id UUID NOT NULL,
    
    -- Core fields
    title VARCHAR(255) NOT NULL,
    description TEXT,
    
    -- Completion tracking
    completed BOOLEAN DEFAULT false,
    completed_at TIMESTAMP,
    completed_by UUID,
    
    -- Source of completion: manual|scanner|ai
    completed_source VARCHAR(20),
    
    -- Auto-detection tracking
    auto_detected_at TIMESTAMP,
    
    -- Dependency management
    depends_on UUID,
    
    -- Ordering for UI
    "order" INTEGER DEFAULT 0,
    
    -- Due date
    due_date TIMESTAMP,
    
    -- Soft delete
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Foreign keys
    CONSTRAINT fk_subactions_mitigation FOREIGN KEY (mitigation_id) REFERENCES mitigations(id) ON DELETE CASCADE,
    CONSTRAINT fk_subactions_completed_by FOREIGN KEY (completed_by) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT fk_subactions_depends_on FOREIGN KEY (depends_on) REFERENCES mitigation_subactions(id) ON DELETE SET NULL
);

-- Create indices for subactions
CREATE INDEX IF NOT EXISTS idx_subactions_mitigation_id ON mitigation_subactions(mitigation_id);
CREATE INDEX IF NOT EXISTS idx_subactions_completed ON mitigation_subactions(completed);
CREATE INDEX IF NOT EXISTS idx_subactions_depends_on ON mitigation_subactions(depends_on);
CREATE INDEX IF NOT EXISTS idx_subactions_deleted_at ON mitigation_subactions(deleted_at);
CREATE INDEX IF NOT EXISTS idx_subactions_completed_by ON mitigation_subactions(completed_by);
-- Composite for progress calculation
CREATE INDEX IF NOT EXISTS idx_subactions_mitigation_completed ON mitigation_subactions(mitigation_id, completed, deleted_at);
-- For ordering
CREATE INDEX IF NOT EXISTS idx_subactions_mitigation_order ON mitigation_subactions(mitigation_id, "order");
