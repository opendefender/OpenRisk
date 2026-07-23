-- RC1 performance hardening: composite indexes on hot tenant-scoped access paths.
-- Every list/dashboard query filters by tenant_id first, then by a status/type
-- discriminator; a (tenant_id, <col>) index serves both that pair AND the
-- tenant_id-only prefix. Idempotent (IF NOT EXISTS) so re-runs are safe.
-- Not CONCURRENTLY: golang-migrate wraps each migration in a transaction.

-- Risks: register list by status, dashboards by criticality.
CREATE INDEX IF NOT EXISTS idx_risks_tenant_status       ON risks (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_risks_tenant_criticality  ON risks (tenant_id, criticality);

-- Vulnerabilities: register sorted/filtered by status and priority tier.
CREATE INDEX IF NOT EXISTS idx_vulns_tenant_status        ON vulnerabilities (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_vulns_tenant_priority      ON vulnerabilities (tenant_id, priority_tier);

-- Assets: inventory filtered by type.
CREATE INDEX IF NOT EXISTS idx_assets_tenant_type         ON assets (tenant_id, type);

-- Compliance controls: framework detail loads by framework; gap analysis by status.
CREATE INDEX IF NOT EXISTS idx_controls_tenant_framework  ON compliance_controls (tenant_id, framework_id);
CREATE INDEX IF NOT EXISTS idx_controls_tenant_status     ON compliance_controls (tenant_id, status);

-- Incidents: register KPIs and filters by status.
CREATE INDEX IF NOT EXISTS idx_incidents_tenant_status    ON incidents (tenant_id, status);

-- Mitigations: board by status, and lookups of mitigations for a given risk.
CREATE INDEX IF NOT EXISTS idx_mitigations_tenant_status  ON mitigations (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_mitigations_tenant_risk    ON mitigations (tenant_id, risk_id);

-- Audit trail: governance journal is read tenant-scoped, newest first.
CREATE INDEX IF NOT EXISTS idx_audit_events_tenant_created ON audit_events (tenant_id, created_at);
