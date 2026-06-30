-- Migration: 0028_create_compliance_schema.up.sql
-- Purpose: Create compliance foundation tables (frameworks, controls, evidences)

-- =============================================================================
-- compliance_frameworks: Global reference table (no tenant_id)
-- Examples: ISO 27001, SOC 2, NIST CSF, DORA, COBAC, BCEAO
-- =============================================================================
CREATE TABLE IF NOT EXISTS compliance_frameworks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Core fields
    name VARCHAR(255) NOT NULL,
    version VARCHAR(50) NOT NULL DEFAULT '',
    description TEXT,

    -- Soft delete + timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Unique constraint: (name, version) pair must be unique among non-deleted rows
CREATE UNIQUE INDEX IF NOT EXISTS idx_compliance_frameworks_name_version
    ON compliance_frameworks(name, version) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_compliance_frameworks_deleted_at
    ON compliance_frameworks(deleted_at);

-- =============================================================================
-- compliance_controls: Per-tenant controls linked to a global framework
-- =============================================================================
CREATE TABLE IF NOT EXISTS compliance_controls (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    framework_id UUID NOT NULL,

    -- Core fields
    reference_code VARCHAR(50) NOT NULL DEFAULT '',
    name VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(30) NOT NULL DEFAULT 'not_implemented',
    -- Allowed statuses: not_implemented | in_progress | implemented | not_applicable

    -- Soft delete + timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,

    -- Foreign keys
    CONSTRAINT fk_controls_framework FOREIGN KEY (framework_id)
        REFERENCES compliance_frameworks(id) ON DELETE CASCADE
);

-- Performance indices
CREATE INDEX IF NOT EXISTS idx_compliance_controls_tenant_id
    ON compliance_controls(tenant_id);
CREATE INDEX IF NOT EXISTS idx_compliance_controls_framework_id
    ON compliance_controls(framework_id);
CREATE INDEX IF NOT EXISTS idx_compliance_controls_status
    ON compliance_controls(status);
CREATE INDEX IF NOT EXISTS idx_compliance_controls_deleted_at
    ON compliance_controls(deleted_at);
-- Composite: tenant + framework (most common query)
CREATE INDEX IF NOT EXISTS idx_compliance_controls_tenant_framework
    ON compliance_controls(tenant_id, framework_id);
-- Unique: (tenant_id, framework_id, reference_code) among non-deleted rows
CREATE UNIQUE INDEX IF NOT EXISTS idx_compliance_controls_tenant_fw_ref
    ON compliance_controls(tenant_id, framework_id, reference_code)
    WHERE deleted_at IS NULL AND reference_code != '';

-- =============================================================================
-- control_evidences: Per-tenant evidence artifacts linked to a control
-- =============================================================================
CREATE TABLE IF NOT EXISTS control_evidences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    control_id UUID NOT NULL,

    -- Core fields
    filename VARCHAR(255) NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    description TEXT,

    -- Who uploaded it
    uploaded_by UUID,

    -- Soft delete + timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,

    -- Foreign keys
    CONSTRAINT fk_evidences_control FOREIGN KEY (control_id)
        REFERENCES compliance_controls(id) ON DELETE CASCADE
);

-- Performance indices
CREATE INDEX IF NOT EXISTS idx_control_evidences_tenant_id
    ON control_evidences(tenant_id);
CREATE INDEX IF NOT EXISTS idx_control_evidences_control_id
    ON control_evidences(control_id);
CREATE INDEX IF NOT EXISTS idx_control_evidences_deleted_at
    ON control_evidences(deleted_at);
-- Composite: tenant + control (most common query)
CREATE INDEX IF NOT EXISTS idx_control_evidences_tenant_control
    ON control_evidences(tenant_id, control_id);
