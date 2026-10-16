-- #811 — a residual-risk acceptance is time-bound.
--
-- Accepting residual risk used to hold forever. These columns record who
-- approved the acceptance, through which governance request, when it was
-- given and when it lapses, plus the two reminder stamps (D-30, D-7) and the
-- moment the system re-opened the risk for assessment.
--
-- Backfill: every risk already in residual_accepted gets an end date. The
-- acceptance date is unknown for those rows; last_mitigated_at is what the
-- transition stamped on entry, updated_at the fallback. The end date is that
-- date plus 12 months, but never sooner than 30 days from now, so an old
-- acceptance gets its D-30 notice instead of expiring at the first sweep.
-- The approver is left NULL: inventing one would put a false name in the
-- record. Idempotent: AutoMigrate adds the columns on boot too.

ALTER TABLE risks ADD COLUMN IF NOT EXISTS accepted_at timestamptz;
ALTER TABLE risks ADD COLUMN IF NOT EXISTS acceptance_approval_id uuid;
ALTER TABLE risks ADD COLUMN IF NOT EXISTS acceptance_approved_by uuid;
ALTER TABLE risks ADD COLUMN IF NOT EXISTS acceptance_expires_at timestamptz;
ALTER TABLE risks ADD COLUMN IF NOT EXISTS acceptance_reminder30_at timestamptz;
ALTER TABLE risks ADD COLUMN IF NOT EXISTS acceptance_reminder7_at timestamptz;
ALTER TABLE risks ADD COLUMN IF NOT EXISTS acceptance_expired_at timestamptz;

CREATE INDEX IF NOT EXISTS idx_risks_acceptance_expires_at ON risks (acceptance_expires_at);
CREATE INDEX IF NOT EXISTS idx_risks_acceptance_expired_at ON risks (acceptance_expired_at);

UPDATE risks
SET accepted_at = COALESCE(last_mitigated_at, updated_at),
    acceptance_expires_at = GREATEST(
        COALESCE(last_mitigated_at, updated_at) + INTERVAL '12 months',
        now() + INTERVAL '30 days')
WHERE lifecycle_state = 'residual_accepted'
  AND acceptance_expires_at IS NULL
  AND deleted_at IS NULL;

-- The board report snapshots the acceptances falling due, like the frameworks.
-- IF EXISTS: board_reports is created by AutoMigrate, not by a migration.
ALTER TABLE IF EXISTS board_reports ADD COLUMN IF NOT EXISTS acceptances_snapshot jsonb;
