-- Activation & onboarding — the server-side source of truth for "how far into the
-- product is this tenant / this user?".
--
-- Why a table rather than client state: activation used to be derived in the
-- browser (localStorage + counts computed in the checklist component), which is
-- what produced the whole bug family — a checklist that never ticked, confetti
-- firing on re-render, and two rows striking through after a single import
-- because two client-side steps read the same count. Events are recorded by the
-- domain use cases; the read model only ever takes the FIRST occurrence per key,
-- and a step is bound to exactly one key (enforced in Go by
-- domain.ValidateActivationSteps).
--
-- Idempotent: AutoMigrate creates these tables too, this file exists so a
-- migrate-only deployment has the same schema, with the partial/covering indexes
-- AutoMigrate cannot express.

-- 1. Append-only activation event log. No updates, no deletes.
CREATE TABLE IF NOT EXISTS activation_events (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid        NOT NULL,
    user_id     uuid,
    event_key   varchar(64) NOT NULL,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    payload     jsonb
);

-- The read path is exclusively "first occurrence per key, for one tenant".
CREATE INDEX IF NOT EXISTS idx_activation_tenant_key
    ON activation_events (tenant_id, event_key, occurred_at);
CREATE INDEX IF NOT EXISTS idx_activation_events_user
    ON activation_events (user_id);
CREATE INDEX IF NOT EXISTS idx_activation_events_occurred_at
    ON activation_events (occurred_at);

-- 2. Per-user celebration ledger. This is what makes the burst fire exactly once
--    per step per user, across reloads and devices: the client never decides.
CREATE TABLE IF NOT EXISTS activation_celebrations (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     uuid        NOT NULL,
    user_id       uuid        NOT NULL,
    step_key      varchar(64) NOT NULL,
    celebrated_at timestamptz NOT NULL DEFAULT now()
);

-- The uniqueness IS the idempotency: a double POST is an upsert no-op.
CREATE UNIQUE INDEX IF NOT EXISTS idx_activation_celebration
    ON activation_celebrations (user_id, step_key);
CREATE INDEX IF NOT EXISTS idx_activation_celebrations_tenant
    ON activation_celebrations (tenant_id);

-- 3. Resumable signup wizard state — one row per USER (not per tenant): a
--    teammate invited into an already-configured organization still walks their
--    own profile step, and the route guard is a statement about a person.
CREATE TABLE IF NOT EXISTS onboarding_progress (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    uuid        NOT NULL,
    user_id      uuid        NOT NULL,
    current_step varchar(32) NOT NULL DEFAULT 'organization',
    completed    boolean     NOT NULL DEFAULT false,
    completed_at timestamptz,
    industry     varchar(64),
    country      varchar(64),
    goal         varchar(64),
    answers      jsonb,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_onboarding_progress_user
    ON onboarding_progress (user_id);
CREATE INDEX IF NOT EXISTS idx_onboarding_progress_tenant
    ON onboarding_progress (tenant_id);

-- 4. Backfill: existing tenants are already past onboarding. Marking them
--    complete is what keeps the new route guard from trapping current users
--    behind a wizard they never asked for.
INSERT INTO onboarding_progress (tenant_id, user_id, current_step, completed, completed_at)
SELECT om.organization_id, om.user_id, 'team', true, now()
FROM organization_members om
ON CONFLICT (user_id) DO NOTHING;

-- 5. Backfill the signup anchor for existing tenants so time-to-Aha has a t0 for
--    them (their org creation date is the honest anchor).
INSERT INTO activation_events (tenant_id, user_id, event_key, occurred_at, payload)
SELECT o.id, o.owner_id, 'signup', o.created_at, '{"backfilled": true}'::jsonb
FROM organizations o
WHERE NOT EXISTS (
    SELECT 1 FROM activation_events ae
    WHERE ae.tenant_id = o.id AND ae.event_key = 'signup'
);
