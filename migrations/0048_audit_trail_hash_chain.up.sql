-- 0048 — Tamper-evident audit trail: hash chaining, retention seals, retention policy.
--
-- Every audit_events row now carries its position in the tenant's chain
-- (sequence), the hash of the previous entry (prev_hash) and its own content
-- hash (hash). Altering, deleting or reordering any entry breaks the chain from
-- that point on and the verification endpoint reports exactly where.

ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS sequence    BIGINT NOT NULL DEFAULT 0;
ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS prev_hash   VARCHAR(64);
ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS hash        VARCHAR(64);
ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS method      VARCHAR(8);
ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS path        VARCHAR(255);
ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS status_code INTEGER;
ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS source      VARCHAR(16);

-- Backfill: entries written before this migration have no chain. They are
-- numbered in creation order per tenant and marked source='legacy' so
-- verification can tell "never chained" apart from "chain broken" — silently
-- pretending they were chained would be the one lie an audit trail cannot tell.
WITH numbered AS (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY tenant_id ORDER BY created_at, id) AS seq
    FROM audit_events
    WHERE sequence = 0
)
UPDATE audit_events e
SET sequence = numbered.seq,
    source   = COALESCE(e.source, 'legacy')
FROM numbered
WHERE e.id = numbered.id;

CREATE UNIQUE INDEX IF NOT EXISTS idx_audit_tenant_seq ON audit_events (tenant_id, sequence);
CREATE INDEX IF NOT EXISTS idx_audit_events_hash       ON audit_events (hash);
CREATE INDEX IF NOT EXISTS idx_audit_events_request_id ON audit_events (request_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_source     ON audit_events (source);

-- Retention seals: pruning old entries must not silently cut the chain. A seal
-- records the range removed and the hash the surviving head links back to, so
-- verification crosses the gap knowingly.
CREATE TABLE IF NOT EXISTS audit_chain_seals (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    reason        VARCHAR(64),
    from_sequence BIGINT,
    to_sequence   BIGINT,
    pruned_count  BIGINT,
    last_hash     VARCHAR(64),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_audit_chain_seals_tenant ON audit_chain_seals (tenant_id);
CREATE INDEX IF NOT EXISTS idx_audit_chain_seals_created ON audit_chain_seals (created_at);

-- Per-tenant retention window. 0 = keep forever, the default a GRC product
-- should have to opt out of rather than into.
CREATE TABLE IF NOT EXISTS audit_retention_policies (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      UUID NOT NULL UNIQUE,
    retention_days INTEGER NOT NULL DEFAULT 0,
    last_pruned_at TIMESTAMPTZ,
    updated_by     UUID,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
