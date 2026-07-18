-- Migration: 0033_vuln_integrations.down.sql

ALTER TABLE vulnerabilities
    DROP COLUMN IF EXISTS risk_id,
    DROP COLUMN IF EXISTS ticket_provider,
    DROP COLUMN IF EXISTS ticket_key,
    DROP COLUMN IF EXISTS ticket_url;

DROP TABLE IF EXISTS vuln_ticketing_configs;
DROP TABLE IF EXISTS vuln_integrations;
