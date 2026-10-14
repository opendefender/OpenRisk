-- #911 — a tenant's "not relevant" on a threat.
--
-- The Threat Intel page lists the catalogue entries (NVD, CISA KEV) that touch
-- the tenant's assets. A reader can mark one not relevant; it stays listed,
-- dimmed, and can be restored. One row per tenant and CVE. The catalogue has no
-- tenant column, so the tenant's call lives here. Idempotent: AutoMigrate
-- creates the same table on boot.

CREATE TABLE IF NOT EXISTS cti_threat_dismissals (
    id           uuid PRIMARY KEY,
    tenant_id    uuid NOT NULL,
    cve_id       varchar(32) NOT NULL,
    dismissed_by uuid,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_cti_threat_dismissal
    ON cti_threat_dismissals (tenant_id, cve_id);
