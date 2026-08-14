-- Migration: 0044_generalized_ownership.down.sql
-- Reverse 0044: drop the three ownership slots from every actionable entity.
--
-- Safe to run: every value written by 0044's backfill was DERIVED from a column
-- that is still present (created_by / assigned_to / approved_by / uploaded_by /
-- reported_by), so nothing that existed before 0044 is lost. Assignments made
-- AFTER 0044 through the new UserPicker are lost — that is inherent to dropping
-- the columns that hold them.
--
-- risks.reviewer_id predates this migration (it was hand-declared on the model),
-- so it is NOT dropped here.

DROP INDEX IF EXISTS idx_control_evidences_owner_id;
DROP INDEX IF EXISTS idx_control_evidences_assignee_id;
DROP INDEX IF EXISTS idx_control_evidences_reviewer_id;
ALTER TABLE control_evidences
    DROP COLUMN IF EXISTS owner_id,
    DROP COLUMN IF EXISTS assignee_id,
    DROP COLUMN IF EXISTS reviewer_id;

DROP INDEX IF EXISTS idx_remediation_plans_owner_id;
DROP INDEX IF EXISTS idx_remediation_plans_assignee_id;
DROP INDEX IF EXISTS idx_remediation_plans_reviewer_id;
ALTER TABLE remediation_plans
    DROP COLUMN IF EXISTS owner_id,
    DROP COLUMN IF EXISTS assignee_id,
    DROP COLUMN IF EXISTS reviewer_id;

DROP INDEX IF EXISTS idx_incidents_owner_id;
DROP INDEX IF EXISTS idx_incidents_assignee_id;
DROP INDEX IF EXISTS idx_incidents_reviewer_id;
ALTER TABLE incidents
    DROP COLUMN IF EXISTS owner_id,
    DROP COLUMN IF EXISTS assignee_id,
    DROP COLUMN IF EXISTS reviewer_id;

DROP INDEX IF EXISTS idx_mitigations_owner_id;
DROP INDEX IF EXISTS idx_mitigations_assignee_id;
DROP INDEX IF EXISTS idx_mitigations_reviewer_id;
ALTER TABLE mitigations
    DROP COLUMN IF EXISTS owner_id,
    DROP COLUMN IF EXISTS assignee_id,
    DROP COLUMN IF EXISTS reviewer_id;

DROP INDEX IF EXISTS idx_risks_owner_id;
DROP INDEX IF EXISTS idx_risks_assignee_id;
ALTER TABLE risks
    DROP COLUMN IF EXISTS owner_id,
    DROP COLUMN IF EXISTS assignee_id;
