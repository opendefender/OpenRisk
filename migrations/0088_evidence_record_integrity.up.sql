-- #554 — evidence record integrity: content hash, observation period,
-- collection method, collector identity and an optional signature.
--
-- Expand only: columns are added, nothing is dropped or renamed. Every column
-- is NOT NULL with an empty default (or nullable for the two instants), so the
-- code that wrote evidences before this migration keeps working while it rolls.
--
-- content_hash is NOT backfilled: a legacy blob was never hashed, and hashing
-- today's bytes would certify whatever is on disk now. Those rows stay with an
-- empty hash and are served flagged "unverified".
--
-- Idempotent: AutoMigrate and database.PrepareForAutoMigrate add the same
-- columns and run the same backfill on boot. The backfill touches only rows
-- that still carry the empty value, so a late replay is a no-op.
--
-- The source -> method mapping is domain.CollectionMethodForSource:
--   manual -> manual_upload, integration -> api_pull,
--   scanner -> automated_collector, automation -> automated_collector.

ALTER TABLE evidences ADD COLUMN IF NOT EXISTS content_hash        varchar(64) NOT NULL DEFAULT '';
ALTER TABLE evidences ADD COLUMN IF NOT EXISTS hash_algorithm      varchar(16) NOT NULL DEFAULT '';
ALTER TABLE evidences ADD COLUMN IF NOT EXISTS observed_from       timestamptz;
ALTER TABLE evidences ADD COLUMN IF NOT EXISTS observed_to         timestamptz;
ALTER TABLE evidences ADD COLUMN IF NOT EXISTS collection_method   varchar(24) NOT NULL DEFAULT '';
ALTER TABLE evidences ADD COLUMN IF NOT EXISTS collector_kind      varchar(16) NOT NULL DEFAULT '';
ALTER TABLE evidences ADD COLUMN IF NOT EXISTS collector_ref       varchar(128) NOT NULL DEFAULT '';
ALTER TABLE evidences ADD COLUMN IF NOT EXISTS signature           text NOT NULL DEFAULT '';
ALTER TABLE evidences ADD COLUMN IF NOT EXISTS signature_algorithm varchar(32) NOT NULL DEFAULT '';

UPDATE evidences
   SET collection_method = CASE source
         WHEN 'integration' THEN 'api_pull'
         WHEN 'scanner'     THEN 'automated_collector'
         WHEN 'automation'  THEN 'automated_collector'
         ELSE 'manual_upload'
       END
 WHERE collection_method = '';

-- A legacy row is a point-in-time observation at its collection instant.
UPDATE evidences
   SET observed_from = collected_at,
       observed_to   = collected_at
 WHERE observed_from IS NULL
   AND observed_to IS NULL;
