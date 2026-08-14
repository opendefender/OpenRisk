-- Migration: 0037_add_governance.down.sql
-- Reverses 0037_add_governance.up.sql.

DROP TABLE IF EXISTS approval_requests;
DROP TABLE IF EXISTS approval_workflows;
DROP TABLE IF EXISTS delegations;
DROP TABLE IF EXISTS audit_events;
