-- #486 — every audit entry says which KIND of identity acted.
--
-- Until now an entry written by a background job carried a NULL actor_id and
-- nothing else, which a reader can only interpret as "system". A regulator does
-- not accept "system": it asks which job, or which token. actor_type is one of
-- user | service_token | job; actor_label names the token or the job.
--
-- Both columns are nullable and have no default. Rows written before this
-- migration keep them NULL, and CanonicalPayload (internal/domain/audit_chain.go)
-- only hashes them when set, so every existing hash still verifies. Adding a
-- column is DDL, not an UPDATE, so the append-only trigger from 0055 is not
-- involved and no row is rewritten.

ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS actor_type  varchar(16);
ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS actor_label varchar(128);

CREATE INDEX IF NOT EXISTS idx_audit_events_actor_type ON audit_events (actor_type);
