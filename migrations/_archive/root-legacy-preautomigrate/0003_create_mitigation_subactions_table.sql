-- Migration: create mitigation_subactions table

CREATE TABLE IF NOT EXISTS mitigation_subactions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  mitigation_id uuid NOT NULL,
  title text NOT NULL,
  completed boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_mitigation_subactions_mitigation_id ON mitigation_subactions (mitigation_id);

-- trigger to update updated_at
DROP TRIGGER IF EXISTS trg_update_mitigation_subactions_updated_at ON mitigation_subactions;
CREATE TRIGGER trg_update_mitigation_subactions_updated_at
BEFORE UPDATE ON mitigation_subactions
FOR EACH ROW EXECUTE PROCEDURE fn_update_updated_at();
