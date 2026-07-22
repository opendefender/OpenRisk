-- Migration: 0037_add_governance.up.sql
-- Purpose: back the Governance module (spec §15 « Gouvernance »).
-- Four tenant-scoped tables:
--   1. audit_events       — the immutable, append-only audit trail (who/what/when/ip/before→after).
--   2. delegations        — time-boxed grants of one user's rights to another.
--   3. approval_workflows  — configurable Maker-Checker chains (trigger = entity_type + action).
--   4. approval_requests   — live runs of a workflow (state machine + embedded decisions).
--
-- GORM's AutoMigrate already creates all four (they are registered in AutoMigrate);
-- this migration exists so a migrations-only deploy is self-sufficient.

-- 1. Immutable audit trail (append-only — no updated_at, no soft delete).
CREATE TABLE IF NOT EXISTS audit_events (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      UUID NOT NULL,
    actor_id       UUID,
    action         VARCHAR(24),
    entity_type    VARCHAR(64),
    entity_id      VARCHAR(128),
    summary        TEXT,
    before         JSONB,
    after          JSONB,
    changed_fields JSONB,
    ip_address     VARCHAR(64),
    user_agent     TEXT,
    request_id     VARCHAR(64),
    created_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_audit_events_tenant      ON audit_events (tenant_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_actor       ON audit_events (actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_action      ON audit_events (action);
CREATE INDEX IF NOT EXISTS idx_audit_events_entity_type ON audit_events (entity_type);
CREATE INDEX IF NOT EXISTS idx_audit_events_entity_id   ON audit_events (entity_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_created_at  ON audit_events (created_at);

-- 2. Delegations.
CREATE TABLE IF NOT EXISTS delegations (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL,
    delegator_id UUID NOT NULL,
    delegate_id  UUID NOT NULL,
    reason       TEXT,
    permissions  JSONB,
    status       VARCHAR(16) NOT NULL DEFAULT 'active',
    starts_at    TIMESTAMPTZ,
    ends_at      TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ,
    created_by   UUID,
    created_at   TIMESTAMPTZ,
    updated_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_delegations_tenant    ON delegations (tenant_id);
CREATE INDEX IF NOT EXISTS idx_delegations_delegator ON delegations (delegator_id);
CREATE INDEX IF NOT EXISTS idx_delegations_delegate  ON delegations (delegate_id);
CREATE INDEX IF NOT EXISTS idx_delegations_status    ON delegations (status);
CREATE INDEX IF NOT EXISTS idx_delegations_starts_at ON delegations (starts_at);
CREATE INDEX IF NOT EXISTS idx_delegations_ends_at   ON delegations (ends_at);

-- 3. Approval workflows (config).
CREATE TABLE IF NOT EXISTS approval_workflows (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    name        VARCHAR(160) NOT NULL,
    description TEXT,
    entity_type VARCHAR(64),
    action      VARCHAR(64),
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    steps       JSONB,
    created_by  UUID,
    created_at  TIMESTAMPTZ,
    updated_at  TIMESTAMPTZ,
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_approval_workflows_tenant     ON approval_workflows (tenant_id);
CREATE INDEX IF NOT EXISTS idx_approval_workflows_entity     ON approval_workflows (entity_type);
CREATE INDEX IF NOT EXISTS idx_approval_workflows_enabled    ON approval_workflows (enabled);
CREATE INDEX IF NOT EXISTS idx_approval_workflows_deleted_at ON approval_workflows (deleted_at);

-- 4. Approval requests (runtime state machine).
CREATE TABLE IF NOT EXISTS approval_requests (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    workflow_id   UUID,
    workflow_name VARCHAR(160),
    entity_type   VARCHAR(64),
    entity_id     VARCHAR(128),
    action        VARCHAR(64),
    title         VARCHAR(255),
    description   TEXT,
    payload       JSONB,
    status        VARCHAR(16) NOT NULL DEFAULT 'pending',
    current_step  INTEGER NOT NULL DEFAULT 0,
    steps         JSONB,
    decisions     JSONB,
    requested_by  UUID,
    resolved_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ,
    updated_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_approval_requests_tenant   ON approval_requests (tenant_id);
CREATE INDEX IF NOT EXISTS idx_approval_requests_workflow ON approval_requests (workflow_id);
CREATE INDEX IF NOT EXISTS idx_approval_requests_entity   ON approval_requests (entity_type);
CREATE INDEX IF NOT EXISTS idx_approval_requests_status   ON approval_requests (status);
CREATE INDEX IF NOT EXISTS idx_approval_requests_by       ON approval_requests (requested_by);
