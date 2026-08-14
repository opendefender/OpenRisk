-- Copyright (c) 2026 OpenDefender Contributors
-- SPDX-License-Identifier: AGPL-3.0-only

-- Rolling back loses information, and says so: coverage 'partial' could have
-- come from either 'partial' or 'related', and there is no way to tell them
-- apart afterwards. It maps back to 'partial', which is the conservative of the
-- two. Curated crosswalks are left in place as manual ones rather than deleted —
-- a tenant may have accepted them, and silently removing links they were relying
-- on would be worse than leaving rows whose provenance is no longer recorded.

ALTER TABLE control_crosswalks ADD COLUMN IF NOT EXISTS relation varchar(16) NOT NULL DEFAULT 'equivalent';
UPDATE control_crosswalks SET relation = CASE coverage WHEN 'full' THEN 'equivalent' ELSE 'partial' END;
ALTER TABLE control_crosswalks DROP COLUMN IF EXISTS coverage;
ALTER TABLE control_crosswalks DROP COLUMN IF EXISTS origin;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name = 'control_crosswalks' AND column_name = 'rationale') THEN
        ALTER TABLE control_crosswalks RENAME COLUMN rationale TO note;
    END IF;
END $$;

ALTER TABLE IF EXISTS control_crosswalks RENAME TO control_mappings;

DROP INDEX IF EXISTS idx_compliance_frameworks_catalog_key;
ALTER TABLE compliance_frameworks DROP COLUMN IF EXISTS catalog_key;
