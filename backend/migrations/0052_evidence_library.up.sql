-- Copyright (c) 2026 OpenDefender Contributors
-- SPDX-License-Identifier: AGPL-3.0-only

-- Evidence library: one proof artifact, N controls.
--
-- control_evidences (one file bolted to one control, no dates) stays in place and
-- is backfilled from, not dropped. Keeping it for a release means a tenant that
-- discovers a bad migration has their original rows; the application stops
-- reading it the moment this lands.

CREATE TABLE IF NOT EXISTS evidences (
    id             uuid PRIMARY KEY,
    tenant_id      uuid NOT NULL,

    title          varchar(255) NOT NULL DEFAULT '',
    type           varchar(20)  NOT NULL DEFAULT 'document',
    description    text,

    file_ref       text NOT NULL DEFAULT '',
    filename       varchar(255) NOT NULL DEFAULT '',
    external_url   text NOT NULL DEFAULT '',

    -- When the proof was taken, not when the row was written.
    collected_at   timestamptz NOT NULL DEFAULT now(),
    valid_until    timestamptz,
    collected_by   uuid,

    review         varchar(16) NOT NULL DEFAULT 'accepted',
    review_note    text,
    reviewed_by    uuid,
    reviewed_at    timestamptz,
    source         varchar(20) NOT NULL DEFAULT 'manual',
    source_detail  varchar(255) NOT NULL DEFAULT '',

    -- Generalised ownership block (migration 0044), embedded as columns.
    owner_id       uuid,
    assignee_id    uuid,
    reviewer_id    uuid,

    reminder_sent_at timestamptz,

    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz
);

CREATE INDEX IF NOT EXISTS idx_evidences_tenant        ON evidences (tenant_id);
CREATE INDEX IF NOT EXISTS idx_evidences_deleted_at    ON evidences (deleted_at);
CREATE INDEX IF NOT EXISTS idx_evidences_type          ON evidences (type);
CREATE INDEX IF NOT EXISTS idx_evidences_review        ON evidences (review);
CREATE INDEX IF NOT EXISTS idx_evidences_source        ON evidences (source);
CREATE INDEX IF NOT EXISTS idx_evidences_collected_at  ON evidences (collected_at);
-- The expiry sweep reads (valid_until, reminder_sent_at) per tenant; a partial
-- index keeps the never-expiring majority out of the worker's way.
CREATE INDEX IF NOT EXISTS idx_evidences_valid_until   ON evidences (valid_until)
    WHERE valid_until IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS evidence_control_links (
    id           uuid PRIMARY KEY,
    tenant_id    uuid NOT NULL,
    evidence_id  uuid NOT NULL,
    control_id   uuid NOT NULL,
    note         text,
    linked_by    uuid,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_evidence_links_tenant   ON evidence_control_links (tenant_id);
CREATE INDEX IF NOT EXISTS idx_evidence_links_evidence ON evidence_control_links (evidence_id);
CREATE INDEX IF NOT EXISTS idx_evidence_links_control  ON evidence_control_links (control_id);

-- Linking the same artifact to the same control twice is not a second proof.
-- The uniqueness is what lets Link() be idempotent instead of accumulating
-- duplicates every time someone re-attaches a file they already attached.
CREATE UNIQUE INDEX IF NOT EXISTS uq_evidence_control_pair
    ON evidence_control_links (evidence_id, control_id);

-- ---------------------------------------------------------------------------
-- Backfill: every existing per-control evidence becomes a library artifact
-- carrying its original id, plus one link to the control it was bolted to.
--
-- Preserving the id matters: the stored file key, any audit-trail entry and any
-- URL a user bookmarked all reference it. A backfill that re-keys is a backfill
-- that breaks download links.
--
-- collected_at falls back to created_at — the honest answer when the old model
-- never recorded when the proof was taken. valid_until stays NULL rather than
-- being invented: guessing an expiry would put dates in front of auditors that
-- nobody ever asserted.
-- ---------------------------------------------------------------------------
INSERT INTO evidences (
    id, tenant_id, title, type, description, file_ref, filename,
    collected_at, collected_by, review, source,
    owner_id, assignee_id, reviewer_id,
    created_at, updated_at, deleted_at
)
SELECT
    ce.id,
    ce.tenant_id,
    COALESCE(NULLIF(ce.filename, ''), 'Preuve'),
    'document',
    ce.description,
    COALESCE(ce.url, ''),
    COALESCE(ce.filename, ''),
    ce.created_at,
    ce.uploaded_by,
    'accepted',
    'manual',
    ce.owner_id, ce.assignee_id, ce.reviewer_id,
    ce.created_at, ce.updated_at, ce.deleted_at
FROM control_evidences ce
WHERE NOT EXISTS (SELECT 1 FROM evidences e WHERE e.id = ce.id);

INSERT INTO evidence_control_links (id, tenant_id, evidence_id, control_id, linked_by, created_at)
SELECT
    gen_random_uuid(), ce.tenant_id, ce.id, ce.control_id, ce.uploaded_by, ce.created_at
FROM control_evidences ce
WHERE ce.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM evidence_control_links l
      WHERE l.evidence_id = ce.id AND l.control_id = ce.control_id
  );
