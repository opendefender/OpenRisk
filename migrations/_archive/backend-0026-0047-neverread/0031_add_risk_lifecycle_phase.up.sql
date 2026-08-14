-- Migration: 0031_add_risk_lifecycle_phase.up.sql
-- Purpose: give the real Risk entity an explicit ISO 31000 lifecycle phase
-- (Identifier → Analyser → Évaluer → Traiter → Surveiller → Clôturer), surfaced
-- live in the register drawer's "Cycle de vie" stepper. Orthogonal to `status`.
--
-- GORM's AutoMigrate already adds this column (Risk is in AutoMigrate) with a
-- DEFAULT of 'identified', so this runs mainly to (a) be self-sufficient for a
-- migrations-only deploy and (b) backfill a *sensible* phase for pre-existing
-- rows from their coarse status instead of flattening everything to 'identified'.

ALTER TABLE risks
    ADD COLUMN IF NOT EXISTS lifecycle_phase VARCHAR(20) NOT NULL DEFAULT 'identified';

-- Best-effort one-time backfill from status. Only touches rows still sitting at
-- the freshly-defaulted 'identified' so it never overwrites a phase already set
-- by the app. Mapping: resolved → closed, mitigated/accepted/in-progress →
-- treated, everything else stays 'identified'.
UPDATE risks
SET lifecycle_phase = CASE
        WHEN status IN ('closed', 'CLOSED') THEN 'closed'
        WHEN status IN ('mitigated', 'MITIGATED', 'accepted', 'ACCEPTED') THEN 'treated'
        WHEN status IN ('in_progress', 'ACTIVE') THEN 'treated'
        ELSE 'identified'
    END
WHERE lifecycle_phase = 'identified'
  AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_risks_lifecycle_phase ON risks (lifecycle_phase);
