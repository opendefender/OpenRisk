-- #879 — undo 0080. Without the column every trusted-device cookie is refused
-- by a backend that expects it, so roll the backend back first.
DO $$
BEGIN
    IF to_regclass('users') IS NOT NULL THEN
        ALTER TABLE users DROP COLUMN IF EXISTS devices_revoked_at;
    END IF;
END
$$;
