-- Tenant-scope teams (isolation audit).
-- The teams table had no tenant column, so an admin of one tenant could list,
-- edit, delete or add members to another tenant's teams. Add tenant_id and index
-- it (RULE #2). Idempotent.
ALTER TABLE teams ADD COLUMN IF NOT EXISTS tenant_id uuid;

-- Best-effort backfill: teams have no direct creator column, so attribute each
-- team to the tenant of its first member when resolvable; leave NULL otherwise
-- (an orphaned team no tenant will match — safe by default).
UPDATE teams t
SET tenant_id = om.organization_id
FROM team_members tm
JOIN organization_members om ON om.user_id = tm.user_id
WHERE t.tenant_id IS NULL AND tm.team_id = t.id;

CREATE INDEX IF NOT EXISTS idx_teams_tenant ON teams (tenant_id);
