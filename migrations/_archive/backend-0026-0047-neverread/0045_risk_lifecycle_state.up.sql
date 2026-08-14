-- Migration: 0045_risk_lifecycle_state.up.sql
-- Purpose: give the risk lifecycle ONE writable source of truth.
--
-- Until now two columns both claimed to say where a risk stood, and nothing
-- reconciled them:
--   * status          — open / in_progress / mitigated / accepted / closed,
--                       plus a legacy uppercase set (DRAFT / ACTIVE / …)
--   * lifecycle_phase — ISO 31000: identified / analyzed / evaluated / treated /
--                       monitored / closed
-- A risk could therefore read "mitigated" while sitting in the "treated" phase
-- with no completed mitigation anywhere. That is the "cycle de vie flou" bug.
--
-- lifecycle_state is now the only field written by the transition use case; the
-- other two are DERIVED from it on every write (domain.Risk.SetState) and keep
-- their columns, so every existing filter, pill and dashboard keeps working.
--
--   DRAFT → IDENTIFIED → ASSESSED → TREATMENT_PLANNED → IN_TREATMENT
--         → (RESIDUAL_ACCEPTED | MITIGATED) → CLOSED   ↘ REOPENED ↗

ALTER TABLE risks
    ADD COLUMN IF NOT EXISTS lifecycle_state VARCHAR(24) NOT NULL DEFAULT 'draft';

CREATE INDEX IF NOT EXISTS idx_risks_lifecycle_state ON risks (lifecycle_state);

-- Backfill. This mirrors domain.RiskStateFromLegacy exactly — keep the two in
-- step if either changes.
--
-- STATUS WINS over phase where they disagree: a resolution ("mitigated",
-- "accepted", "closed") is a stronger statement than a phase, and it is the one
-- users actually acted on in the UI.
UPDATE risks
   SET lifecycle_state = CASE
        -- 1. Resolutions, from the status (both vocabularies).
        WHEN lower(status) = 'closed'                         THEN 'closed'
        WHEN lower(status) = 'mitigated'                      THEN 'mitigated'
        WHEN lower(status) = 'accepted'                       THEN 'residual_accepted'
        WHEN status = 'DRAFT'                                 THEN 'draft'
        -- 2. Otherwise the ISO 31000 phase.
        WHEN lifecycle_phase = 'closed'                       THEN 'closed'
        WHEN lifecycle_phase IN ('treated', 'monitored')      THEN 'in_treatment'
        WHEN lifecycle_phase = 'evaluated'                    THEN 'treatment_planned'
        WHEN lifecycle_phase = 'analyzed'                     THEN 'assessed'
        WHEN lifecycle_phase = 'identified'                   THEN 'identified'
        -- 3. No phase recorded: fall back on the coarse status.
        WHEN lower(status) IN ('in_progress', 'active')       THEN 'in_treatment'
        ELSE 'identified'
   END;

-- Re-derive the two legacy columns from the state that was just computed, so
-- rows that disagreed BEFORE this migration stop disagreeing after it. This is
-- the only place the historical inconsistency is repaired; from here on
-- SetState keeps them aligned.
UPDATE risks
   SET status = CASE lifecycle_state
        WHEN 'draft'             THEN 'DRAFT'
        WHEN 'in_treatment'      THEN 'in_progress'
        WHEN 'mitigated'         THEN 'mitigated'
        WHEN 'residual_accepted' THEN 'accepted'
        WHEN 'closed'            THEN 'closed'
        ELSE 'open'
       END,
       lifecycle_phase = CASE lifecycle_state
        WHEN 'draft'             THEN 'identified'
        WHEN 'identified'        THEN 'identified'
        WHEN 'reopened'          THEN 'identified'
        WHEN 'assessed'          THEN 'analyzed'
        WHEN 'treatment_planned' THEN 'evaluated'
        WHEN 'in_treatment'      THEN 'treated'
        WHEN 'mitigated'         THEN 'monitored'
        WHEN 'residual_accepted' THEN 'monitored'
        WHEN 'closed'            THEN 'closed'
        ELSE 'identified'
       END;
