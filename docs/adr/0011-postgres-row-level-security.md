# 0011 — Postgres row-level security: evaluated, not implemented

Status: **proposed**
Proposed: 2026-11-13, on #808 (criterion 5)
Builds on: D-055 (owner, 2026-09-28, option C), D-110 (the tenant guard)
Not implemented. D-055 decided a GORM callback now and RLS after the launch behind its own
ADR. This is that ADR, written so the post-launch decision starts from an evaluation and not
from a blank page. It is not reopened here.

## Context

Tenant isolation today rests on two layers. Every repository query carries `tenant_id` (ABSOLUTE
RULE 2), enforced by review. Since #808 a GORM callback, `internal/infrastructure/tenantguard`,
fails any SELECT, UPDATE or DELETE on a tenant-scoped table whose WHERE has no `tenant_id` or
`organization_id`; it runs on the whole test suite, in the CI boot, and in staging.

What the guard does not cover (D-110, package doc):
- it only sees queries that run, so its coverage is the test suite's plus staging's;
- it catches a missing filter, not a wrong one (the #807 class): a filter on a joined table
  satisfies it, so does one on the wrong tenant;
- raw SQL is checked lexically, after comments are removed and string literals blanked: a
  predicate in a CTE or a comma join, a `MERGE`, or a join glue `ON a.tenant_id = b.tenant_id`
  with no bound value, passes; an OR inside one raw string is not parsed; the SELECT part of an
  `INSERT ... SELECT` is checked like a read;
- on the builder side, a right-hand side the guard does not read is accepted as a filter
  (`tenant_id IN (SELECT tenant_id ...)`, `= (SELECT ...)`, `= COALESCE(tenant_id, ?)`, and a
  column compared with another column); only a tenant column compared with itself is refused;
- a scoped table reached only through `Joins` from an unscoped base table is not checked;
- the precedence of `.Or()` is approximated (no production query uses it today);
- a preload, and any statement reusing the parent's context while it runs (a query from an
  `AfterFind` hook included), inherits the parent's pass without being checked;
- INSERT, including `ON CONFLICT DO UPDATE`, is not checked;
- it is off in production unless `TENANT_GUARD` is set.

Inventory of what an RLS policy would have to cover: about 90 `TenantID` fields and 11
`OrganizationID` fields in the domain models (93 and 11 on 2026-11-13, from
`grep -rnE '^\s+TenantID\s' backend/internal/domain --include=*.go | grep -v _test | wc -l`,
and the same with `OrganizationID`; some are not models); tables gated only through a parent
(`team_members`, `mitigation_subactions`, `risk_histories`, `risk_assets`, and others); and the
global tables that must stay outside it (`users`, `organizations`, `roles`, `permissions`).

The question a regulated buyer's security review asks is: "if a developer writes a query with no
tenant filter, what stops it?" The guard answers it for the paths that are tested. Only the
database can answer it for every path.

## Decision (proposed, after the launch)

Enable and force row-level security on every tenant-scoped table.

- `ALTER TABLE ... ENABLE ROW LEVEL SECURITY` and `FORCE ROW LEVEL SECURITY`, with the policy
  `USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)` and the same
  `WITH CHECK`. `NULLIF` matters: once `SET LOCAL` has run on a pooled connection,
  `current_setting` answers an empty string, not NULL, and `''::uuid` raises an error instead of
  matching nothing. Tables keyed by `organization_id` use that column.
- The application connects as a non-owner role without `BYPASSRLS`.
- The tenant is set with `SET LOCAL app.tenant_id` inside a per-request transaction, from a GORM
  `ConnPool` / `BeginTx` wrapper. A plain `SET` is rejected: it leaks across pooled connections.
- Workers and backfills use a separate `openrisk_system` role with `BYPASSRLS`. The guard's
  `CrossTenant` allowlist (`grep -rn 'tenantguard.CrossTenant' backend`) is a starting
  inventory of what needs it, not a one-to-one map: about twenty of its sites are on the
  request path (pre-session token lookups, a user's own sessions across organisations), where
  the system role cannot be used and a narrower policy is needed (for instance a lookup
  function owned by the system role, called with the token hash).
- Migrations run as the table owner.
- Parent-gated tables get either `EXISTS` policies through the parent, or a denormalised
  `tenant_id` column added in two releases (expand, then contract), per CLAUDE.md.

## Consequences

- Every tenant-scoped request needs a transaction. Today there is one per use case, not one per
  request. A middleware has to open it.
- Connection pinning and pgbouncer in transaction mode are compatible with `SET LOCAL`;
  session mode is not needed.
- The sqlite tests cannot exercise RLS. A Postgres-only isolation suite is required, run in the
  CI integration job.
- A policy predicate costs something on hot list queries. The `tenant_id` indexes exist already;
  the cost has to be measured, not assumed.
- A forgotten filter becomes an empty result, not an error. That is safer and harder to debug:
  the guard keeps its value in tests and staging because it names the offending caller.
- It does not fix a wrong filter on the right tenant (#807 class): RLS bounds the tenant, not
  the predicate inside it.

## Alternatives rejected

- **Schema per tenant**: migration fan-out and a connection pool per schema.
- **Database per tenant**: cost and operations.
- **Views with security barriers**: every query is rewritten.
- **The guard alone**: only catches missing filters, and only on executed paths. Kept as the
  first layer, not as the last.

## Prerequisites before this can be accepted

1. A per-request transaction middleware.
2. A system role in the Helm chart and in the selfhost compose.
3. An inventory of the parent-gated tables with the chosen policy for each.
4. A benchmark on the list endpoints, with and without the policy.
5. A Postgres isolation suite that runs in CI.
