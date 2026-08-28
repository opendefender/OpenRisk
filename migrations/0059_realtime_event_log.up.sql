-- W0-07 — The durable realtime event log.
--
-- Redis pub/sub is fire-and-forget: an event published while a browser is
-- reconnecting is gone. This table is what turns a reconnect into a replay from
-- a cursor instead of a full refetch of every dashboard on every lid-close.
--
-- Ordering lives here: `sequence` is a per-tenant monotonic counter assigned
-- inside the append transaction, exactly as audit_events assigns its own. The
-- unique index on (tenant_id, sequence) is the backstop that makes two racing
-- publishers fail loudly rather than silently fork the tenant's order.
--
-- This is NOT a transactional outbox: the row is appended after the business
-- transaction commits, not inside it. The window is documented in
-- docs/W0-07_REALTIME_EVENT_HUB.md rather than papered over.

BEGIN;

CREATE TABLE IF NOT EXISTS realtime_events (
    id                UUID         PRIMARY KEY,
    tenant_id         UUID         NOT NULL,
    sequence          BIGINT       NOT NULL,

    type              VARCHAR(64)  NOT NULL,
    version           INTEGER      NOT NULL DEFAULT 1,
    envelope_version  INTEGER      NOT NULL DEFAULT 1,

    aggregate_type    VARCHAR(64)  NOT NULL,
    aggregate_id      VARCHAR(128),

    actor_id          UUID,
    correlation_id    VARCHAR(64),
    causation_id      VARCHAR(64),

    payload           JSONB,

    occurred_at       TIMESTAMPTZ  NOT NULL,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- The ordering guarantee. Without this a concurrent append could reuse a
-- position, and a client holding sequence N could be handed two different
-- events under the same number.
CREATE UNIQUE INDEX IF NOT EXISTS idx_rt_tenant_seq
    ON realtime_events (tenant_id, sequence);

-- Replay: "everything for this tenant after this cursor, oldest first" is the
-- only query the stream endpoint runs on the hot path.
CREATE INDEX IF NOT EXISTS idx_realtime_events_tenant_id ON realtime_events (tenant_id);
CREATE INDEX IF NOT EXISTS idx_realtime_events_type ON realtime_events (type);
CREATE INDEX IF NOT EXISTS idx_realtime_events_aggregate_type ON realtime_events (aggregate_type);
CREATE INDEX IF NOT EXISTS idx_realtime_events_aggregate_id ON realtime_events (aggregate_id);
CREATE INDEX IF NOT EXISTS idx_realtime_events_actor_id ON realtime_events (actor_id);
CREATE INDEX IF NOT EXISTS idx_realtime_events_correlation_id ON realtime_events (correlation_id);
CREATE INDEX IF NOT EXISTS idx_realtime_events_causation_id ON realtime_events (causation_id);
CREATE INDEX IF NOT EXISTS idx_realtime_events_occurred_at ON realtime_events (occurred_at);

-- Retention sweeps delete by created_at, so it needs its own index or the purge
-- degrades into a full scan as the table grows.
CREATE INDEX IF NOT EXISTS idx_realtime_events_created_at ON realtime_events (created_at);

COMMIT;
