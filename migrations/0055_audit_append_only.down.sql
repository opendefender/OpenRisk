DROP TRIGGER IF EXISTS trg_admin_audit_events_append_only ON admin_audit_events;
DROP TRIGGER IF EXISTS trg_audit_events_append_only ON audit_events;
DROP FUNCTION IF EXISTS openrisk_audit_append_only();
