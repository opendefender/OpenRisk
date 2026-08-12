-- Revert 0050.

DROP INDEX IF EXISTS idx_approval_workflows_request_type;
DROP INDEX IF EXISTS idx_approval_requests_request_type;
DROP INDEX IF EXISTS idx_approval_requests_expires_at;

ALTER TABLE approval_workflows DROP COLUMN IF EXISTS request_type;
ALTER TABLE approval_workflows DROP COLUMN IF EXISTS mode;
ALTER TABLE approval_workflows DROP COLUMN IF EXISTS expires_in_hours;

ALTER TABLE approval_requests DROP COLUMN IF EXISTS mode;
ALTER TABLE approval_requests DROP COLUMN IF EXISTS expires_at;
ALTER TABLE approval_requests DROP COLUMN IF EXISTS request_type;
