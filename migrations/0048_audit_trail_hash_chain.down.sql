-- Revert 0048 — drop the chain columns and the retention tables.

DROP TABLE IF EXISTS audit_retention_policies;
DROP TABLE IF EXISTS audit_chain_seals;

DROP INDEX IF EXISTS idx_audit_tenant_seq;
DROP INDEX IF EXISTS idx_audit_events_hash;
DROP INDEX IF EXISTS idx_audit_events_request_id;
DROP INDEX IF EXISTS idx_audit_events_source;

ALTER TABLE audit_events DROP COLUMN IF EXISTS sequence;
ALTER TABLE audit_events DROP COLUMN IF EXISTS prev_hash;
ALTER TABLE audit_events DROP COLUMN IF EXISTS hash;
ALTER TABLE audit_events DROP COLUMN IF EXISTS method;
ALTER TABLE audit_events DROP COLUMN IF EXISTS path;
ALTER TABLE audit_events DROP COLUMN IF EXISTS status_code;
ALTER TABLE audit_events DROP COLUMN IF EXISTS source;
