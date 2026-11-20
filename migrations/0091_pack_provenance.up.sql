-- #1483 (ADR 0013 D3/D4, D-121) — content version on frameworks, pack provenance
-- on imported rows.
--
-- DDL only, expand only: ADD COLUMN IF NOT EXISTS with defaults, and one partial
-- unique index. Nothing dropped, renamed, retyped or backfilled. Legacy rows keep
-- empty provenance and a '' stamp ("unknown"); the later pack upgrade adopts them.
--
-- Provenance is written only by the catalogue import. The columns are in no input
-- DTO and no API response.
--
-- Idempotent: AutoMigrate builds the same columns and index from the GORM tags
-- (domain.ComplianceFramework, ComplianceControl, ControlCrosswalk,
-- controlchain.ControlRow), same names and types, and on a database where it ran
-- first every statement below is a no-op.

ALTER TABLE compliance_frameworks ADD COLUMN IF NOT EXISTS catalog_content_version varchar(32) NOT NULL DEFAULT '';
ALTER TABLE compliance_frameworks ADD COLUMN IF NOT EXISTS catalog_crosswalks_version varchar(32) NOT NULL DEFAULT '';
ALTER TABLE compliance_frameworks ADD COLUMN IF NOT EXISTS catalog_pending_kinds jsonb NOT NULL DEFAULT '[]';

ALTER TABLE compliance_controls ADD COLUMN IF NOT EXISTS pack_key varchar(64) NOT NULL DEFAULT '';
ALTER TABLE compliance_controls ADD COLUMN IF NOT EXISTS pack_item_key varchar(255) NOT NULL DEFAULT '';
ALTER TABLE compliance_controls ADD COLUMN IF NOT EXISTS pack_version varchar(32) NOT NULL DEFAULT '';
ALTER TABLE compliance_controls ADD COLUMN IF NOT EXISTS pack_base jsonb;
ALTER TABLE compliance_controls ADD COLUMN IF NOT EXISTS pack_base_hash char(64);
ALTER TABLE compliance_controls ADD COLUMN IF NOT EXISTS pack_retired_in varchar(32) NOT NULL DEFAULT '';
ALTER TABLE compliance_controls ADD COLUMN IF NOT EXISTS pack_reverted_by uuid;

ALTER TABLE control_crosswalks ADD COLUMN IF NOT EXISTS pack_key varchar(64) NOT NULL DEFAULT '';
ALTER TABLE control_crosswalks ADD COLUMN IF NOT EXISTS pack_item_key varchar(255) NOT NULL DEFAULT '';
ALTER TABLE control_crosswalks ADD COLUMN IF NOT EXISTS pack_version varchar(32) NOT NULL DEFAULT '';
ALTER TABLE control_crosswalks ADD COLUMN IF NOT EXISTS pack_base jsonb;
ALTER TABLE control_crosswalks ADD COLUMN IF NOT EXISTS pack_base_hash char(64);
ALTER TABLE control_crosswalks ADD COLUMN IF NOT EXISTS pack_retired_in varchar(32) NOT NULL DEFAULT '';
ALTER TABLE control_crosswalks ADD COLUMN IF NOT EXISTS pack_reverted_by uuid;

-- Canonical controls table (0086): only pack_reverted_by, for a later revert.
ALTER TABLE controls ADD COLUMN IF NOT EXISTS pack_reverted_by uuid;

-- One live-or-deleted row per (tenant, framework, pack, item). Leads on tenant_id.
-- Deleted rows are NOT excluded: a deleted row is the tombstone the import reads.
CREATE UNIQUE INDEX IF NOT EXISTS ux_compliance_controls_pack_item
    ON compliance_controls (tenant_id, framework_id, pack_key, pack_item_key)
    WHERE pack_key <> '';

-- Same for links. The index also holds the two control ids (D-123): one tenant can
-- import a catalogue into two frameworks, which gives two links the same
-- pack_item_key, and (tenant_id, pack_key, pack_item_key) alone would refuse the
-- second import. ADR 0013 D4 is to be amended. Leads on tenant_id, deleted rows
-- included.
CREATE UNIQUE INDEX IF NOT EXISTS ux_control_crosswalks_pack_item
    ON control_crosswalks (tenant_id, pack_key, pack_item_key, source_control_id, target_control_id)
    WHERE pack_key <> '';
