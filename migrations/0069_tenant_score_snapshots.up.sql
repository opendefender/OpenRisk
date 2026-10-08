-- #901 — daily history of the tenant exposure score.
--
-- The tenant score is computed live and nothing kept its past values, so the
-- dashboard could not draw a 12-month line or say how much the score moved in
-- 30 days. One row per tenant per UTC day; a later computation on the same day
-- updates the row. AutoMigrate builds the same table on boot, so every
-- statement is idempotent.

CREATE TABLE IF NOT EXISTS tenant_score_snapshots (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid NOT NULL,
    day             date NOT NULL,
    value           double precision NOT NULL,
    band            varchar(16) NOT NULL,
    formula_version varchar(16) NOT NULL,
    created_at      timestamptz,
    updated_at      timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_tenant_score_snapshot_day
    ON tenant_score_snapshots (tenant_id, day);
