-- #344 — realtime_events becomes a transactional outbox for enlisted use cases (ADR 0010, D-109).
-- Additive only. published_at arrives with a fast default (no table rewrite): rows that exist now
-- were delivered inline before this migration, so they read as published at migration time; new
-- rows default to NULL. The block runs only when the column is absent, so replaying this file on a
-- live database changes no row. Mirrored by database.prepareRealtimeOutbox for DB_HOST boots.
BEGIN;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                  WHERE table_schema = current_schema()
                    AND table_name = 'realtime_events' AND column_name = 'published_at') THEN
    ALTER TABLE realtime_events ADD COLUMN published_at TIMESTAMPTZ DEFAULT now();
    ALTER TABLE realtime_events ALTER COLUMN published_at DROP DEFAULT;
  END IF;
END $$;

ALTER TABLE realtime_events ADD COLUMN IF NOT EXISTS abandoned_at TIMESTAMPTZ;
ALTER TABLE realtime_events ADD COLUMN IF NOT EXISTS attempts     INTEGER NOT NULL DEFAULT 0;
ALTER TABLE realtime_events ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ;
ALTER TABLE realtime_events ADD COLUMN IF NOT EXISTS last_error   TEXT NOT NULL DEFAULT '';

-- The relay's working set: pending rows per tenant in sequence order. Tiny by construction.
CREATE INDEX IF NOT EXISTS idx_rt_outbox_pending
    ON realtime_events (tenant_id, sequence)
 WHERE published_at IS NULL AND abandoned_at IS NULL;

-- One row per tenant: the sequence allocator and the relay's per-tenant lock (ADR 0010 sections 1-2).
CREATE TABLE IF NOT EXISTS realtime_event_sequences (
    tenant_id     UUID        PRIMARY KEY,
    last_sequence BIGINT      NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMIT;
