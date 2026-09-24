-- Reverses 0064's index only. The columns are NOT dropped: rows written since
-- 0064 hash actor_type and actor_label, so dropping them would make every one of
-- those entries fail chain verification — a down migration must not be the
-- thing that makes an untampered journal look tampered. An operator who wants
-- them gone must also accept that the chain can no longer be verified.

DROP INDEX IF EXISTS idx_audit_events_actor_type;
