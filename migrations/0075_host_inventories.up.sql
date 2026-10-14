-- #910 — the software inventory the OpenRisk agent reports for its host.
--
-- One row per (tenant, host) holds the latest snapshot; its packages are
-- replaced as a whole on every inventory. The backend checks those versions
-- online (OSV, NVD) and writes findings to the vulnerability register. An
-- inventory never creates an asset: asset_id is set only when one matches.
-- Idempotent: AutoMigrate creates the same tables on boot.

CREATE TABLE IF NOT EXISTS host_inventories (
    id                uuid PRIMARY KEY,
    tenant_id         uuid NOT NULL,
    agent_id          uuid,
    host_key          varchar(255) NOT NULL,
    hostname          varchar(255),
    asset_id          uuid,
    platform          varchar(32),
    os                varchar(128),
    os_version        varchar(128),
    ecosystem         varchar(64),
    package_count     integer NOT NULL DEFAULT 0,
    uncheckable_count integer NOT NULL DEFAULT 0,
    collected_at      timestamptz NOT NULL,
    matched_at        timestamptz,
    match_error       varchar(64),
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_host_inventory_host ON host_inventories (tenant_id, host_key);
CREATE INDEX IF NOT EXISTS idx_host_inventories_asset_id ON host_inventories (asset_id);

CREATE TABLE IF NOT EXISTS host_packages (
    id           uuid PRIMARY KEY,
    tenant_id    uuid NOT NULL,
    inventory_id uuid NOT NULL,
    name         varchar(256) NOT NULL,
    version      varchar(256),
    source       varchar(16),
    publisher    varchar(256),
    arch         varchar(32)
);
CREATE INDEX IF NOT EXISTS idx_host_packages_tenant_id ON host_packages (tenant_id);
CREATE INDEX IF NOT EXISTS idx_host_packages_inventory_id ON host_packages (inventory_id);
