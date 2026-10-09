ALTER TABLE tenant_score_snapshots DROP COLUMN IF EXISTS compliance_pct;
ALTER TABLE tenant_score_snapshots DROP COLUMN IF EXISTS critical_risks;
ALTER TABLE tenant_score_snapshots DROP COLUMN IF EXISTS ale_xaf;
