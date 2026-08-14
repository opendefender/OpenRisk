-- Migration: join table between risks and assets (many-to-many)

CREATE TABLE IF NOT EXISTS risk_assets (
  risk_id uuid NOT NULL REFERENCES risks(id) ON DELETE CASCADE,
  asset_id uuid NOT NULL,
  PRIMARY KEY (risk_id, asset_id)
);

CREATE INDEX IF NOT EXISTS idx_risk_assets_asset ON risk_assets (asset_id);
