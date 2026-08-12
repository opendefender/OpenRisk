-- 0051 — Incidents that say where they came from, who must know, and what was learned.

-- Provenance. Without these an automatically-opened incident is indistinguishable
-- from one somebody declared, which is how automatic alerts lose their audience.
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS origin              VARCHAR(24) NOT NULL DEFAULT 'manual';
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS origin_rule_id      UUID;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS origin_rule_name    VARCHAR(160);
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS origin_execution_id UUID;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS origin_detail       TEXT;

-- Real links. risk_id is a bigint while risks are UUID-keyed, so incident → risk
-- was structurally impossible; these carry the actual ids. risk_id is left in
-- place (unused) rather than dropped, so a rollback loses nothing.
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS risk_ids     JSONB DEFAULT '[]'::jsonb;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS asset_ids    JSONB DEFAULT '[]'::jsonb;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS stakeholders JSONB DEFAULT '[]'::jsonb;

CREATE INDEX IF NOT EXISTS idx_incidents_origin      ON incidents (origin);
CREATE INDEX IF NOT EXISTS idx_incidents_origin_rule ON incidents (origin_rule_id);

-- Every incident that exists today was declared by a human through the UI: the
-- automatic producers are introduced by this release. Saying so is a statement
-- of fact, not a guess.
UPDATE incidents SET origin = 'manual' WHERE origin IS NULL OR origin = '';

-- The structured review. One per incident.
CREATE TABLE IF NOT EXISTS incident_post_mortems (
    id                   UUID PRIMARY KEY,
    tenant_id            UUID NOT NULL,
    incident_id          BIGINT NOT NULL UNIQUE,
    summary              TEXT,
    root_cause           TEXT,
    contributing_factors TEXT,
    impact               TEXT,
    detection            TEXT,
    what_went_well       TEXT,
    lessons_learned      TEXT,
    timeline             JSONB DEFAULT '[]'::jsonb,
    corrective_actions   JSONB DEFAULT '[]'::jsonb,
    status               VARCHAR(16) NOT NULL DEFAULT 'draft',
    author_id            UUID,
    published_at         TIMESTAMPTZ,
    published_by         UUID,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_post_mortems_tenant ON incident_post_mortems (tenant_id);
CREATE INDEX IF NOT EXISTS idx_post_mortems_status ON incident_post_mortems (status);
