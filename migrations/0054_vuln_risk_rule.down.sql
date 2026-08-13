-- Reverse of 0054_vuln_risk_rule.

DROP INDEX IF EXISTS idx_risks_draft_review;
DROP INDEX IF EXISTS idx_risks_source_vulnerability;

ALTER TABLE risks DROP COLUMN IF EXISTS source_rule_reason;
ALTER TABLE risks DROP COLUMN IF EXISTS source_vulnerability_id;

DROP TABLE IF EXISTS vuln_risk_rules;
