-- Migration: 0046_risk_taxonomy.up.sql
-- Purpose: separate three concepts that shared one column and one badge.
--
--   tags             — free text, user-authored          → colonne « Étiquettes »
--   categories       — vocabulaire CONTRÔLÉ par tenant   → colonne « Catégorie »
--   control_mappings — référence à un contrôle réel      → colonne « Référentiel »
--
-- The reported bug, literally: the register's "Référentiel" column read
--
--     fw = risk.frameworks[0] ?? risk.tags[0]
--
-- so a user's free-text LABEL was rendered with a framework badge whenever the
-- frameworks array was empty. And `frameworks` itself was free text produced by
-- a hard-coded dropdown (ISO27001/CIS/NIST/OWASP) that was never checked against
-- the frameworks the tenant had actually imported — so even the non-fallback
-- case was a string, not a reference.
--
-- MIGRATION STRATEGY (validated before execution):
--   1. Each entry of `risks.frameworks` is RESOLVED against the tenant's
--      compliance_frameworks by normalised name.
--   2. Resolved   → a framework-level row in risk_control_mappings.
--   3. Unresolved → it was a label all along: appended to `tags`, deduplicated.
--   4. `control_ids` → resolved against compliance_controls.reference_code.
--   5. `risks.frameworks` is FROZEN, not dropped: no longer read or written,
--      kept one release so a rollback is possible.

