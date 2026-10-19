ALTER TABLE IF EXISTS board_reports DROP COLUMN IF EXISTS acceptances_snapshot;

DROP INDEX IF EXISTS idx_risks_acceptance_expired_at;
DROP INDEX IF EXISTS idx_risks_acceptance_expires_at;

ALTER TABLE risks DROP COLUMN IF EXISTS acceptance_expired_at;
ALTER TABLE risks DROP COLUMN IF EXISTS acceptance_reminder7_at;
ALTER TABLE risks DROP COLUMN IF EXISTS acceptance_reminder30_at;
ALTER TABLE risks DROP COLUMN IF EXISTS acceptance_expires_at;
ALTER TABLE risks DROP COLUMN IF EXISTS acceptance_approved_by;
ALTER TABLE risks DROP COLUMN IF EXISTS acceptance_approval_id;
ALTER TABLE risks DROP COLUMN IF EXISTS accepted_at;
