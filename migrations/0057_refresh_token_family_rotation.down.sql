DROP INDEX IF EXISTS idx_refresh_tokens_rotated_at;
DROP INDEX IF EXISTS idx_refresh_tokens_family_id;
ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS rotated_at;
ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS family_id;
