-- Reverse of 0062. Dropping the index removes the database backstop only; the
-- repository still supersedes the previous token before issuing a new one. The
-- tables themselves belong to AutoMigrate and are not touched here.

BEGIN;

DROP INDEX IF EXISTS idx_vendor_assessment_tokens_one_active;

COMMIT;
