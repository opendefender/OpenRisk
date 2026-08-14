-- Migration: create risks table

CREATE TABLE IF NOT EXISTS risks (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  title text NOT NULL,
  description text,
  impact integer NOT NULL CHECK (impact >= 0),
  probability integer NOT NULL CHECK (probability >= 0),
  score numeric(8,2) NOT NULL DEFAULT 0,
  status varchar(32) NOT NULL DEFAULT 'OPEN',
  tags text[],
  frameworks text[],
  source text,
  source_id text,
  custom_fields jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_risks_score ON risks (score DESC);
CREATE INDEX IF NOT EXISTS idx_risks_status ON risks (status);
CREATE INDEX IF NOT EXISTS idx_risks_tags ON risks USING GIN (tags);
CREATE INDEX IF NOT EXISTS idx_risks_frameworks ON risks USING GIN (frameworks);
CREATE INDEX IF NOT EXISTS idx_risks_custom ON risks USING GIN (custom_fields);

-- Trigger to update updated_at on row change
CREATE OR REPLACE FUNCTION fn_update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_update_risks_updated_at ON risks;
CREATE TRIGGER trg_update_risks_updated_at
BEFORE UPDATE ON risks
FOR EACH ROW EXECUTE PROCEDURE fn_update_updated_at();
