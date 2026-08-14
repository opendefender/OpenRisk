-- Migration: 0026_add_composite_indices.sql
-- Purpose: Add tenant-scoped composite indices for high-performance multi-tenant queries

-- Risks queries often filter on organization_id + status or order by score
CREATE INDEX IF NOT EXISTS idx_risks_org_status ON risks (organization_id, status);
CREATE INDEX IF NOT EXISTS idx_risks_org_score_desc ON risks (organization_id, score DESC);
CREATE INDEX IF NOT EXISTS idx_risks_org_created_desc ON risks (organization_id, created_at DESC);

-- Mitigation list queries often filter by organization_id and status
CREATE INDEX IF NOT EXISTS idx_mitigations_org_status ON mitigations (organization_id, status);

-- Notifications read model often filters by user and tenant
CREATE INDEX IF NOT EXISTS idx_notifications_user_tenant_status ON notifications (user_id, tenant_id, status);

-- Assets queries often filter by organization_id and criticality
CREATE INDEX IF NOT EXISTS idx_assets_org_criticality ON assets (organization_id, criticality DESC);

-- Users and roles are usually scoped by organization_id
CREATE INDEX IF NOT EXISTS idx_users_org_email ON users (organization_id, email);
