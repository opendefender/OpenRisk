-- #876 — index the lookup login actually runs.
--
-- Since #688, GetByEmail matches LOWER(email) so the casing typed never decides
-- which account is reached. The unique index on users.email is on the raw
-- column and cannot serve that predicate, so every sign-in, sign-up, reset and
-- invitation scanned users.
--
-- Deliberately NOT unique: rows written before #687 may hold case-variant
-- duplicates, and a unique index would fail this migration, and the boot with
-- it, on such a database. docs/MIGRATIONS.md has the query that lists them.
--
-- users is created by AutoMigrate, which runs before this layer; the guard only
-- matters for a run against an empty database.

DO $$
BEGIN
    IF to_regclass('users') IS NULL THEN
        RETURN;
    END IF;
    CREATE INDEX IF NOT EXISTS idx_users_email_lower ON users (LOWER(email));
END
$$;
