-- #879, D-064 — when a user's trusted-device cookies were last revoked.
--
-- A trusted-device cookie lets the owner's usual browser past a sign-in lock
-- someone else keeps setting. Cookies issued before this moment are refused.
-- It is stamped with the sessions: password reset or change, and "sign out
-- other devices". NULL means never revoked.
--
-- GORM adds the column too (domain.User.DevicesRevokedAt); this file makes it
-- part of the SQL chain. The table may not exist yet on a migrate-only run.

DO $$
BEGIN
    IF to_regclass('users') IS NOT NULL THEN
        ALTER TABLE users ADD COLUMN IF NOT EXISTS devices_revoked_at timestamptz;
    END IF;
END
$$;
