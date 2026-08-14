-- Migration: 0032_add_asset_snapshot_changed_by.up.sql
-- Purpose: give the asset history ("historique des modifications") its missing
-- "who" — the user who performed the update/delete that superseded a snapshot's
-- state. The history already recorded the *quoi* (prior state) and *quand*
-- (created_at); this adds the *qui* mandated by the centralized-asset-management
-- spec ("qui a modifié quoi, et quand").
--
-- GORM's AutoMigrate already adds this column (AssetSnapshot is in AutoMigrate),
-- so this exists mainly to (a) be self-sufficient for a migrations-only deploy
-- and (b) document the schema change. The column is nullable: pre-existing
-- snapshot rows have no known actor and stay NULL (rendered as "Système" in the UI).

ALTER TABLE asset_snapshots
    ADD COLUMN IF NOT EXISTS changed_by UUID;

CREATE INDEX IF NOT EXISTS idx_asset_snapshots_changed_by ON asset_snapshots (changed_by);
