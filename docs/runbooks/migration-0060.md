# Runbook — upgrading across migration 0060 (MFA grace anchor)

Applies to every upgrade from a release whose SQL layer is at version 59 or
below (`v1.1.0-rc.3` and earlier) to a release that ships
`migrations/0060_mfa_deferred_enrollment` (`v1.1.0-rc.4` and later). Issue #349.

The general mechanics (AutoMigrate first, then the SQL layer, dirty-state
recovery) are in [migrations.md](migrations.md). This page covers what is
specific to 0060 and to 0065, which completes it.

## What the upgrade changes

| Object | Change | Written by |
|---|---|---|
| `organization_members.mfa_grace_started_at` | new `TIMESTAMPTZ NULL` column | AutoMigrate, then 0060 (`ADD COLUMN IF NOT EXISTS`) |
| same column, existing rows | set to `COALESCE(joined_at, created_at)` where NULL | 0060 |
| `mfa_policies` | new table, one row per tenant, no rows at upgrade | AutoMigrate |
| `mfa_policies` | `CHECK (grace_days BETWEEN 0 AND 90)`, default `grace_days = 7`, default `id = gen_random_uuid()` | 0065 |

0065 exists because AutoMigrate creates `mfa_policies` before the SQL layer
runs, so the `CREATE TABLE IF NOT EXISTS` in 0060, which carries the CHECK and
the defaults, never runs on a server boot. Without 0065 a direct SQL write could
store `grace_days = 36500`, which switches the MFA requirement off.

Nothing else in `organization_members` is written: the upgrade test compares
every other column of every row, `updated_at` included, before and after.

### Behaviour change users will notice

The countdown for a privileged member without a verified authenticator runs
from when their membership began. With the default 7-day window:

- An admin or root who joined more than 7 days ago is **required** to enrol at
  their next login. The previous release already refused them a session
  without MFA, so nothing changes for them.
- An admin or root who joined less than 7 days ago now gets the rest of their
  window instead of an immediate wall.
- **New:** members with the `rssi` business role are privileged by default
  (`MFA_REQUIRED_BUSINESS_ROLES`). The previous release did not require MFA from
  them. Those who joined more than 7 days ago and have no verified
  authenticator are required to enrol at their next login. Count them before
  the upgrade (pre-check 4) and tell them.

## Measured cost

Upgrade test (`TestMigration0060_Success`, PostgreSQL 16, 4 vCPU, local disk),
time spent in the SQL layer from 0060 to head, almost all of it the 0060
backfill:

| Memberships | SQL layer |
|---|---|
| 20 000 | 0.6 s |
| 200 000 | 7.8 s |
| 1 000 000 | 48 s |

Real upgrade rehearsed with the `v1.1.0-rc.3` binary and 200 001 memberships:
AutoMigrate plus the SQL layer took 8 s.

**Locks.** 0060's `ALTER TABLE` and its `UPDATE` share one transaction, so the
ACCESS EXCLUSIVE lock on `organization_members` is held until the backfill
commits (`TestMigration0060_LockImpact`). For that whole time, every read and
write of memberships waits: login, token refresh and member management, on
every replica still serving the previous release. Size the maintenance window
from the table above.

**Boot time after the migration.** `SeedRBAC` visits every membership on every
boot, one query each. With 200 000 memberships the server started listening
about 9 minutes after the migrations finished, in both rehearsals. This is not
caused by 0060, but it decides the probe settings below.

## Before the upgrade

1. **Back up.** `pg_dump -Fc` or a storage snapshot. The rollback below does not
   need it, but a failed restore of anything else will.
