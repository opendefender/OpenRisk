-- Copyright (c) 2026 OpenDefender Contributors
-- SPDX-License-Identifier: AGPL-3.0-only

-- The reporting engine: asynchronous generation, three formats, versioned
-- templates, an integrity hash and an editorial lifecycle.
--
-- report_jobs (the earlier synchronous compliance-only job) is left in place and
-- not migrated from. Its rows carried no format, no language, no hash and no
-- lifecycle, so a backfill would have to invent all four; a report is a record,
-- and a record with invented provenance is worse than an absent one. The old
-- table keeps its rows and its download route until the release that removes it.

CREATE TABLE IF NOT EXISTS reports (
    id               uuid PRIMARY KEY,
    tenant_id        uuid NOT NULL,

    type             varchar(40) NOT NULL,
    format           varchar(8)  NOT NULL DEFAULT 'pdf',
    locale           varchar(8)  NOT NULL DEFAULT 'fr',

    -- The layout AND its version, stored rather than resolved: a document
    -- approved in March has to keep saying which layout produced it.
    template_key     varchar(64) NOT NULL DEFAULT '',
    template_version varchar(16) NOT NULL DEFAULT '',

    params           jsonb,
    title            varchar(255) NOT NULL DEFAULT '',

    -- Generation state, distinct from the editorial state below.
    run_state        varchar(16) NOT NULL DEFAULT 'queued',
    progress         integer NOT NULL DEFAULT 0,
    step             varchar(120) NOT NULL DEFAULT '',
    error            text,

    lifecycle        varchar(16) NOT NULL DEFAULT 'draft',

    filename         varchar(255) NOT NULL DEFAULT '',
    content_type     varchar(128) NOT NULL DEFAULT '',
    artifact         bytea,
    size_bytes       integer NOT NULL DEFAULT 0,
    -- SHA-256 of the exact bytes served, printed on the document itself.
    content_hash     varchar(64) NOT NULL DEFAULT '',

    version          integer NOT NULL DEFAULT 1,
    supersedes       uuid,

    requested_by     uuid,
    approved_by      uuid,
    approved_at      timestamptz,
    published_at     timestamptz,

    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    completed_at     timestamptz
);

CREATE INDEX IF NOT EXISTS idx_reports_tenant       ON reports (tenant_id);
CREATE INDEX IF NOT EXISTS idx_reports_type         ON reports (type);
CREATE INDEX IF NOT EXISTS idx_reports_lifecycle    ON reports (lifecycle);
CREATE INDEX IF NOT EXISTS idx_reports_content_hash ON reports (content_hash);
CREATE INDEX IF NOT EXISTS idx_reports_supersedes   ON reports (supersedes);
CREATE INDEX IF NOT EXISTS idx_reports_requested_by ON reports (requested_by);
-- The recent-reports list is (tenant, created_at DESC); the worker's claim is
-- (run_state, created_at ASC). Both are hot enough to deserve their own index.
CREATE INDEX IF NOT EXISTS idx_reports_tenant_created ON reports (tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_reports_queued ON reports (run_state, created_at)
    WHERE run_state = 'queued';

CREATE TABLE IF NOT EXISTS report_comments (
    id          uuid PRIMARY KEY,
    tenant_id   uuid NOT NULL,
    report_id   uuid NOT NULL,
    author_id   uuid,
    body        text NOT NULL,
    -- The lifecycle move this comment accompanied, empty for a plain remark.
    -- It is what turns a comment list into an audit trail: who approved this,
    -- and what did they say when they did.
    transition  varchar(16) NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_report_comments_tenant ON report_comments (tenant_id);
CREATE INDEX IF NOT EXISTS idx_report_comments_report ON report_comments (report_id);
