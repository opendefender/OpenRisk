-- Revert tenant scoping of teams.
DROP INDEX IF EXISTS idx_teams_tenant;
ALTER TABLE teams DROP COLUMN IF EXISTS tenant_id;
