-- Migration: 0047_mitigation_due_reminders.up.sql
-- Purpose: back the D-7 / D-1 deadline nudges, and repair the progress values
-- that the old rule left wrong.
--
-- `due_date` already existed on mitigations but nothing ever read it: no badge,
-- no reminder, no filter. These two columns are the bookkeeping that lets a
-- reminder be sent ONCE — an hourly sweep with no memory would send the same
-- nudge twenty-four times a day.

ALTER TABLE mitigations
    ADD COLUMN IF NOT EXISTS reminder_d7_sent_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS reminder_d1_sent_at TIMESTAMPTZ;

-- The sweep's predicate: unfinished, has a deadline, still owes a nudge.
CREATE INDEX IF NOT EXISTS idx_mitigations_due_date ON mitigations (due_date)
    WHERE due_date IS NOT NULL AND deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- Repair the progress column.
--
-- The old rule returned 0 for any plan with no sub-actions, whatever its status
-- — so a plan somebody had marked DONE still read 0 %. That is the reported
-- "progression de mitigation bloquée à 0 %". The rule now lives in
-- domain.ComputeMitigationProgress and is applied on every mutation; this
-- statement brings existing rows in line with it exactly once.
--
--   with sub-actions    → completed / total
--   without sub-actions → planned 0 · in_progress|review 50 · done 100 · cancelled 0
-- ---------------------------------------------------------------------------
UPDATE mitigations m
   SET progress = COALESCE(sub.computed, base.fallback)
  FROM (
        SELECT
            CASE
                WHEN upper(m2.status) = 'DONE' THEN 100
                WHEN upper(m2.status) IN ('IN_PROGRESS', 'REVIEW') THEN 50
                ELSE 0
            END AS fallback,
            m2.id
          FROM mitigations m2
  ) base
  LEFT JOIN (
        SELECT s.mitigation_id,
               (COUNT(*) FILTER (WHERE s.completed) * 100) / NULLIF(COUNT(*), 0) AS computed
          FROM mitigation_subactions s
         WHERE s.deleted_at IS NULL
         GROUP BY s.mitigation_id
  ) sub ON sub.mitigation_id = base.id
 WHERE base.id = m.id
   AND m.deleted_at IS NULL;

COMMENT ON COLUMN mitigations.progress IS
    'COMPUTED, never client-supplied: completed sub-actions / total, or the coarse status when there are none. Recalculated server-side on every mutation (domain.ComputeMitigationProgress).';
