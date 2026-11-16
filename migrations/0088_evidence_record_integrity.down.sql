-- #554 — undo 0088: drop the nine record-integrity columns of evidences, and
-- nothing else. This is a rollback of a release, not a routine step: the content
-- hashes, observation periods, collector identities and signatures recorded
-- since 0088 ran are lost with the columns. The up migration is expand-only.
ALTER TABLE evidences DROP COLUMN IF EXISTS content_hash;
ALTER TABLE evidences DROP COLUMN IF EXISTS hash_algorithm;
ALTER TABLE evidences DROP COLUMN IF EXISTS observed_from;
ALTER TABLE evidences DROP COLUMN IF EXISTS observed_to;
ALTER TABLE evidences DROP COLUMN IF EXISTS collection_method;
ALTER TABLE evidences DROP COLUMN IF EXISTS collector_kind;
ALTER TABLE evidences DROP COLUMN IF EXISTS collector_ref;
ALTER TABLE evidences DROP COLUMN IF EXISTS signature;
ALTER TABLE evidences DROP COLUMN IF EXISTS signature_algorithm;
