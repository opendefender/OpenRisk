-- 0050 — A real approval workflow: request types, named approvers, quorum,
-- parallel chains, and deadlines that actually close.

ALTER TABLE approval_workflows ADD COLUMN IF NOT EXISTS request_type     VARCHAR(64);
ALTER TABLE approval_workflows ADD COLUMN IF NOT EXISTS mode             VARCHAR(16) NOT NULL DEFAULT 'sequential';
ALTER TABLE approval_workflows ADD COLUMN IF NOT EXISTS expires_in_hours INTEGER NOT NULL DEFAULT 0;

ALTER TABLE approval_requests ADD COLUMN IF NOT EXISTS mode         VARCHAR(16) NOT NULL DEFAULT 'sequential';
ALTER TABLE approval_requests ADD COLUMN IF NOT EXISTS expires_at   TIMESTAMPTZ;
ALTER TABLE approval_requests ADD COLUMN IF NOT EXISTS request_type VARCHAR(64);

-- Existing workflows are sequential and never expire; that is what they have
-- been doing, so recording it is a statement of fact rather than a new policy.
UPDATE approval_workflows SET mode = 'sequential' WHERE mode IS NULL OR mode = '';
UPDATE approval_requests  SET mode = 'sequential' WHERE mode IS NULL OR mode = '';

-- The risk lifecycle already enforces this type: RESIDUAL_ACCEPTED requires an
-- approved risk_acceptance request. Backfill the catalogue key on workflows that
-- were already bound to that pair, so the UI can name them.
UPDATE approval_workflows
   SET request_type = 'risk_acceptance'
 WHERE request_type IS NULL AND entity_type = 'risk_acceptance' AND action = 'accept';

UPDATE approval_requests
   SET request_type = 'risk_acceptance'
 WHERE request_type IS NULL AND entity_type = 'risk_acceptance';

CREATE INDEX IF NOT EXISTS idx_approval_workflows_request_type ON approval_workflows (request_type);
CREATE INDEX IF NOT EXISTS idx_approval_requests_request_type  ON approval_requests (request_type);
CREATE INDEX IF NOT EXISTS idx_approval_requests_expires_at    ON approval_requests (expires_at);
