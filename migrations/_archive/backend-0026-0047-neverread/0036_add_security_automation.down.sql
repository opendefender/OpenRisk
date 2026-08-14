-- Migration: 0036_add_security_automation.down.sql
-- Reverses 0036_add_security_automation.up.sql.

DROP TABLE IF EXISTS automation_channels;
DROP TABLE IF EXISTS sla_trackers;
DROP TABLE IF EXISTS automation_executions;
DROP TABLE IF EXISTS automation_rules;
