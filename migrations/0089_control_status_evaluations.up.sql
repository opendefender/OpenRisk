-- #555 (ADR 0012, D-119) — control_status_evaluations: the last evaluation of each
-- control, written only by the control status engine.
--
-- DDL only, expand only: one new table, nothing altered, nothing backfilled. The
-- worker fills it cold (a live control with no row is claimed and computed), so a
-- release that carries this migration loses nothing and needs no data step.
--
-- This table is engine output, never an input. No route writes it, no DTO carries
-- it, and no user-facing read trusts it: GET /compliance/internal-controls/:id/status
-- computes live. #1465 decides whether the score readers join it.
--
-- Numbered 0089, the next free number. #1456 (evidence integrity verdict) takes the next one.
--
-- Idempotent: AutoMigrate builds the same table from controlstatus.EvaluationRow,
-- with the same index names and the same CHECK constraints, and on a database
-- where it ran first every statement below is a no-op.
--
-- tenant_id NOT NULL, and the primary key leads with it (internal/security/
-- isolation/tables.go lists the table and a Postgres test asserts it). No foreign
-- key, like 0086 and 0087: the codebase builds schemas with
-- DisableForeignKeyConstraintWhenMigrating, and the tenant gate on the control is
-- the engine's (ABSOLUTE RULE 2), written once.
--
-- evaluated_at is the instant of the computation (the epoch on a row the claim
-- created and nothing has computed yet). next_change_at is the earliest instant
-- the clock alone can change the result, or the claim's lease while a computation
-- is in flight. stale is set by every write that can change an input, inside that
-- write's own transaction, and cleared by the claim.

CREATE TABLE IF NOT EXISTS control_status_evaluations (
    tenant_id         uuid NOT NULL,
    control_id        uuid NOT NULL,
    status            varchar(24) NOT NULL,
    underlying_status varchar(24) NOT NULL,
    fresh_signals     integer NOT NULL DEFAULT 0,
    counted_signals   integer NOT NULL DEFAULT 0,
    freshness_percent numeric(5,2) NOT NULL DEFAULT 0,
    evaluated_at      timestamptz NOT NULL,
    next_change_at    timestamptz,
    stale             boolean NOT NULL DEFAULT false,
    PRIMARY KEY (tenant_id, control_id),
    CONSTRAINT chk_control_status_evaluations_status CHECK (status IN ('pass', 'fail', 'insufficient_evidence', 'exception_active', 'not_applicable')),
    CONSTRAINT chk_control_status_evaluations_underlying CHECK (underlying_status IN ('pass', 'fail', 'insufficient_evidence')),
    CONSTRAINT chk_control_status_evaluations_counts CHECK (fresh_signals >= 0 AND counted_signals >= fresh_signals),
    CONSTRAINT chk_control_status_evaluations_percent CHECK (freshness_percent >= 0 AND freshness_percent <= 100)
);
-- The worker's claim reads these two sets: rows marked stale, and rows whose
-- next_change_at has passed. Both are small partial indexes.
CREATE INDEX IF NOT EXISTS idx_control_status_evaluations_stale ON control_status_evaluations (evaluated_at) WHERE stale;
CREATE INDEX IF NOT EXISTS idx_control_status_evaluations_due ON control_status_evaluations (next_change_at) WHERE next_change_at IS NOT NULL;
