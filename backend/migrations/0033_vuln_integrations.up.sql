-- Migration: 0033_vuln_integrations.up.sql
-- Purpose: back the vulnerability-management "connectors config" — per-source
-- scanner integrations (encrypted API credentials, live-pull schedule, inbound
-- webhook token, automation toggles) plus the tenant ITSM/ticketing config used
-- for auto-ticketing. Also adds the cross-module linkage columns to the
-- vulnerabilities register (auto-created risk id + opened ticket ref).
--
-- GORM's AutoMigrate already creates/updates these (the models are in
-- AutoMigrate); this file exists for migrations-only deploys and documentation.

CREATE TABLE IF NOT EXISTS vuln_integrations (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id             UUID NOT NULL,
    source                VARCHAR(24) NOT NULL,
    name                  VARCHAR(128),
    enabled               BOOLEAN DEFAULT TRUE,
    base_url              VARCHAR(512),
    encrypted_credentials TEXT,
    live_pull_enabled     BOOLEAN DEFAULT FALSE,
    schedule_minutes      INTEGER DEFAULT 0,
    last_pull_at          TIMESTAMPTZ,
    last_pull_status      VARCHAR(16) DEFAULT 'never',
    last_pull_error       TEXT,
    last_pull_count       INTEGER DEFAULT 0,
    webhook_enabled       BOOLEAN DEFAULT FALSE,
    webhook_token         VARCHAR(80),
    auto_create_risk      BOOLEAN DEFAULT FALSE,
    auto_create_ticket    BOOLEAN DEFAULT FALSE,
    created_at            TIMESTAMPTZ,
    updated_at            TIMESTAMPTZ,
    deleted_at            TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_vuln_integration_tenant_source
    ON vuln_integrations (tenant_id, source);
CREATE UNIQUE INDEX IF NOT EXISTS idx_vuln_integrations_webhook_token
    ON vuln_integrations (webhook_token);
CREATE INDEX IF NOT EXISTS idx_vuln_integrations_deleted_at ON vuln_integrations (deleted_at);

CREATE TABLE IF NOT EXISTS vuln_ticketing_configs (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id             UUID NOT NULL,
    provider              VARCHAR(16),
    enabled               BOOLEAN DEFAULT FALSE,
    base_url              VARCHAR(512),
    project_or_table      VARCHAR(128),
    default_issue_type    VARCHAR(64) DEFAULT 'Bug',
    encrypted_credentials TEXT,
    created_at            TIMESTAMPTZ,
    updated_at            TIMESTAMPTZ,
    deleted_at            TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_vuln_ticketing_tenant
    ON vuln_ticketing_configs (tenant_id);
CREATE INDEX IF NOT EXISTS idx_vuln_ticketing_deleted_at ON vuln_ticketing_configs (deleted_at);

ALTER TABLE vulnerabilities
    ADD COLUMN IF NOT EXISTS risk_id         UUID,
    ADD COLUMN IF NOT EXISTS ticket_provider VARCHAR(16),
    ADD COLUMN IF NOT EXISTS ticket_key      VARCHAR(64),
    ADD COLUMN IF NOT EXISTS ticket_url      VARCHAR(512);
CREATE INDEX IF NOT EXISTS idx_vulnerabilities_risk_id ON vulnerabilities (risk_id);
