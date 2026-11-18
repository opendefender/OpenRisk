---
name: control-status-engine-posture
description: #555 control status engine (ADR 0012, D-119) review 2026-11-18 PASS — worker Claim cross-tenant read, stale marks, control_status_evaluations cache
metadata:
  type: project
---

#555 reviewed at e4c8d176 on 2026-11-18: SECURITY: PASS, 0 blocking.

- Store.Claim (infrastructure/controlstatus/store.go) is the one tenantguard.CrossTenant read; returns only (tenant_id, control_id) and the cold fill takes the tenant from the `controls` row itself. Loader/Save/Delete all bind the claimed tenant.
- Stale marks (mark.go) bind the tenant on every table, called inside the write tx, before deletes. Tenant always from caller/DB-loaded row.
- GET /compliance/internal-controls/:controlId/status is computed live, never reads the cache; cross-tenant -> 404 same body.
- The repo has NO Postgres RLS (no ENABLE ROW LEVEL SECURITY in migrations) — don't ask for RLS parity on new tables.
- Postgres-only tests (cross-tenant loader, SKIP LOCKED claim) SKIP locally; confirm CI runs them.

Open LOW for #1465 (score readers joining the cache): no per-tenant fairness in the claim; one tenant's backlog can starve others' cache freshness. Matters once the cache is user-facing.

**How to apply:** when #1465 lands, re-check the cache is still never trusted for a security decision and revisit fairness. Related: [[recurring-defect-areas]], [[canonical-model-adr-posture]].