-- ---------------------------------------------------------------- categories
CREATE TABLE IF NOT EXISTS risk_categories (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    name        VARCHAR(120) NOT NULL,
    slug        VARCHAR(120) NOT NULL,
    description TEXT,
    color       VARCHAR(32) DEFAULT 'neutral',
    sort_order  INTEGER DEFAULT 0,
    active      BOOLEAN DEFAULT TRUE,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

-- Controlled means unique per tenant: two entries with the same key would let
-- the same concept be counted twice in a dashboard, which is what tags are for
-- and categories are not.
CREATE UNIQUE INDEX IF NOT EXISTS idx_risk_categories_tenant_slug
    ON risk_categories (tenant_id, slug);
CREATE INDEX IF NOT EXISTS idx_risk_categories_active ON risk_categories (active);
CREATE INDEX IF NOT EXISTS idx_risk_categories_deleted_at ON risk_categories (deleted_at);

ALTER TABLE risks ADD COLUMN IF NOT EXISTS category_id UUID;
CREATE INDEX IF NOT EXISTS idx_risks_category_id ON risks (category_id);

-- ---------------------------------------------------------- control mappings
CREATE TABLE IF NOT EXISTS risk_control_mappings (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL,
    risk_id      UUID NOT NULL,
    framework_id UUID NOT NULL,
    -- NULL = the mapping names a framework but no specific control. The data
    -- migration below can only ever be that honest about the old free-text
    -- values, and the UI links to the framework's control list in that case.
    control_id   UUID,
    note         TEXT,
    created_by   UUID,
    source       VARCHAR(20) DEFAULT 'manual',
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    updated_at   TIMESTAMPTZ DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_risk_control_mappings_tenant ON risk_control_mappings (tenant_id);
CREATE INDEX IF NOT EXISTS idx_risk_control_mappings_risk ON risk_control_mappings (risk_id);
CREATE INDEX IF NOT EXISTS idx_risk_control_mappings_framework ON risk_control_mappings (framework_id);
CREATE INDEX IF NOT EXISTS idx_risk_control_mappings_control ON risk_control_mappings (control_id);
CREATE INDEX IF NOT EXISTS idx_risk_control_mappings_deleted_at ON risk_control_mappings (deleted_at);

-- Two rows for the same (risk, framework, control) are the same statement made
-- twice. Partial indexes because NULL never equals NULL in a unique index.
CREATE UNIQUE INDEX IF NOT EXISTS idx_risk_control_mappings_unique_control
    ON risk_control_mappings (risk_id, control_id)
    WHERE control_id IS NOT NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_risk_control_mappings_unique_framework
    ON risk_control_mappings (risk_id, framework_id)
    WHERE control_id IS NULL AND deleted_at IS NULL;

-- ===========================================================================
-- DATA MIGRATION
-- ===========================================================================

-- Step 1 — `control_ids` → control-level mappings, where the reference code
-- matches a control the tenant actually holds. This column was effectively dead
-- (nothing wrote it but risk duplication) so this is expected to move few rows;
-- it runs first because a control-level mapping is strictly better information
-- than the framework-level one step 2 would otherwise create.
INSERT INTO risk_control_mappings (tenant_id, risk_id, framework_id, control_id, source, note)
SELECT DISTINCT r.tenant_id, r.id, c.framework_id, c.id, 'import',
       'Migré depuis risks.control_ids (migration 0046)'
  FROM risks r
  CROSS JOIN LATERAL unnest(COALESCE(r.control_ids, '{}')) AS code
  JOIN compliance_controls c
    ON c.tenant_id = r.tenant_id
   AND lower(c.reference_code) = lower(code)
   AND c.deleted_at IS NULL
 WHERE r.deleted_at IS NULL
ON CONFLICT DO NOTHING;

-- Step 2 — `frameworks` entries that RESOLVE to a framework the tenant imported
-- become framework-level mappings.
--
-- Matching is on the normalised name: 'ISO27001' → 'ISO 27001', 'NIST-CSF' →
-- 'NIST CSF'. Spaces, hyphens, dots and underscores are stripped from both
-- sides, and the version is tried as a suffix so 'ISO270012022' also matches
-- "ISO 27001" version "2022".
INSERT INTO risk_control_mappings (tenant_id, risk_id, framework_id, control_id, source, note)
SELECT DISTINCT r.tenant_id, r.id, f.id, NULL, 'import',
       'Migré depuis risks.frameworks (migration 0046)'
  FROM risks r
  CROSS JOIN LATERAL unnest(COALESCE(r.frameworks, '{}')) AS fw
  JOIN compliance_frameworks f
    ON f.tenant_id = r.tenant_id
   AND f.deleted_at IS NULL
   AND (
        regexp_replace(lower(f.name), '[^a-z0-9]', '', 'g')
          = regexp_replace(lower(fw), '[^a-z0-9]', '', 'g')
     OR regexp_replace(lower(f.name || COALESCE(f.version, '')), '[^a-z0-9]', '', 'g')
          = regexp_replace(lower(fw), '[^a-z0-9]', '', 'g')
   )
 WHERE r.deleted_at IS NULL
   -- Do not add a framework-level row when step 1 already produced a
   -- control-level one for the same framework: it would be strictly less
   -- precise and render a second, vaguer badge.
   AND NOT EXISTS (
        SELECT 1 FROM risk_control_mappings m
         WHERE m.risk_id = r.id AND m.framework_id = f.id AND m.control_id IS NOT NULL
   )
ON CONFLICT DO NOTHING;

-- Step 3 — the unresolved entries were LABELS. They move to `tags`, which is
-- where free text belongs, deduplicated against what is already there.
--
-- This is the decision that repairs the reported bug for existing data: after
-- this, the "Référentiel" column can only ever render a real reference, and the
-- values that used to squat there show up under "Étiquettes" where they read as
-- what they are.
UPDATE risks r
   SET tags = (
        SELECT ARRAY(
            SELECT DISTINCT t
              FROM unnest(
                    COALESCE(r.tags, '{}') ||
                    ARRAY(
                        SELECT fw
                          FROM unnest(COALESCE(r.frameworks, '{}')) AS fw
                         WHERE NOT EXISTS (
                            SELECT 1 FROM compliance_frameworks f
                             WHERE f.tenant_id = r.tenant_id
                               AND f.deleted_at IS NULL
                               AND (
                                    regexp_replace(lower(f.name), '[^a-z0-9]', '', 'g')
                                      = regexp_replace(lower(fw), '[^a-z0-9]', '', 'g')
                                 OR regexp_replace(lower(f.name || COALESCE(f.version, '')), '[^a-z0-9]', '', 'g')
                                      = regexp_replace(lower(fw), '[^a-z0-9]', '', 'g')
                               )
                         )
                    )
              ) AS t
             WHERE t IS NOT NULL AND t <> ''
        )
   )
 WHERE r.deleted_at IS NULL
   AND COALESCE(array_length(r.frameworks, 1), 0) > 0;

-- Step 4 — seed the controlled vocabulary for every tenant that has risks and
-- no categories. Without it the "Catégorie" column ships dead on arrival. An
-- admin can rename, reorder or deactivate any of these; nothing is classified
-- automatically, because guessing a risk's category from its title would be
-- exactly the kind of invention this migration exists to remove.
INSERT INTO risk_categories (tenant_id, name, slug, description, color, sort_order, active)
SELECT t.tenant_id, v.name, v.slug, v.description, v.color, v.sort_order, TRUE
  FROM (SELECT DISTINCT tenant_id FROM risks WHERE deleted_at IS NULL) t
  CROSS JOIN (VALUES
      ('Cybersécurité',          'cybersecurite',        'Menaces techniques : intrusion, malware, exfiltration.',   'critical', 0),
      ('Conformité',             'conformite',           'Manquement à une exigence réglementaire ou contractuelle.', 'high',     1),
      ('Opérationnel',           'operationnel',         'Défaillance de processus, d''outil ou de personne.',        'medium',   2),
      ('Financier',              'financier',            'Perte, fraude ou exposition monétaire directe.',            'high',     3),
      ('Fournisseurs & tiers',   'fournisseurs-tiers',   'Dépendance à un prestataire ou à un sous-traitant.',        'medium',   4),
      ('Continuité d''activité', 'continuite-d-activite','Indisponibilité durable d''un service essentiel.',          'high',     5),
      ('Données personnelles',   'donnees-personnelles', 'Traitement de données à caractère personnel.',              'critical', 6),
      ('Réputation',             'reputation',           'Atteinte à l''image auprès des clients ou du régulateur.',  'low',      7)
  ) AS v(name, slug, description, color, sort_order)
 WHERE NOT EXISTS (
        SELECT 1 FROM risk_categories rc WHERE rc.tenant_id = t.tenant_id
   )
ON CONFLICT DO NOTHING;

-- Step 5 — `risks.frameworks` and `risks.control_ids` are now FROZEN. They are
-- deliberately NOT dropped: keeping them one release makes 0046 reversible and
-- lets a deployment compare before/after. A later migration drops them once
-- this one has proven itself in production.
COMMENT ON COLUMN risks.frameworks IS
    'DEPRECATED (0046): free-text framework names from a hard-coded dropdown. Frozen — superseded by risk_control_mappings. Dropped in a later migration.';
COMMENT ON COLUMN risks.control_ids IS
    'DEPRECATED (0046): free-text control codes, never read. Migrated into risk_control_mappings. Dropped in a later migration.';
