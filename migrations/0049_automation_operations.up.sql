-- 0049 — Automation you can trust: rule lifecycle, richer run history, more channels.
--
-- Adds the columns behind: suspend-with-a-reason, the live health indicator,
-- run history that records what went in / came out / who is answerable, and
-- two extra outbound channels (generic webhook, HTTP SMS gateway).

-- Rule lifecycle + live health -------------------------------------------------
ALTER TABLE automation_rules ADD COLUMN IF NOT EXISTS suspended_at      TIMESTAMPTZ;
ALTER TABLE automation_rules ADD COLUMN IF NOT EXISTS suspended_by      UUID;
ALTER TABLE automation_rules ADD COLUMN IF NOT EXISTS suspended_reason  TEXT;
ALTER TABLE automation_rules ADD COLUMN IF NOT EXISTS last_status       VARCHAR(16);
ALTER TABLE automation_rules ADD COLUMN IF NOT EXISTS last_executed_at  TIMESTAMPTZ;
ALTER TABLE automation_rules ADD COLUMN IF NOT EXISTS last_error        TEXT;
ALTER TABLE automation_rules ADD COLUMN IF NOT EXISTS failure_streak    INTEGER NOT NULL DEFAULT 0;
ALTER TABLE automation_rules ADD COLUMN IF NOT EXISTS template_key      VARCHAR(64);

-- Rules that are already disabled predate the "why" field. Say that plainly
-- rather than inventing a reason nobody gave.
UPDATE automation_rules
   SET suspended_reason = 'disabled before suspension reasons were recorded'
 WHERE enabled = false AND (suspended_reason IS NULL OR suspended_reason = '');

-- Run history ------------------------------------------------------------------
ALTER TABLE automation_executions ADD COLUMN IF NOT EXISTS mode          VARCHAR(16);
ALTER TABLE automation_executions ADD COLUMN IF NOT EXISTS input         JSONB;
ALTER TABLE automation_executions ADD COLUMN IF NOT EXISTS output        JSONB;
ALTER TABLE automation_executions ADD COLUMN IF NOT EXISTS actor_id      UUID;
ALTER TABLE automation_executions ADD COLUMN IF NOT EXISTS replayed_from UUID;
ALTER TABLE automation_executions ADD COLUMN IF NOT EXISTS duration_ms   BIGINT NOT NULL DEFAULT 0;

-- Existing rows were all live runs, and their duration is recoverable.
UPDATE automation_executions SET mode = 'live' WHERE mode IS NULL;
UPDATE automation_executions
   SET duration_ms = GREATEST(0, (EXTRACT(EPOCH FROM (finished_at - started_at)) * 1000)::BIGINT)
 WHERE duration_ms = 0 AND finished_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_automation_exec_mode     ON automation_executions (mode);
CREATE INDEX IF NOT EXISTS idx_automation_exec_actor    ON automation_executions (actor_id);
CREATE INDEX IF NOT EXISTS idx_automation_exec_replayed ON automation_executions (replayed_from);

-- Outbound channels ------------------------------------------------------------
ALTER TABLE automation_channels ADD COLUMN IF NOT EXISTS webhook_enabled  BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE automation_channels ADD COLUMN IF NOT EXISTS webhook_url      VARCHAR(512);
ALTER TABLE automation_channels ADD COLUMN IF NOT EXISTS webhook_secret   VARCHAR(255);
ALTER TABLE automation_channels ADD COLUMN IF NOT EXISTS sms_enabled      BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE automation_channels ADD COLUMN IF NOT EXISTS sms_gateway_url  VARCHAR(512);
ALTER TABLE automation_channels ADD COLUMN IF NOT EXISTS sms_api_key      VARCHAR(255);
ALTER TABLE automation_channels ADD COLUMN IF NOT EXISTS sms_sender       VARCHAR(32);
ALTER TABLE automation_channels ADD COLUMN IF NOT EXISTS sms_recipients   VARCHAR(512);
ALTER TABLE automation_channels ADD COLUMN IF NOT EXISTS sms_to_field     VARCHAR(32);
ALTER TABLE automation_channels ADD COLUMN IF NOT EXISTS sms_text_field   VARCHAR(32);
ALTER TABLE automation_channels ADD COLUMN IF NOT EXISTS sms_sender_field VARCHAR(32);
