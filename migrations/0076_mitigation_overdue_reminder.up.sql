-- #757 — one "deadline passed" reminder per mitigation plan.
--
-- The D-7 / D-1 stamps already exist; this adds the third. Idempotent: boot's
-- AutoMigrate adds the same column from domain.Mitigation, and
-- database.PrepareForAutoMigrate runs the same backfill on deployments that
-- boot without DATABASE_URL (and so never reach this file).
--
-- Plans that are ALREADY late when this ships are stamped now. Without that,
-- the first sweep after the deploy would mail every overdue plan in every
-- tenant at once, some of them months late — a flood that teaches people to
-- filter the sender. New lateness, from today on, is announced once. D-1 and
-- D-7 are closed with it, so a backfilled plan cannot fall through to an
-- earlier nudge worded as overdue.

ALTER TABLE mitigations ADD COLUMN IF NOT EXISTS reminder_overdue_sent_at timestamptz;

UPDATE mitigations
   SET reminder_overdue_sent_at = NOW(),
       reminder_d1_sent_at = COALESCE(reminder_d1_sent_at, NOW()),
       reminder_d7_sent_at = COALESCE(reminder_d7_sent_at, NOW())
 WHERE reminder_overdue_sent_at IS NULL
   AND due_date IS NOT NULL
   AND due_date < date_trunc('day', NOW());
