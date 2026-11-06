DO $$
BEGIN
    IF to_regclass('scan_jobs') IS NULL THEN
        RETURN;
    END IF;

    ALTER TABLE scan_jobs DROP COLUMN IF EXISTS scan_type;
END $$;
