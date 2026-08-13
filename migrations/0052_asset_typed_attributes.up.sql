-- Attack Surface §1 — typed attributes by asset category.
--
-- One schema row per (tenant, category): the tenant-editable contract that the
-- asset form is generated from and every asset write is validated against.

CREATE TABLE IF NOT EXISTS asset_type_schemas (
    id          uuid PRIMARY KEY,
    tenant_id   uuid        NOT NULL,
    category    varchar(32) NOT NULL,
    label       varchar(128),
    attributes  jsonb       NOT NULL DEFAULT '[]'::jsonb,
    customized  boolean     NOT NULL DEFAULT false,
    version     integer     NOT NULL DEFAULT 1,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- Exactly one schema per category per tenant. This is also what makes the
-- seed-on-first-read path safe: two concurrent readers racing to seed the same
-- default collide here instead of creating two competing schemas.
CREATE UNIQUE INDEX IF NOT EXISTS ux_asset_schema_tenant_category
    ON asset_type_schemas (tenant_id, category);

-- Assets gain their category, their typed attribute bag, and the denormalised
-- correlation fingerprints extracted from that bag.
ALTER TABLE assets ADD COLUMN IF NOT EXISTS category          varchar(32);
ALTER TABLE assets ADD COLUMN IF NOT EXISTS attributes        jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE assets ADD COLUMN IF NOT EXISTS hostnames         text[];
ALTER TABLE assets ADD COLUMN IF NOT EXISTS ip_addresses      text[];
ALTER TABLE assets ADD COLUMN IF NOT EXISTS cloud_resource_id varchar(512);

CREATE INDEX IF NOT EXISTS idx_assets_category ON assets (category);
CREATE INDEX IF NOT EXISTS idx_assets_cloud_resource_id ON assets (cloud_resource_id);

-- GIN indexes on the fingerprint arrays: the vulnerability↔asset correlator
-- resolves every ingested finding against them, so a 10k-finding import would
-- otherwise be a 10k-times sequential scan of the inventory.
CREATE INDEX IF NOT EXISTS idx_assets_hostnames_gin ON assets USING gin (hostnames);
CREATE INDEX IF NOT EXISTS idx_assets_ips_gin       ON assets USING gin (ip_addresses);
CREATE INDEX IF NOT EXISTS idx_assets_attributes_gin ON assets USING gin (attributes);

-- Backfill: give existing assets a category derived from their free-text type,
-- so a pre-existing inventory is typed rather than blank. Nothing is invented —
-- a type that does not map to a category is left NULL and the asset simply has
-- no typed attributes until someone assigns it one.
UPDATE assets SET category = CASE lower(coalesce(type, ''))
    WHEN 'server'      THEN 'server'
    WHEN 'laptop'      THEN 'workstation'
    WHEN 'workstation' THEN 'workstation'
    WHEN 'application' THEN 'application'
    WHEN 'saas'        THEN 'application'
    WHEN 'database'    THEN 'database'
    WHEN 'network'     THEN 'network'
    WHEN 'cloud'       THEN 'cloud'
    WHEN 'storage'     THEN 'cloud'
    WHEN 'supplier'    THEN 'vendor'
    WHEN 'vendor'      THEN 'vendor'
    WHEN 'data'        THEN 'data_processing'
    ELSE NULL
END
WHERE category IS NULL;
