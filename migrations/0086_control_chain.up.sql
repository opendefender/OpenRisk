-- #553 (D-108, ADR 0008 amendment A10) — the control chain: Requirement ->
-- Control -> Implementation -> Test -> Result -> Evidence, plus the one edge
-- table that stores the relationships between them.
--
-- DDL only, on purpose. The Control that answers each compliance_controls row
-- has the id canonical.ControlIDFor(row.id), a UUIDv5 (SHA-1). PostgreSQL has no
-- SHA-1 UUID without the uuid-ossp extension, which no migration creates and
-- managed or hardened servers do not always grant, so the backfill is Go, not
-- SQL: controlchain.Reconcile runs on every boot, after AutoMigrate, with or
-- without DATABASE_URL (a compose boot never runs this SQL layer). It is
-- insert-only and idempotent, so a late replay changes nothing that exists.
-- This file therefore moves no row and touches none of compliance_controls,
-- evidence_control_links or risk_control_mappings.
--
-- Idempotent: AutoMigrate builds the same tables from the controlchain row
-- types, with the same index names and the same CHECK constraints, and on a
-- database where it ran first every statement below is a no-op.
--
-- Every table carries tenant_id NOT NULL and an index that leads with it
-- (internal/security/isolation/tables.go lists them and a Postgres test
-- asserts it). No foreign key: the codebase builds schemas with
-- DisableForeignKeyConstraintWhenMigrating, and the tenant gate on every end
-- of an edge is the application's (ABSOLUTE RULE 2), written once.

