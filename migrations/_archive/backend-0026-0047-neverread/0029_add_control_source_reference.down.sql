-- Migration: 0029_add_control_source_reference.down.sql
-- Purpose: Rollback migration 0029

ALTER TABLE compliance_controls
    DROP COLUMN IF EXISTS source_reference;
