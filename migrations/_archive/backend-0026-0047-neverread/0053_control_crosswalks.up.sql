-- Copyright (c) 2026 OpenDefender Contributors
-- SPDX-License-Identifier: AGPL-3.0-only

-- Crosswalks between frameworks: what a tenant already holds, and how much of a
-- newly imported framework it answers.
--
-- This RENAMES control_mappings rather than creating a second table beside it.
-- Two tables for "a link between two controls" would be two truths, and the
-- inherited-coverage number would depend on which one a given screen wrote to.

ALTER TABLE IF EXISTS control_mappings RENAME TO control_crosswalks;

-- coverage replaces relation. The old vocabulary had three values; "related"
-- was unusable, because thematic relatedness does not tell anyone whether their
-- existing proof can be reused. equivalent -> full, and both partial and related
-- -> partial: demoting "related" is the safe direction, since over-claiming
-- coverage is what tells someone they can stop working.
ALTER TABLE control_crosswalks ADD COLUMN IF NOT EXISTS coverage varchar(16) NOT NULL DEFAULT 'full';

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name = 'control_crosswalks' AND column_name = 'relation') THEN
        UPDATE control_crosswalks
           SET coverage = CASE relation
                            WHEN 'equivalent' THEN 'full'
                            ELSE 'partial'
                          END;
        ALTER TABLE control_crosswalks DROP COLUMN relation;
    END IF;
END $$;

-- note -> rationale. Renamed rather than added because it is the same field
-- doing the same job; the name change is the point. A crosswalk feeds a number
-- an auditor will question, so the reasoning is required rather than a remark.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name = 'control_crosswalks' AND column_name = 'note')
       AND NOT EXISTS (SELECT 1 FROM information_schema.columns
                       WHERE table_name = 'control_crosswalks' AND column_name = 'rationale') THEN
        ALTER TABLE control_crosswalks RENAME COLUMN note TO rationale;
    END IF;
END $$;

ALTER TABLE control_crosswalks ADD COLUMN IF NOT EXISTS rationale text;

-- origin distinguishes what the PRODUCT asserted from what the TENANT asserted.
-- Rows that predate this column were all hand-made, so 'manual' is not a default
-- so much as a fact about them.
ALTER TABLE control_crosswalks ADD COLUMN IF NOT EXISTS origin varchar(16) NOT NULL DEFAULT 'manual';
CREATE INDEX IF NOT EXISTS idx_control_crosswalks_origin ON control_crosswalks (origin);

-- Which catalog a framework was imported from. Curated crosswalks are defined
-- between catalogs, so without this the only way to know that a framework called
-- "SOC 2 (v2)" holds AICPA criteria would be to guess from its title — and
-- inherited coverage would stop working the moment somebody renamed one.
ALTER TABLE compliance_frameworks ADD COLUMN IF NOT EXISTS catalog_key varchar(64) NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_compliance_frameworks_catalog_key ON compliance_frameworks (catalog_key);
