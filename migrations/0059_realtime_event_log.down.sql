-- Reverse of 0059. The log is a replay buffer, not a system of record: the
-- business state it describes lives in the tables the events were derived from,
-- so dropping it loses replay capability and nothing else.

BEGIN;

DROP INDEX IF EXISTS idx_realtime_events_created_at;
DROP INDEX IF EXISTS idx_realtime_events_occurred_at;
DROP INDEX IF EXISTS idx_realtime_events_causation_id;
DROP INDEX IF EXISTS idx_realtime_events_correlation_id;
DROP INDEX IF EXISTS idx_realtime_events_actor_id;
DROP INDEX IF EXISTS idx_realtime_events_aggregate_id;
DROP INDEX IF EXISTS idx_realtime_events_aggregate_type;
DROP INDEX IF EXISTS idx_realtime_events_type;
DROP INDEX IF EXISTS idx_realtime_events_tenant_id;
DROP INDEX IF EXISTS idx_rt_tenant_seq;

DROP TABLE IF EXISTS realtime_events;

COMMIT;
