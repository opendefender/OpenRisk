-- #997 — notice e-mails go through an outbox, grouped per person and item.
--
-- notice_email_outbox: one row per e-mail to send, a single notice or a digest
-- of the notices gathered during a window. A worker on every replica claims
-- due rows with FOR UPDATE SKIP LOCKED, sends them, and retries what fails.
-- No address is stored: the worker resolves the recipient at send time.
--
-- notice_email_throttle: when the last e-mail about an item went out to a
-- person. Its row lock serializes "send now or add to the digest" per key.
--
-- notice_email_policies: each organization's grouping window, in minutes.
--
-- AutoMigrate builds the same tables from domain.NoticeEmailOutbox,
-- NoticeEmailThrottle and NoticeEmailPolicy on a deployment without the SQL
-- layer; this file is then a no-op. tenant_id is on every table and in every
-- per-row query.

CREATE TABLE IF NOT EXISTS notice_email_throttle (
    tenant_id     uuid        NOT NULL,
    user_id       uuid        NOT NULL,
    resource_type text        NOT NULL,
    resource_id   uuid        NOT NULL,
    sent_at       timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, user_id, resource_type, resource_id)
);

CREATE TABLE IF NOT EXISTS notice_email_outbox (
    id             uuid        PRIMARY KEY,
    tenant_id      uuid        NOT NULL,
    user_id        uuid        NOT NULL,
    resource_type  text        NOT NULL DEFAULT '',
    resource_id    uuid,
    resource_title text        NOT NULL DEFAULT '',
    kind           text        NOT NULL,
    grouped        boolean     NOT NULL,
    locale         text        NOT NULL,
    events         jsonb       NOT NULL,
    omitted        bigint      NOT NULL DEFAULT 0,
    status         text        NOT NULL,
    due_at         timestamptz NOT NULL,
    attempts       bigint      NOT NULL DEFAULT 0,
    locked_until   timestamptz,
    last_error     text        NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL,
    sent_at        timestamptz
);

CREATE INDEX IF NOT EXISTS idx_notice_email_outbox_key
    ON notice_email_outbox (tenant_id, user_id, resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_notice_email_outbox_due
    ON notice_email_outbox (status, due_at);

CREATE TABLE IF NOT EXISTS notice_email_policies (
    id             uuid        PRIMARY KEY,
    tenant_id      uuid        NOT NULL,
    window_minutes bigint      NOT NULL,
    updated_by_id  uuid,
    created_at     timestamptz,
    updated_at     timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_notice_email_policies_tenant_id
    ON notice_email_policies (tenant_id);
