-- Migration: 0044_generalized_ownership.up.sql
-- Purpose: give every actionable entity the same three accountability slots —
-- owner_id (responsable, answers for the outcome), assignee_id (exécutant, does
-- the work), reviewer_id (validateur, signs off).
--
-- Why: the model conflated them. A risk had `assigned_to` + a free-text `owner`
-- string; a mitigation had a jsonb array `assigned_to`; an incident had a
-- free-text `assigned_to`; evidence had only `uploaded_by`. Nothing could answer
-- "what is assigned to me", and "impossible d'assigner un risque" was the
-- user-visible consequence.
--
-- All three columns are NULLABLE by design: an entity ingested by a machine
-- (CTI auto-created risk, scanner finding) has no human author, and a NOT NULL
-- constraint would block ingestion rather than improve accountability.
--
-- GORM AutoMigrate also adds these (domain.Ownership is embedded on all five
-- models); this migration exists so a migrations-only deploy is self-sufficient
-- and so the BACKFILL below is explicit and reviewable.

-- ---------------------------------------------------------------- risks
ALTER TABLE risks
    ADD COLUMN IF NOT EXISTS owner_id    UUID,
    ADD COLUMN IF NOT EXISTS assignee_id UUID;
-- reviewer_id already existed on risks (declared by hand before the embed).
ALTER TABLE risks
    ADD COLUMN IF NOT EXISTS reviewer_id UUID;

CREATE INDEX IF NOT EXISTS idx_risks_owner_id    ON risks (owner_id);
CREATE INDEX IF NOT EXISTS idx_risks_assignee_id ON risks (assignee_id);
CREATE INDEX IF NOT EXISTS idx_risks_reviewer_id ON risks (reviewer_id);

-- Backfill: the owner of an existing risk is its creator (the only defensible
-- answer — nobody else ever claimed it). The assignee comes from the legacy
-- `assigned_to` column, which is exactly what it meant.
UPDATE risks SET owner_id    = created_by  WHERE owner_id    IS NULL AND created_by IS NOT NULL;
UPDATE risks SET assignee_id = assigned_to WHERE assignee_id IS NULL AND assigned_to IS NOT NULL;

-- The legacy free-text `owner` column sometimes holds a user UUID rather than an
-- email. Recover those; leave anything that is not a UUID alone (it is an email
-- or a team name and is preserved in the legacy column).
UPDATE risks
   SET owner_id = owner::uuid
 WHERE owner_id IS NULL
   AND owner ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';

-- ----------------------------------------------------------- mitigations
ALTER TABLE mitigations
    ADD COLUMN IF NOT EXISTS owner_id    UUID,
    ADD COLUMN IF NOT EXISTS assignee_id UUID,
    ADD COLUMN IF NOT EXISTS reviewer_id UUID;

CREATE INDEX IF NOT EXISTS idx_mitigations_owner_id    ON mitigations (owner_id);
CREATE INDEX IF NOT EXISTS idx_mitigations_assignee_id ON mitigations (assignee_id);
CREATE INDEX IF NOT EXISTS idx_mitigations_reviewer_id ON mitigations (reviewer_id);

UPDATE mitigations SET owner_id = created_by WHERE owner_id IS NULL AND created_by IS NOT NULL;

-- `assigned_to` is a jsonb array of user ids. The first element is the de-facto
-- assignee; the array is left intact for legacy readers.
UPDATE mitigations
   SET assignee_id = (assigned_to->>0)::uuid
 WHERE assignee_id IS NULL
   AND jsonb_typeof(assigned_to) = 'array'
   AND jsonb_array_length(assigned_to) > 0
   AND (assigned_to->>0) ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';

-- A mitigation that was already approved has a validator on record.
UPDATE mitigations SET reviewer_id = approved_by WHERE reviewer_id IS NULL AND approved_by IS NOT NULL;

-- ------------------------------------------------------------- incidents
ALTER TABLE incidents
    ADD COLUMN IF NOT EXISTS owner_id    UUID,
    ADD COLUMN IF NOT EXISTS assignee_id UUID,
    ADD COLUMN IF NOT EXISTS reviewer_id UUID;

CREATE INDEX IF NOT EXISTS idx_incidents_owner_id    ON incidents (owner_id);
CREATE INDEX IF NOT EXISTS idx_incidents_assignee_id ON incidents (assignee_id);
CREATE INDEX IF NOT EXISTS idx_incidents_reviewer_id ON incidents (reviewer_id);

-- reported_by / assigned_to are free text (historically an email OR a user id).
-- Only UUID-shaped values can be recovered; emails stay in the legacy columns.
UPDATE incidents
   SET owner_id = reported_by::uuid
 WHERE owner_id IS NULL
   AND reported_by ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';

UPDATE incidents
   SET assignee_id = assigned_to::uuid
 WHERE assignee_id IS NULL
   AND assigned_to ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';

-- ------------------------------------------------------ remediation_plans
ALTER TABLE remediation_plans
    ADD COLUMN IF NOT EXISTS owner_id    UUID,
    ADD COLUMN IF NOT EXISTS assignee_id UUID,
    ADD COLUMN IF NOT EXISTS reviewer_id UUID;

CREATE INDEX IF NOT EXISTS idx_remediation_plans_owner_id    ON remediation_plans (owner_id);
CREATE INDEX IF NOT EXISTS idx_remediation_plans_assignee_id ON remediation_plans (assignee_id);
CREATE INDEX IF NOT EXISTS idx_remediation_plans_reviewer_id ON remediation_plans (reviewer_id);

UPDATE remediation_plans SET owner_id    = created_by  WHERE owner_id    IS NULL AND created_by  IS NOT NULL;
UPDATE remediation_plans SET assignee_id = assigned_to WHERE assignee_id IS NULL AND assigned_to IS NOT NULL;

-- ------------------------------------------------------ control_evidences
ALTER TABLE control_evidences
    ADD COLUMN IF NOT EXISTS owner_id    UUID,
    ADD COLUMN IF NOT EXISTS assignee_id UUID,
    ADD COLUMN IF NOT EXISTS reviewer_id UUID;

CREATE INDEX IF NOT EXISTS idx_control_evidences_owner_id    ON control_evidences (owner_id);
CREATE INDEX IF NOT EXISTS idx_control_evidences_assignee_id ON control_evidences (assignee_id);
CREATE INDEX IF NOT EXISTS idx_control_evidences_reviewer_id ON control_evidences (reviewer_id);

UPDATE control_evidences SET owner_id = uploaded_by WHERE owner_id IS NULL AND uploaded_by IS NOT NULL;
