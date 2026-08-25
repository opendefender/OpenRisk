-- OR26-03 — deferrable MFA enrolment.
--
-- Two changes:
--   1. mfa_policies: one row per tenant holding how many days a privileged
--      member may defer enrolment. An absent row means the shipped default
--      (7 days), so a tenant that never opens the setting behaves exactly like
--      one that saved the defaults.
--   2. organization_members gains mfa_grace_started_at — the anchor the
--      countdown runs from. Backfilled to when the membership began, NOT to
--      now(): a six-month-old production administrator must not be handed a
--      fresh week, while an organization created today (the evaluation case
--      this issue is about) gets its full window.

BEGIN;

-- 1. Tenant MFA policy --------------------------------------------------------

CREATE TABLE IF NOT EXISTS mfa_policies (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      UUID        NOT NULL,
    grace_days     INTEGER     NOT NULL DEFAULT 7,
    updated_by_id  UUID,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- The bound is declared here as well as in domain.MFAPolicy.Validate: a
    -- direct SQL edit must not be able to express "never require MFA".
    CONSTRAINT mfa_policies_grace_days_bounds CHECK (grace_days BETWEEN 0 AND 90)
);

-- One policy per tenant. The unique index is what makes "read the tenant's
-- policy" a single-row lookup rather than a question with several answers.
CREATE UNIQUE INDEX IF NOT EXISTS uq_mfa_policies_tenant ON mfa_policies (tenant_id);

-- 2. Grace anchor -------------------------------------------------------------

ALTER TABLE organization_members
    ADD COLUMN IF NOT EXISTS mfa_grace_started_at TIMESTAMPTZ;

UPDATE organization_members
   SET mfa_grace_started_at = COALESCE(joined_at, created_at)
 WHERE mfa_grace_started_at IS NULL;

COMMIT;
