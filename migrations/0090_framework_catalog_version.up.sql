-- #1478 — compliance_frameworks.catalog_version: the edition of the standard a
-- framework was imported from (e.g. '2022'), beside catalog_key.
--
-- DDL only, expand only: one new column, nothing altered, nothing backfilled.
-- Rows that exist before this migration keep '' — "unknown", never a guessed
-- value (a framework imported last year may not be the edition the catalogue
-- carries today). The import use case stamps the column on the next import.
--
-- Numbered 0090, the next free number after 0089.
--
-- Idempotent: AutoMigrate builds the same column from domain.ComplianceFramework,
-- and on a database where it ran first the statement below is a no-op.
--
-- compliance_frameworks already carries tenant_id; no new table, no new index.

ALTER TABLE compliance_frameworks ADD COLUMN IF NOT EXISTS catalog_version VARCHAR(64) NOT NULL DEFAULT '';
-- catalog_revision: fingerprint of OpenRisk's own catalogue content at the import
-- that stamped the catalogue. Same rules: expand only, no backfill, '' means unknown.
ALTER TABLE compliance_frameworks ADD COLUMN IF NOT EXISTS catalog_revision VARCHAR(64) NOT NULL DEFAULT '';
