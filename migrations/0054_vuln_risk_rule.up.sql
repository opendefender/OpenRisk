-- Attack Surface §4 — the configurable vulnerability→risk rule.
--
-- The condition used to be hardcoded ("P1 or CISA-KEV, on a known asset") in
-- the ingest use case: every tenant got the same appetite and nobody could see
-- what it was. One row per tenant now holds it.

CREATE TABLE IF NOT EXISTS vuln_risk_rules (
    id                        uuid PRIMARY KEY,
    tenant_id                 uuid NOT NULL UNIQUE,
    enabled                   boolean NOT NULL DEFAULT false,
    min_cvss                  numeric(4,1) NOT NULL DEFAULT 7,
    require_internet_exposure boolean NOT NULL DEFAULT false,
    min_asset_criticality     varchar(16),
    require_kev               boolean NOT NULL DEFAULT false,
    require_asset             boolean NOT NULL DEFAULT true,
    notify_on_create          boolean NOT NULL DEFAULT true,
    created_at                timestamptz NOT NULL DEFAULT now(),
    updated_at                timestamptz NOT NULL DEFAULT now(),
    updated_by                uuid
);

-- Deliberately NO backfill of enabled rules. Automatic risk creation writes to
-- somebody's register; switching it on for every existing tenant during an
-- upgrade would be exactly the silent surprise this feature removes. Tenants who
-- relied on the old hardcoded behaviour enable the rule themselves, and the
-- shipped defaults (CVSS ≥ 7, asset criticality ≥ HIGH) reproduce it.

-- Origin of an automatically proposed risk: which finding proposed it, and the
-- rule's own explanation frozen at creation time.
ALTER TABLE risks ADD COLUMN IF NOT EXISTS source_vulnerability_id uuid;
ALTER TABLE risks ADD COLUMN IF NOT EXISTS source_rule_reason      text;

CREATE INDEX IF NOT EXISTS idx_risks_source_vulnerability ON risks (source_vulnerability_id);

-- The draft-review queue's query: machine-proposed risks awaiting a decision.
CREATE INDEX IF NOT EXISTS idx_risks_draft_review
    ON risks (tenant_id, created_at DESC)
    WHERE lifecycle_state = 'draft' AND source IN ('scan_auto', 'cti_auto');
