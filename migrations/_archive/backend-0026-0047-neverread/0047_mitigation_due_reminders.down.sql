-- Migration: 0047_mitigation_due_reminders.down.sql
-- Reverse 0047: drop the reminder bookkeeping.
--
-- `progress` is NOT restored to its previous values: 0047 corrected rows that
-- were wrong (a DONE plan reading 0 %), and putting the wrong numbers back would
-- be a regression, not a rollback.

DROP INDEX IF EXISTS idx_mitigations_due_date;

ALTER TABLE mitigations
    DROP COLUMN IF EXISTS reminder_d7_sent_at,
    DROP COLUMN IF EXISTS reminder_d1_sent_at;
