-- Restores the old column default only. The rows moved from 'starter' to
-- 'free' are not moved back: which of them were 'starter' is not recorded,
-- and turning Free organisations into Pro on a rollback would grant plans
-- nobody bought.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'organizations') THEN
        ALTER TABLE organizations ALTER COLUMN plan SET DEFAULT 'starter';
    END IF;
END $$;
