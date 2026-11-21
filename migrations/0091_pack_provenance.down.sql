-- Reverses 0091: the indexes first, then the added columns.
DROP INDEX IF EXISTS ux_control_crosswalks_pack_item;
DROP INDEX IF EXISTS ux_compliance_controls_pack_item;

ALTER TABLE controls DROP COLUMN IF EXISTS pack_reverted_by;

ALTER TABLE control_crosswalks DROP COLUMN IF EXISTS pack_reverted_by;
ALTER TABLE control_crosswalks DROP COLUMN IF EXISTS pack_retired_in;
ALTER TABLE control_crosswalks DROP COLUMN IF EXISTS pack_base_hash;
ALTER TABLE control_crosswalks DROP COLUMN IF EXISTS pack_base;
ALTER TABLE control_crosswalks DROP COLUMN IF EXISTS pack_version;
ALTER TABLE control_crosswalks DROP COLUMN IF EXISTS pack_item_key;
ALTER TABLE control_crosswalks DROP COLUMN IF EXISTS pack_key;

ALTER TABLE compliance_controls DROP COLUMN IF EXISTS pack_reverted_by;
ALTER TABLE compliance_controls DROP COLUMN IF EXISTS pack_retired_in;
ALTER TABLE compliance_controls DROP COLUMN IF EXISTS pack_base_hash;
ALTER TABLE compliance_controls DROP COLUMN IF EXISTS pack_base;
ALTER TABLE compliance_controls DROP COLUMN IF EXISTS pack_version;
ALTER TABLE compliance_controls DROP COLUMN IF EXISTS pack_item_key;
ALTER TABLE compliance_controls DROP COLUMN IF EXISTS pack_key;

ALTER TABLE compliance_frameworks DROP COLUMN IF EXISTS catalog_pending_kinds;
ALTER TABLE compliance_frameworks DROP COLUMN IF EXISTS catalog_crosswalks_version;
ALTER TABLE compliance_frameworks DROP COLUMN IF EXISTS catalog_content_version;
