-- Reverse of 0052_asset_typed_attributes.

DROP INDEX IF EXISTS idx_assets_attributes_gin;
DROP INDEX IF EXISTS idx_assets_ips_gin;
DROP INDEX IF EXISTS idx_assets_hostnames_gin;
DROP INDEX IF EXISTS idx_assets_cloud_resource_id;
DROP INDEX IF EXISTS idx_assets_category;

ALTER TABLE assets DROP COLUMN IF EXISTS cloud_resource_id;
ALTER TABLE assets DROP COLUMN IF EXISTS ip_addresses;
ALTER TABLE assets DROP COLUMN IF EXISTS hostnames;
ALTER TABLE assets DROP COLUMN IF EXISTS attributes;
ALTER TABLE assets DROP COLUMN IF EXISTS category;

DROP INDEX IF EXISTS ux_asset_schema_tenant_category;
DROP TABLE IF EXISTS asset_type_schemas;
