-- RULE #4 (Master Prompt V5): the audit trail is append-only at the DATABASE
-- level, not merely by application discipline. A BEFORE UPDATE OR DELETE trigger
-- rejects any mutation of an already-written audit row.
--
-- Escape hatch for legitimate SYSTEM maintenance only: a session that sets the
-- custom GUC openrisk.audit_maintenance = 'on' (SET LOCAL, transaction-scoped)
-- may mutate. Exactly two code paths do this, both auditable:
--   * retention pruning (seals then deletes aged entries) — GormAuditChainRepository.Prune
--   * one-time legacy sequence backfill — database.PrepareForAutoMigrate
-- Ordinary application writes (the audit plugin, handlers) never set it, so for
-- every normal operation UPDATE and DELETE are refused. current_setting(...,true)
-- returns NULL when unset (missing_ok), which is not 'on', so the default is deny.

CREATE OR REPLACE FUNCTION openrisk_audit_append_only() RETURNS trigger AS $$
BEGIN
    IF current_setting('openrisk.audit_maintenance', true) = 'on' THEN
        RETURN COALESCE(NEW, OLD); -- privileged maintenance: allow (NEW on UPDATE, OLD on DELETE)
    END IF;
    RAISE EXCEPTION 'audit trail is append-only: % on % is not permitted', TG_OP, TG_TABLE_NAME
        USING ERRCODE = 'check_violation';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_audit_events_append_only ON audit_events;
CREATE TRIGGER trg_audit_events_append_only
    BEFORE UPDATE OR DELETE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION openrisk_audit_append_only();

DROP TRIGGER IF EXISTS trg_admin_audit_events_append_only ON admin_audit_events;
CREATE TRIGGER trg_admin_audit_events_append_only
    BEFORE UPDATE OR DELETE ON admin_audit_events
    FOR EACH ROW EXECUTE FUNCTION openrisk_audit_append_only();
