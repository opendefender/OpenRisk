-- Migration: 0038_add_member_business_role.down.sql
-- Reverse 0038: drop the membership business-role column.

DROP INDEX IF EXISTS idx_organization_members_business_role;

ALTER TABLE organization_members
    DROP COLUMN IF EXISTS business_role;
