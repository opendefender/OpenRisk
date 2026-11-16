-- #1428 (#555 slice 2) — control exceptions: a time-boxed, owned, justified
-- waiver of one control, with a revocation of its own.
--
-- DDL only. There is no status column and no expiry job: an exception reads as
-- open, expired or revoked from its own columns at the caller's instant
-- (domain.ControlException.StatusAt). The control is the canonical controls.id
-- of migration 0086, of the same tenant.
--
-- Idempotent: AutoMigrate builds the same table from
-- controlexception.ExceptionRow, with the same index name and the same CHECK
-- constraints, and on a database where it ran first every statement below is a
-- no-op.
--
-- tenant_id NOT NULL, and the one index leads with it (internal/security/
-- isolation/tables.go lists the table and a Postgres test asserts it). No
-- foreign key, like 0086: the codebase builds schemas with
-- DisableForeignKeyConstraintWhenMigrating, and the tenant gate on the control
-- is the repository's (ABSOLUTE RULE 2), written once.

CREATE TABLE IF NOT EXISTS control_exceptions (
    id            uuid PRIMARY KEY,
    tenant_id     uuid NOT NULL,
    control_id    uuid NOT NULL,
    owner_id      uuid NOT NULL,
    reason        text NOT NULL,
    approved_by   uuid NOT NULL,
    starts_at     timestamptz NOT NULL,
    expires_at    timestamptz NOT NULL,
    revoked_at    timestamptz,
    revoked_by    uuid,
    revoke_reason text NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_control_exceptions_window CHECK (expires_at > starts_at),
    CONSTRAINT chk_control_exceptions_revocation CHECK ((revoked_at IS NULL) = (revoked_by IS NULL)),
    CONSTRAINT chk_control_exceptions_reason CHECK (length(trim(reason)) > 0)
);
CREATE INDEX IF NOT EXISTS idx_control_exceptions_control ON control_exceptions (tenant_id, control_id, expires_at);
