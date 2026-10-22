-- #879, D-065 and D-074 — remove the plain SHA-256 of e-mail addresses written
-- before the hash was keyed.
--
-- A plain SHA-256 of an address is reversed by anyone holding the data and a
-- list of candidate addresses. Since #879 the backend writes an HMAC under
-- AUDIT_HASH_KEY instead, but the older values cannot be re-keyed without the
-- addresses themselves, so they are removed. Owner decision, 2026-10-21.
--
-- auth_audit_logs: the row stays (date, IP, user agent, outcome, reason); only
-- the hash in the reason is replaced. New rows carry `email_hmac=`, which this
-- pattern does not touch, so running it again changes nothing.
--
-- password_reset_tokens: a row is only read inside the one-hour rate-limit
-- window and the 30-minute life of its link. Rows past both are deleted here,
-- and the backend keeps deleting them every hour. Rows still inside the window
-- are left to that hourly purge.
--
-- Both tables are created by the backend (GORM), not by this chain, so each
-- statement runs only when its table exists.
--
-- Irreversible: see the down file.

DO $$
BEGIN
    IF to_regclass('auth_audit_logs') IS NOT NULL THEN
        UPDATE auth_audit_logs
           SET failure_reason = regexp_replace(failure_reason, 'email_sha256=[0-9a-f]{64}', 'email_sha256=redacted', 'g')
         WHERE failure_reason ~ 'email_sha256=[0-9a-f]{64}';
    END IF;

    IF to_regclass('password_reset_tokens') IS NOT NULL THEN
        DELETE FROM password_reset_tokens
         WHERE expires_at < now() - interval '1 hour';
    END IF;
END
$$;
