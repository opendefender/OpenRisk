-- #344 — undo 0085. Removes the outbox bookkeeping; the events themselves stay.
BEGIN;

DROP TABLE IF EXISTS realtime_event_sequences;
DROP INDEX IF EXISTS idx_rt_outbox_pending;

ALTER TABLE realtime_events DROP COLUMN IF EXISTS last_error;
ALTER TABLE realtime_events DROP COLUMN IF EXISTS locked_until;
ALTER TABLE realtime_events DROP COLUMN IF EXISTS attempts;
ALTER TABLE realtime_events DROP COLUMN IF EXISTS abandoned_at;
ALTER TABLE realtime_events DROP COLUMN IF EXISTS published_at;

COMMIT;