2. **Run the pre-checks** against the production database:

   ```sql
   -- 1. The SQL layer is where the previous release left it: 59, not dirty.
   SELECT version, dirty FROM schema_migrations;

   -- 2. Volume, for the maintenance window.
   SELECT count(*) FROM organization_members;

   -- 3. Memberships with no start date at all. 0060 cannot anchor them; the
   --    application treats them as "required now" (fail closed).
   SELECT count(*) FROM organization_members WHERE joined_at IS NULL AND created_at IS NULL;

   -- 4. Members who will be required to enrol at their next login
   --    (default privileged set: root, admin, rssi; default window: 7 days).
   SELECT m.role, m.business_role, count(*)
     FROM organization_members m
     LEFT JOIN mfa_secrets s ON s.user_id = m.user_id AND s.is_verified
    WHERE s.user_id IS NULL
      AND (m.role IN ('root', 'admin') OR m.business_role = 'rssi')
      AND COALESCE(m.status, 'active') = 'active'
      AND COALESCE(m.joined_at, m.created_at, '-infinity') < now() - interval '7 days'
    GROUP BY 1, 2;

   -- 5. Nothing holds organization_members for long. 0060 waits behind any of
   --    these, and every login waits behind 0060.
   SELECT pid, now() - xact_start AS open_for, state, left(query, 80)
     FROM pg_stat_activity
    WHERE xact_start < now() - interval '1 minute' AND pid <> pg_backend_pid();
   ```

3. **Give the first pod time to finish.** The chart's liveness probe kills a
   backend that has not answered `/health` after about 60 s. A pod killed during
   0060 leaves the database dirty at version 60, and every later pod refuses to
   boot. For the upgrade rollout only:

   ```bash
   helm upgrade openrisk ./helm/openrisk -n openrisk \
     --set backend.replicaCount=1 \
     --set backend.livenessProbe.initialDelaySeconds=1800 \
     --set backend.readinessProbe.initialDelaySeconds=60
   ```

   One replica means one process runs the schema upgrade. Restore the usual
   values once the pod is ready.

4. **Do not run `migrate up` on its own before the new pods start.** The SQL
   layer relies on AutoMigrate having run first. From a `v1.1.0-rc.3` database,
   `openrisk migrate up` fails at 0062 (`relation "vendor_assessment_tokens"
   does not exist`) and leaves the database dirty at 62. If it has already
   happened: `openrisk migrate force 61`, then boot the new release normally.
   This was rehearsed.

## During the upgrade

The new release's first boot runs `PrepareForAutoMigrate`, AutoMigrate, then
0060 to 0065. The line to wait for:

```
migrations: applied successfully (version=65 dirty=false)
```

## After the upgrade

Run the post-checks. Each must return the value shown. They are the
invariants `TestMigration0060_Success` asserts.

```sql
-- Version 65 (or later), not dirty.
SELECT version, dirty FROM schema_migrations;

-- 0: every membership that existed before the upgrade is anchored to its start.
--    Run it right after the upgrade: a promotion re-anchors a member on purpose.
SELECT count(*) FROM organization_members
 WHERE mfa_grace_started_at IS DISTINCT FROM COALESCE(joined_at, created_at)
   AND (created_at IS NULL OR created_at < '<upgrade start time>');

-- Equal: an anchor stays NULL only where the row has no start date.
SELECT count(*) FILTER (WHERE mfa_grace_started_at IS NULL),
       count(*) FILTER (WHERE joined_at IS NULL AND created_at IS NULL)
  FROM organization_members;

-- 1: the grace_days bound exists and is validated.
SELECT count(*) FROM pg_constraint
 WHERE conrelid = 'mfa_policies'::regclass
   AND conname = 'mfa_policies_grace_days_bounds' AND convalidated;

-- grace_days = 7, id = gen_random_uuid().
SELECT column_name, column_default FROM information_schema.columns
 WHERE table_name = 'mfa_policies' AND column_name IN ('id', 'grace_days');
```

Memberships created after the upgrade keep `mfa_grace_started_at` NULL: nothing
writes it at creation, and the application reads the anchor from `joined_at`
(`OrganizationMember.MFAGraceAnchor`). This is expected, and the post-check above
ignores them through the `created_at` filter.

