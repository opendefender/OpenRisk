-- #903 — "Reporter au prochain comité" on a pending approval.
--
-- A dated note on the request, kept apart from the decisions so the signature
-- engine (quorum, four-eyes, delegations) is untouched: a deferred request is
-- still pending. AutoMigrate adds the column on boot too.

ALTER TABLE approval_requests ADD COLUMN IF NOT EXISTS deferrals jsonb DEFAULT '[]'::jsonb;
