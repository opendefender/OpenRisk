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

--
-- The stamp belongs to the moment the column appears, and only then (#1001).
-- When it already exists, AutoMigrate or PrepareForAutoMigrate added it and
-- ran this backfill back then. Running it again, as a compose install's first
-- pass through the SQL layer would, marks every plan that has gone late since
-- as already reminded, and those reminders are never sent.

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
         WHERE table_schema = current_schema()
           AND table_name = 'mitigations'
           AND column_name = 'reminder_overdue_sent_at'
    ) THEN
        ALTER TABLE mitigations ADD COLUMN reminder_overdue_sent_at timestamptz;

        UPDATE mitigations
           SET reminder_overdue_sent_at = NOW(),
               reminder_d1_sent_at = COALESCE(reminder_d1_sent_at, NOW()),
               reminder_d7_sent_at = COALESCE(reminder_d7_sent_at, NOW())
         WHERE due_date IS NOT NULL
           AND due_date < date_trunc('day', NOW());
    END IF;
END $$;
