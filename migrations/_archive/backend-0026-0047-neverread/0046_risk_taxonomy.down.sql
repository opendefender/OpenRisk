-- Migration: 0046_risk_taxonomy.down.sql
-- Reverse 0046: drop the category vocabulary and the risk↔control mappings.
--
-- `risks.frameworks` and `risks.control_ids` were FROZEN rather than dropped by
-- the up migration precisely so this rollback loses nothing that existed before
-- it: both columns still hold their original values.
--
-- What is NOT undone: step 3 appended the unresolved framework strings to
-- `tags`. Those tags stay. Removing them would mean deleting labels the user may
-- have edited since, and the values are still present in `frameworks` anyway —
-- so the only effect of leaving them is a harmless duplicate, where removing
-- them risks destroying real work.

DROP INDEX IF EXISTS idx_risk_control_mappings_unique_framework;
DROP INDEX IF EXISTS idx_risk_control_mappings_unique_control;
DROP INDEX IF EXISTS idx_risk_control_mappings_deleted_at;
DROP INDEX IF EXISTS idx_risk_control_mappings_control;
DROP INDEX IF EXISTS idx_risk_control_mappings_framework;
DROP INDEX IF EXISTS idx_risk_control_mappings_risk;
DROP INDEX IF EXISTS idx_risk_control_mappings_tenant;
DROP TABLE IF EXISTS risk_control_mappings;

DROP INDEX IF EXISTS idx_risks_category_id;
ALTER TABLE risks DROP COLUMN IF EXISTS category_id;

DROP INDEX IF EXISTS idx_risk_categories_deleted_at;
DROP INDEX IF EXISTS idx_risk_categories_active;
DROP INDEX IF EXISTS idx_risk_categories_tenant_slug;
DROP TABLE IF EXISTS risk_categories;
