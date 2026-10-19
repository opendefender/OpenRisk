-- #1001 — who lost their MFA backup codes to the universal-code purge, and
-- whether they have been told.
--
-- The purge (backend/internal/migrations/universal_backup_codes.go) runs just
-- before 0056 and can therefore run before this file: it creates the table
-- itself with this same shape, and this file is then a no-op. On a fresh
-- install, nothing is purged and the table starts empty.
--
-- tenant_id scopes every row. The backend reads pending rows at boot, mails
-- each person once in their language, and stamps notified_at before sending,
-- so a restart never mails anyone twice.

CREATE TABLE IF NOT EXISTS mfa_backup_code_purges (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid        NOT NULL,
    user_id     uuid        NOT NULL,
    purged_at   timestamptz NOT NULL DEFAULT now(),
    notified_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_mfa_backup_code_purges_pending
    ON mfa_backup_code_purges (purged_at)
 WHERE notified_at IS NULL;
