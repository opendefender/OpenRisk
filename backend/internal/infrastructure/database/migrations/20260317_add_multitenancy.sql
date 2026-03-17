-- Migration 20260317: Add Multi-Tenancy System
-- Implements complete organization-based multi-tenancy with IAM profiles and permissions
-- Preserves all existing tables and adds new multi-tenant schema

-- ═══════════════════════════════════════════════════════
-- STEP 1.1 — Extend existing users table (don't drop it)
-- ═══════════════════════════════════════════════════════

-- Add new columns to existing users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS first_name VARCHAR(100);
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_name VARCHAR(100);
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_verified BOOLEAN DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS mfa_enabled BOOLEAN DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS mfa_secret TEXT; -- encrypted TOTP secret
ALTER TABLE users ADD COLUMN IF NOT EXISTS default_org_id UUID; -- FK added after orgs table
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NOW();

-- ═══════════════════════════════════════════════════════
-- STEP 1.2 — Organizations (each org = 1 isolated tenant)
-- ═══════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS organizations (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name        VARCHAR(255) NOT NULL,
  slug        VARCHAR(100) UNIQUE NOT NULL,
  logo_url    TEXT,
  industry    VARCHAR(100),
  size        VARCHAR(50) CHECK (size IN ('1-50','51-200','201-1000','1000+')),
  plan        VARCHAR(50) NOT NULL DEFAULT 'starter'
                CHECK (plan IN ('free','starter','professional','enterprise')),
  owner_id    UUID NOT NULL REFERENCES users(id),
  is_active   BOOLEAN DEFAULT true,
  settings    JSONB NOT NULL DEFAULT '{}',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Now add the FK from users to organizations
ALTER TABLE users
  ADD CONSTRAINT fk_users_default_org
  FOREIGN KEY (default_org_id) REFERENCES organizations(id) ON DELETE SET NULL;

-- ═══════════════════════════════════════════════════════
-- STEP 1.3 — Profiles (IAM-style, created per org by Root/Admin)
-- ═══════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS profiles (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name            VARCHAR(100) NOT NULL,
  description     TEXT,
  is_system       BOOLEAN DEFAULT false, -- true = built-in profile, not deletable
  is_default      BOOLEAN DEFAULT false, -- auto-assigned to new org members
  created_by      UUID NOT NULL REFERENCES users(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE (organization_id, name)
);

-- ═══════════════════════════════════════════════════════
-- STEP 1.4 — Profile permissions (granular IAM rules)
-- ═══════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS profile_permissions (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
  resource   VARCHAR(50) NOT NULL
               CHECK (resource IN (
                 'risks','assets','mitigations','users','audit_logs','settings',
                 'members','profiles','reports','integrations','connectors','groups'
               )),
  action     VARCHAR(20) NOT NULL
               CHECK (action IN ('read','write','delete','manage','export','assign')),
  scope      VARCHAR(20) NOT NULL
               CHECK (scope IN ('all','assigned','own','none')),
  conditions JSONB DEFAULT '{}',

  UNIQUE (profile_id, resource, action)
);

-- ═══════════════════════════════════════════════════════
-- STEP 1.5 — Organization members (user ↔ org membership)
-- ═══════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS organization_members (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role            VARCHAR(20) NOT NULL
                    CHECK (role IN ('root','admin','user')),
  profile_id      UUID REFERENCES profiles(id) ON DELETE SET NULL,
  -- profile_id is only relevant when role = 'user'
  -- root and admin have hardcoded full permissions
  is_active       BOOLEAN DEFAULT true,
  joined_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  invited_by      UUID REFERENCES users(id),

  UNIQUE (organization_id, user_id)
  -- one membership record per user per org; role stored here
);

-- ═══════════════════════════════════════════════════════
-- STEP 1.6 — Invitations
-- ═══════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS invitations (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  token           UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  email           VARCHAR(255) NOT NULL,
  role            VARCHAR(20) NOT NULL
                    CHECK (role IN ('root','admin','user')),
  profile_id      UUID REFERENCES profiles(id) ON DELETE SET NULL,
  status          VARCHAR(20) NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','accepted','expired','revoked')),
  expires_at      TIMESTAMPTZ NOT NULL,
  invited_by      UUID REFERENCES users(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ═══════════════════════════════════════════════════════
-- STEP 1.7 — Sessions (track active sessions per tenant)
-- ═══════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS user_sessions (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  token_hash      TEXT NOT NULL,
  ip_address      INET,
  user_agent      TEXT,
  expires_at      TIMESTAMPTZ NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ═══════════════════════════════════════════════════════
-- STEP 1.8 — Audit logs
-- ═══════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS audit_logs (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id         UUID REFERENCES users(id) ON DELETE SET NULL,
  action          VARCHAR(100) NOT NULL,
  resource_type   VARCHAR(50),
  resource_id     UUID,
  details         JSONB DEFAULT '{}',
  ip_address      INET,
  user_agent      TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ═══════════════════════════════════════════════════════
-- STEP 1.9 — Indexes for performance
-- ═══════════════════════════════════════════════════════

CREATE INDEX IF NOT EXISTS idx_org_members_user ON organization_members(user_id);
CREATE INDEX IF NOT EXISTS idx_org_members_org ON organization_members(organization_id);
CREATE INDEX IF NOT EXISTS idx_invitations_email ON invitations(email);
CREATE INDEX IF NOT EXISTS idx_invitations_token ON invitations(token);
CREATE INDEX IF NOT EXISTS idx_invitations_org ON invitations(organization_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON user_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_token ON user_sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_audit_org ON audit_logs(organization_id);
CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_profile_perms ON profile_permissions(profile_id);

-- ═══════════════════════════════════════════════════════
-- STEP 1.10 — Row-Level Security (ALL existing tables)
-- ═══════════════════════════════════════════════════════

-- Add organization_id to all existing tables that are org-scoped
-- Check which tables exist first, then add if missing:
ALTER TABLE risks ADD COLUMN IF NOT EXISTS organization_id UUID REFERENCES organizations(id);
ALTER TABLE assets ADD COLUMN IF NOT EXISTS organization_id UUID REFERENCES organizations(id);
ALTER TABLE mitigations ADD COLUMN IF NOT EXISTS organization_id UUID REFERENCES organizations(id);

-- Add indexes for org-scoped queries
CREATE INDEX IF NOT EXISTS idx_risks_org ON risks(organization_id);
CREATE INDEX IF NOT EXISTS idx_assets_org ON assets(organization_id);
CREATE INDEX IF NOT EXISTS idx_mitigations_org ON mitigations(organization_id);