CREATE TABLE IF NOT EXISTS controls (
    id                           uuid PRIMARY KEY,
    tenant_id                    uuid NOT NULL,
    source                       varchar(64) NOT NULL,
    source_id                    varchar(255) NOT NULL,
    source_version               varchar(64),
    confidence                   numeric(3,2) NOT NULL,
    provenance                   jsonb NOT NULL,
    observed_at                  timestamptz,
    created_at                   timestamptz NOT NULL DEFAULT now(),
    updated_at                   timestamptz NOT NULL DEFAULT now(),
    deleted_at                   timestamptz,
    name                         varchar(255) NOT NULL,
    description                  text,
    objective                    text,
    owner_id                     uuid,
    assignee_id                  uuid,
    reviewer_id                  uuid,
    applicability                varchar(16) NOT NULL,
    applicability_justification  text,
    applicability_review_pending boolean NOT NULL DEFAULT false,
    declared_status              varchar(20) NOT NULL,
    tags                         jsonb NOT NULL DEFAULT '[]',
    CONSTRAINT chk_controls_confidence CHECK (confidence >= 0 AND confidence <= 1),
    CONSTRAINT chk_controls_applicability CHECK (applicability IN ('applicable', 'not_applicable')),
    CONSTRAINT chk_controls_declared_status CHECK (declared_status IN ('not_implemented', 'in_progress', 'implemented')),
    CONSTRAINT chk_controls_not_applicable CHECK (
        applicability <> 'not_applicable'
        OR applicability_review_pending
        OR length(trim(COALESCE(applicability_justification, ''))) > 0)
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_controls_identity ON controls (tenant_id, source, source_id);
CREATE INDEX IF NOT EXISTS idx_controls_tenant_deleted ON controls (tenant_id, deleted_at);

CREATE TABLE IF NOT EXISTS control_implementations (
    id             uuid PRIMARY KEY,
    tenant_id      uuid NOT NULL,
    source         varchar(64) NOT NULL,
    source_id      varchar(255) NOT NULL,
    source_version varchar(64),
    confidence     numeric(3,2) NOT NULL,
    provenance     jsonb NOT NULL,
    observed_at    timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz,
    control_id     uuid NOT NULL,
    description    text NOT NULL,
    status         varchar(16) NOT NULL,
    in_place_since date,
    owner_id       uuid,
    assignee_id    uuid,
    reviewer_id    uuid,
    CONSTRAINT chk_control_implementations_confidence CHECK (confidence >= 0 AND confidence <= 1),
    CONSTRAINT chk_control_implementations_status CHECK (status IN ('planned', 'in_progress', 'in_place', 'retired'))
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_control_implementations_identity ON control_implementations (tenant_id, source, source_id);
CREATE INDEX IF NOT EXISTS idx_control_implementations_control ON control_implementations (tenant_id, control_id);

CREATE TABLE IF NOT EXISTS control_tests (
    id                uuid PRIMARY KEY,
    tenant_id         uuid NOT NULL,
    source            varchar(64) NOT NULL,
    source_id         varchar(255) NOT NULL,
    source_version    varchar(64),
    confidence        numeric(3,2) NOT NULL,
    provenance        jsonb NOT NULL,
    observed_at       timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    deleted_at        timestamptz,
    implementation_id uuid NOT NULL,
    name              varchar(255) NOT NULL,
    checks            text NOT NULL,
    method            varchar(16) NOT NULL,
    procedure         text,
    collector         varchar(255),
    cadence           varchar(32),
    pass_criteria     text NOT NULL,
    CONSTRAINT chk_control_tests_confidence CHECK (confidence >= 0 AND confidence <= 1),
    CONSTRAINT chk_control_tests_method CHECK (method IN ('manual', 'automated')),
    CONSTRAINT chk_control_tests_declares_itself CHECK (
        (method <> 'manual' OR length(trim(COALESCE(procedure, ''))) > 0)
        AND (method <> 'automated' OR length(trim(COALESCE(collector, ''))) > 0))
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_control_tests_identity ON control_tests (tenant_id, source, source_id);
CREATE INDEX IF NOT EXISTS idx_control_tests_implementation ON control_tests (tenant_id, implementation_id);

-- Append-only: no updated_at, no deleted_at. tenant_id is copied from the test
-- row when a result is recorded (A10.5).
CREATE TABLE IF NOT EXISTS control_test_results (
    id               uuid PRIMARY KEY,
    tenant_id        uuid NOT NULL,
    test_id          uuid NOT NULL,
    outcome          varchar(16) NOT NULL,
    observed_at      timestamptz NOT NULL,
    recorded_at      timestamptz NOT NULL,
    source           varchar(64) NOT NULL,
    source_id        varchar(255) NOT NULL,
    confidence       numeric(3,2) NOT NULL,
    executed_by_kind varchar(16) NOT NULL,
    executed_by_id   varchar(255) NOT NULL,
    note             text,
    CONSTRAINT chk_control_test_results_confidence CHECK (confidence >= 0 AND confidence <= 1),
    CONSTRAINT chk_control_test_results_outcome CHECK (outcome IN ('pass', 'fail', 'inconclusive')),
    CONSTRAINT chk_control_test_results_executed_by CHECK (executed_by_kind IN ('user', 'system', 'vendor_contact'))
);
CREATE INDEX IF NOT EXISTS idx_control_test_results_test ON control_test_results (tenant_id, test_id, recorded_at);
CREATE UNIQUE INDEX IF NOT EXISTS ux_control_test_results_identity ON control_test_results (tenant_id, test_id, source, source_id);

-- Findings of kind control_gap only (A10.6): vulnerabilities stays the scanner
-- register, with its own dedup, SLA and risk-proposal paths.
CREATE TABLE IF NOT EXISTS control_gap_findings (
    id                 uuid PRIMARY KEY,
    tenant_id          uuid NOT NULL,
    source             varchar(64) NOT NULL,
    source_id          varchar(255) NOT NULL,
    source_version     varchar(64),
    confidence         numeric(3,2) NOT NULL,
    provenance         jsonb NOT NULL,
    observed_at        timestamptz NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    kind               varchar(16) NOT NULL DEFAULT 'control_gap',
    test_id            uuid NOT NULL,
    title              varchar(512) NOT NULL,
    description        text,
    severity           varchar(16) NOT NULL,
    status             varchar(16) NOT NULL,
    first_observed_at  timestamptz NOT NULL,
    sla_due_at         timestamptz,
    remediated_at      timestamptz,
    remediation_hint   text,
    CONSTRAINT chk_control_gap_findings_confidence CHECK (confidence >= 0 AND confidence <= 1),
    CONSTRAINT chk_control_gap_findings_kind CHECK (kind = 'control_gap'),
    CONSTRAINT chk_control_gap_findings_severity CHECK (severity IN ('critical', 'high', 'medium', 'low', 'info')),
    CONSTRAINT chk_control_gap_findings_status CHECK (status IN ('open', 'triaged', 'in_remediation', 'remediated', 'accepted', 'false_positive'))
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_control_gap_findings_identity ON control_gap_findings (tenant_id, source, source_id);
CREATE INDEX IF NOT EXISTS idx_control_gap_findings_test ON control_gap_findings (tenant_id, test_id);
CREATE INDEX IF NOT EXISTS idx_control_gap_findings_status ON control_gap_findings (tenant_id, status);

-- One table for every stored verb (D-108 Q1). implemented_by and tested_by are
-- the control_id and implementation_id columns above, never rows (A10.4);
-- maps_to stays in control_crosswalks. qualifier is the nil uuid when the edge
-- has none, so the unique key is total, as canonical.EdgeKey reads it (A10.3).
CREATE TABLE IF NOT EXISTS canonical_edges (
    id         uuid PRIMARY KEY,
    tenant_id  uuid NOT NULL,
    from_type  varchar(32) NOT NULL,
    from_id    uuid NOT NULL,
    verb       varchar(24) NOT NULL,
    to_type    varchar(32) NOT NULL,
    to_id      uuid NOT NULL,
    qualifier  uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000',
    attributes jsonb NOT NULL DEFAULT '{}',
    source     varchar(64) NOT NULL,
    source_id  varchar(255) NOT NULL,
    confidence numeric(3,2) NOT NULL,
    provenance jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_canonical_edges_confidence CHECK (confidence >= 0 AND confidence <= 1),
    CONSTRAINT chk_canonical_edges_verb CHECK (verb IN ('addressed_by', 'applies_to', 'evidenced_by', 'mitigated_by', 'raised_by', 'indicates'))
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_canonical_edges_key ON canonical_edges (tenant_id, from_type, from_id, verb, to_type, to_id, qualifier);
CREATE INDEX IF NOT EXISTS idx_canonical_edges_to ON canonical_edges (tenant_id, to_type, to_id, verb);
