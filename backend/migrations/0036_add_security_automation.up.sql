-- Migration: 0036_add_security_automation.up.sql
-- Purpose: back the Security Automation / SOAR engine (spec §10 « Automatisation »).
-- Three tenant-scoped tables:
--   1. automation_rules      — trigger + conditions + ordered action chain + SLA policy.
--   2. automation_executions — audit record of each rule firing (steps as jsonb).
--   3. sla_trackers          — live resolution countdowns with escalation state.
--
-- GORM's AutoMigrate already creates all three (they are registered in
-- AutoMigrate); this migration exists so a migrations-only deploy is self-sufficient.

CREATE TABLE IF NOT EXISTS automation_rules (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL,
    name              VARCHAR(160) NOT NULL,
    description       TEXT,
    enabled           BOOLEAN NOT NULL DEFAULT TRUE,
    trigger           VARCHAR(40) NOT NULL,
    conditions        JSONB,
    actions           JSONB,
    sla               JSONB,
    priority          INTEGER NOT NULL DEFAULT 100,
    last_triggered_at TIMESTAMPTZ,
    trigger_count     INTEGER NOT NULL DEFAULT 0,
    created_by        UUID,
    created_at        TIMESTAMPTZ,
    updated_at        TIMESTAMPTZ,
    deleted_at        TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_automation_rules_tenant       ON automation_rules (tenant_id);
CREATE INDEX IF NOT EXISTS idx_automation_rules_trigger      ON automation_rules (trigger);
CREATE INDEX IF NOT EXISTS idx_automation_rules_enabled      ON automation_rules (enabled);
CREATE INDEX IF NOT EXISTS idx_automation_rules_deleted_at   ON automation_rules (deleted_at);

CREATE TABLE IF NOT EXISTS automation_executions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL,
    rule_id      UUID,
    rule_name    VARCHAR(160),
    trigger      VARCHAR(40),
    trigger_ref  VARCHAR(128),
    subject      VARCHAR(255),
    severity     VARCHAR(16),
    status       VARCHAR(16),
    steps        JSONB,
    error        TEXT,
    started_at   TIMESTAMPTZ,
    finished_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_automation_exec_tenant      ON automation_executions (tenant_id);
CREATE INDEX IF NOT EXISTS idx_automation_exec_rule        ON automation_executions (rule_id);
CREATE INDEX IF NOT EXISTS idx_automation_exec_trigger     ON automation_executions (trigger);
CREATE INDEX IF NOT EXISTS idx_automation_exec_trigger_ref ON automation_executions (trigger_ref);
CREATE INDEX IF NOT EXISTS idx_automation_exec_status      ON automation_executions (status);

CREATE TABLE IF NOT EXISTS sla_trackers (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL,
    rule_id           UUID,
    execution_id      UUID,
    subject_type      VARCHAR(16),
    subject_id        VARCHAR(128),
    risk_id           UUID,
    title             VARCHAR(255),
    severity          VARCHAR(16),
    ticket_ref        VARCHAR(128),
    status            VARCHAR(16),
    due_at            TIMESTAMPTZ,
    escalate_at       TIMESTAMPTZ,
    escalate_to_role  VARCHAR(24),
    escalate_channels JSONB,
    escalation_level  INTEGER NOT NULL DEFAULT 0,
    escalated_at      TIMESTAMPTZ,
    owner_id          UUID,
    closed_at         TIMESTAMPTZ,
    created_at        TIMESTAMPTZ,
    updated_at        TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_sla_trackers_tenant      ON sla_trackers (tenant_id);
CREATE INDEX IF NOT EXISTS idx_sla_trackers_status      ON sla_trackers (status);
CREATE INDEX IF NOT EXISTS idx_sla_trackers_due_at      ON sla_trackers (due_at);
CREATE INDEX IF NOT EXISTS idx_sla_trackers_escalate_at ON sla_trackers (escalate_at);
CREATE INDEX IF NOT EXISTS idx_sla_trackers_risk        ON sla_trackers (risk_id);
CREATE INDEX IF NOT EXISTS idx_sla_trackers_subject     ON sla_trackers (subject_id);
CREATE INDEX IF NOT EXISTS idx_sla_trackers_severity    ON sla_trackers (severity);
