-- #903 — the executive view's quarter-on-quarter deltas.
--
-- The daily snapshot keeps, next to the exposure score, the annualised
-- exposure (XAF), the live critical risks and the average compliance
-- coverage. Nullable: a figure whose source failed that day is a gap.

ALTER TABLE tenant_score_snapshots ADD COLUMN IF NOT EXISTS ale_xaf double precision;
ALTER TABLE tenant_score_snapshots ADD COLUMN IF NOT EXISTS critical_risks integer;
ALTER TABLE tenant_score_snapshots ADD COLUMN IF NOT EXISTS compliance_pct double precision;
