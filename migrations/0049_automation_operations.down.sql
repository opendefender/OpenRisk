-- Revert 0049.

DROP INDEX IF EXISTS idx_automation_exec_mode;
DROP INDEX IF EXISTS idx_automation_exec_actor;
DROP INDEX IF EXISTS idx_automation_exec_replayed;

ALTER TABLE automation_rules DROP COLUMN IF EXISTS suspended_at;
ALTER TABLE automation_rules DROP COLUMN IF EXISTS suspended_by;
ALTER TABLE automation_rules DROP COLUMN IF EXISTS suspended_reason;
ALTER TABLE automation_rules DROP COLUMN IF EXISTS last_status;
ALTER TABLE automation_rules DROP COLUMN IF EXISTS last_executed_at;
ALTER TABLE automation_rules DROP COLUMN IF EXISTS last_error;
ALTER TABLE automation_rules DROP COLUMN IF EXISTS failure_streak;
ALTER TABLE automation_rules DROP COLUMN IF EXISTS template_key;

ALTER TABLE automation_executions DROP COLUMN IF EXISTS mode;
ALTER TABLE automation_executions DROP COLUMN IF EXISTS input;
ALTER TABLE automation_executions DROP COLUMN IF EXISTS output;
ALTER TABLE automation_executions DROP COLUMN IF EXISTS actor_id;
ALTER TABLE automation_executions DROP COLUMN IF EXISTS replayed_from;
ALTER TABLE automation_executions DROP COLUMN IF EXISTS duration_ms;

ALTER TABLE automation_channels DROP COLUMN IF EXISTS webhook_enabled;
ALTER TABLE automation_channels DROP COLUMN IF EXISTS webhook_url;
ALTER TABLE automation_channels DROP COLUMN IF EXISTS webhook_secret;
ALTER TABLE automation_channels DROP COLUMN IF EXISTS sms_enabled;
ALTER TABLE automation_channels DROP COLUMN IF EXISTS sms_gateway_url;
ALTER TABLE automation_channels DROP COLUMN IF EXISTS sms_api_key;
ALTER TABLE automation_channels DROP COLUMN IF EXISTS sms_sender;
ALTER TABLE automation_channels DROP COLUMN IF EXISTS sms_recipients;
ALTER TABLE automation_channels DROP COLUMN IF EXISTS sms_to_field;
ALTER TABLE automation_channels DROP COLUMN IF EXISTS sms_text_field;
ALTER TABLE automation_channels DROP COLUMN IF EXISTS sms_sender_field;
