-- Migration: add deleted_at column to mitigation_subactions for GORM soft-delete

ALTER TABLE mitigation_subactions
  ADD COLUMN IF NOT EXISTS deleted_at timestamptz NULL;

CREATE INDEX IF NOT EXISTS idx_mitigation_subactions_deleted_at ON mitigation_subactions (deleted_at);
