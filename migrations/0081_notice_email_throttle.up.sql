-- #997 — at most one notice e-mail per recipient, type and resource within a
-- window. One row per key holds when the last e-mail went out; the backend
-- claims it with INSERT ... ON CONFLICT ... DO UPDATE ... WHERE sent_at is
-- older than the window, so replicas share the throttle.
--
-- AutoMigrate builds the same table from domain.NoticeEmailClaim on a
-- deployment without the SQL layer; this file is then a no-op.
--
-- tenant_id leads the primary key and every query filters on it.

CREATE TABLE IF NOT EXISTS notice_email_throttle (
    tenant_id     uuid        NOT NULL,
    user_id       uuid        NOT NULL,
    type          text        NOT NULL,
    resource_type text        NOT NULL,
    resource_id   uuid        NOT NULL,
    sent_at       timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, user_id, type, resource_type, resource_id)
);
