-- #1478 — undo 0090: drop compliance_frameworks.catalog_version and catalog_revision, and nothing else.
-- The column is a label stamped at import; every other column, including
-- catalog_key, name and version, is untouched, so no existing row loses data
-- beyond the label itself. Applying 0090 again leaves the label empty until the
-- next import.
ALTER TABLE compliance_frameworks DROP COLUMN IF EXISTS catalog_version;
ALTER TABLE compliance_frameworks DROP COLUMN IF EXISTS catalog_revision;
