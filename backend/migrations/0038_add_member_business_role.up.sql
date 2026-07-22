-- Migration: 0038_add_member_business_role.up.sql
-- Purpose: give an organization membership its GRC job-role preset. The runtime
-- authorization path expands OrganizationMember → JWT permissions at login;
-- root/admin get the "*" wildcard, but a plain 'user' member previously had no
-- coherent way to receive the permission strings the routes actually check
-- (risks:create, compliance:frameworks:read, vulnerabilities:read, scanner:*,
-- automation:*, reports:board:*, incidents:*, …) and was effectively locked out
-- of most of the app. business_role names one of the least-privilege presets in
-- internal/domain/business_roles.go (rssi, dsi, risk_manager, auditor,
-- compliance_officer, internal_control, asset_owner, risk_owner,
-- security_analyst, executive, viewer), resolved to permissions by
-- OrganizationMember.EffectivePermissions().
--
-- GORM's AutoMigrate already adds this column (OrganizationMember is in
-- AutoMigrate); this file makes a migrations-only deploy self-sufficient. The
-- column is nullable/empty: root/admin members and legacy profile-driven users
-- keep an empty business_role.

ALTER TABLE organization_members
    ADD COLUMN IF NOT EXISTS business_role VARCHAR(64);

CREATE INDEX IF NOT EXISTS idx_organization_members_business_role
    ON organization_members (business_role);