## Rolling back

**Roll back the application, keep the schema.** The column and the table are
additive, and the previous release ignores them.

The previous release refuses to boot on the upgraded database, because it does
not know version 65:

```
SQL migrations failed: migrations: apply: no migration found for version 65: read down for version 65 .: file does not exist
```

So `helm rollback` alone ends in a crash loop. The procedure, as rehearsed with
`v1.1.0-rc.3`:

1. Scale the new release to zero.
2. With the **previous** image, record the previous version. This writes the
   version number only; no SQL runs:

   ```bash
   docker run --rm -e DATABASE_URL="$DATABASE_URL" <previous-image> migrate force 59
   # or, in the cluster, a one-off pod from the previous image with the same arguments
   ```
3. Deploy the previous release (`helm rollback`). It boots and serves.
4. To upgrade again later, deploy the new release as above. 0060 to 0065 run
   again and are idempotent: tenant policies and promotion anchors are kept, and
   memberships the previous release created in the meantime are anchored to
   their `joined_at` (`TestMigration0060_Idempotent`, and the rehearsal).

**Do not run the down migrations** (`migrate down`, `make migrate-rollback`) to
leave 0060. They drop `mfa_policies` and `mfa_grace_started_at`, and a
re-upgrade cannot restore either: every tenant's configured window returns to
the 7-day default, and every promotion anchor falls back to the membership
start, which can make a recently promoted admin required at once
(`TestMigration0060_Rollback`).

## If the migration is interrupted

0060 runs in one transaction. A failure part-way through the backfill (killed
pod, lock timeout, lost connection) writes no anchor at all
(`TestMigration0060_InterruptedBackfill` stops it half-way through 4 000 rows and
finds none written). golang-migrate records version 60 as dirty, and the next
boot stops with:

```
migrations: database is DIRTY at version 60 — a previous migration failed mid-apply, ...
```

Recovery:

1. Find the cause in the log of the pod that failed: liveness kill, lock
   timeout, disk full.
2. Remove it: see pre-checks 3 and 5.
3. `openrisk migrate force 59`, then boot the new release again.

The same applies to 0065 (force 64). Both recoveries are covered by
`TestMigration0060_InterruptedBackfill` and `TestMigration0060_LockTimeout`.

## Evidence

| Requirement (#349) | Where it is proven |
|---|---|
| Backfill correctness | `TestMigration0060_Success`, rehearsal post-checks (200 001 rows, 0 wrong anchors) |
| NULL handling | `TestMigration0060_NotFound`: no start date gives NULL, and the decision fails closed |
| Existing accounts | `TestMigration0060_Success`: a 6-month-old admin is required; a 1-hour-old admin keeps their window; `joined_at` wins over `created_at` |
| Newly created accounts | `TestMigration0060_Success`: created by the application, or by a previous-release replica during a rolling deploy |
| Duration | table above |
| Lock impact | `TestMigration0060_LockImpact`: ACCESS EXCLUSIVE held until commit, and a plain read waits |
| Indexes | `TestMigration0060_Success` / `_Unauthorized`: unique index on `mfa_policies.tenant_id`, second policy refused |
| Rollback / recovery | `TestMigration0060_Rollback`, `_Idempotent`, rehearsal with the `v1.1.0-rc.3` binary |
| Interrupted migration | `TestMigration0060_InterruptedBackfill`, `_LockTimeout` |

The tests live in `backend/cmd/server/migration_0060_integration_test.go` and run
in the CI "Backend Integration Tests" job. To run them locally:

```bash
cd backend && DATABASE_URL=postgres://user:pass@localhost:5432/db?sslmode=disable \
  go test -tags=integration -run TestMigration0060 ./cmd/server/
# more volume:
OPENRISK_MIGRATION_TEST_ROWS=1000000 go test -tags=integration -timeout 30m -run TestMigration0060_Success ./cmd/server/
```
