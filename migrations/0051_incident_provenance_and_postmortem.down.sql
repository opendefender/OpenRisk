-- Revert 0051.

DROP TABLE IF EXISTS incident_post_mortems;

DROP INDEX IF EXISTS idx_incidents_origin;
DROP INDEX IF EXISTS idx_incidents_origin_rule;

ALTER TABLE incidents DROP COLUMN IF EXISTS origin;
ALTER TABLE incidents DROP COLUMN IF EXISTS origin_rule_id;
ALTER TABLE incidents DROP COLUMN IF EXISTS origin_rule_name;
ALTER TABLE incidents DROP COLUMN IF EXISTS origin_execution_id;
ALTER TABLE incidents DROP COLUMN IF EXISTS origin_detail;
ALTER TABLE incidents DROP COLUMN IF EXISTS risk_ids;
ALTER TABLE incidents DROP COLUMN IF EXISTS asset_ids;
ALTER TABLE incidents DROP COLUMN IF EXISTS stakeholders;
