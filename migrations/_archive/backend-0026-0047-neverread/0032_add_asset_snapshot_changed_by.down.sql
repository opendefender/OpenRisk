-- Migration: 0032_add_asset_snapshot_changed_by.down.sql
-- Reverse 0032: drop the asset history "who" column.

DROP INDEX IF EXISTS idx_asset_snapshots_changed_by;

ALTER TABLE asset_snapshots
    DROP COLUMN IF EXISTS changed_by;
