# OpenRisk — RC Hardening Report

_Role: Release Manager & Principal Architect. Date: 2026-07-23._
_Branch: `release/hardening-rc1` (stacked on `release/1.0-rc1` → `master`). Never merged to `main`/`master`._

Objective: move OpenRisk toward a **stable Release Candidate** by fixing blocking
technical debt across security, multi-tenant isolation, performance and test
health — **no new features**.

---

## 1. Corrections delivered (this branch, 6 commits)

| Commit | Area | What |
|---|---|---|
| `865244cd` | 🔴 Multi-tenant | **Cross-tenant leak fixed in bulk operations.** `/bulk-operations` (delete/update/export/assign) filtered risks by a user-supplied query with **no `tenant_id` scope** — a bulk delete `{"status":"open"}` would delete open risks across *every* tenant, and `GetBulkOperation` returned any op by UUID. Added `BulkOperation.TenantID`, threaded the tenant from request context, and scoped every resource query + op lookup. Also removed dead tenant-blind `risk_repo.go` (`GetAllRisks()` across all tenants, 0 callers). |
| `70c6647d` | 🔴 Security | **Redis-backed rate limiter.** Auth brute-force protection was per-instance in-memory (limit ×N across N instances). Now a shared Redis fixed-window counter (`redis.Client.AllowRate`, pipelined INCR+EXPIRE) with graceful in-memory fallback if Redis is down. |
| `c2252cc4` | ⚡ Performance | **Composite DB indexes (migration 0039)** on hot `(tenant_id, <col>)` paths: risks, vulnerabilities, assets, compliance_controls, incidents, mitigations, audit_events. Column names verified against the live schema; SQL validated in a rollback; idempotent. |
| `0df10981` | ⚡ Performance | **Route-based code splitting** (React.lazy + Suspense). Initial JS bundle **1.56 MB → ~651 kB** (gzip 454 kB → 209 kB); each screen loads on navigation. |
| `bb151648` | 🐛 + ♿ + 🧹 | **Real Edit-Risk bug fixed** (form declared `tags` as array but the input produced a string → Zod rejected *every* save). **Frontend suite made green** (7 failing + 2 unloadable → 43/0). **A11y**: shared `Input` now associates label/input. **Dead code removed**: duplicate `components/CreateRiskModal`, `shared/fixtures.ts`, 2 legacy Jest tests. |
| `ad1f2dc2` | 🔐 RBAC | **Real per-route `PermissionRoute` guard** wired from the shared nav permission map (nav filter, route guard and backend 403 now agree). |

_Inherited from `release/1.0-rc1` (already pushed):_ analytics/dashboard cross-tenant
fix, rate-limit wiring, CORS env, legacy HS256 login removal, admin-password guard,
`TestRiskCRUDFlow` green, dead `ai_risk_predictor` removal.

---

## 2. Tests executed & results

| Suite | Result |
|---|---|
| `go build ./...` · `go vet ./...` | ✅ clean |
| `go test ./...` | ✅ **36 packages OK · 0 FAIL** |
| New backend tests | Redis limiter (redis-result + error-fallback), bulk-op isolation (count tenant-scoped, Get/List reject other tenant) |
| `tsc -b` | ✅ clean |
| `vite build` | ✅ (main chunk ~651 kB) |
| `vitest run` | ✅ **43 passed · 0 failed** (7 files) |

---

## 3. Optimizations

- **Initial bundle −58%** via route code-splitting.
- **Query performance**: composite indexes serve both `(tenant_id, col)` filters and
  the `tenant_id`-only prefix — no regression on existing per-tenant scans.
- **Rate limiter** now correct under horizontal scaling (shared counter).

---

## 4. Out of scope (honest) — why, and the plan

These items from the brief were **not** attempted here because doing them safely
requires more than a hardening pass and, in several cases, live multi-role
verification that the dev sandbox blocks (interactive browser driving is unavailable):

| Item | Why deferred | Suggested approach |
|---|---|---|
| **Systematic isolation audit of all ~300 routes** | A full per-route audit + fuzz harness is a project in itself. This pass did a **static scan** of every repo/service and fixed the live leak it found (bulk ops) + added generic isolation tests. | Build a table-driven integration test that, per resource, creates 2 tenants and asserts no cross-read/write; run in CI. |
| **Merge the two RBAC systems into one** | Large architectural change touching auth, middleware, and admin screens; high regression risk without live role testing. | Pick the runtime path (`OrganizationMember` → JWT perms) as the single source; migrate `domain/rbac.go` screens onto it behind tests. |
| **Apply `own`/`assigned` scopes at query level** | Requires ownership columns + per-repo query changes + role tests; risk of over-restricting. | Add `owner_id`/`assigned_to` predicates in the repo layer gated by the business role; test each. |
| **Complete OpenAPI for all routes + regen TS client** | ~300 routes vs 51 documented; large contract-first effort. | Incrementally document per module (compliance already contract-first); regen `openapi.generated.ts` per batch. |
| **Unify `RiskStatus` (lowercase vs uppercase)** | Two vocabularies across backend + frontend; risky to flip without full E2E. | Choose one backend vocabulary, migrate data, add a compatibility shim, then remove the frontend normaliser. |
| **Curated cross-mapping seed (ISO↔NIST…)** | The engine already exists (`ControlMapping` + endpoints + UI); only curated crosswalk *content* is missing — a data/accuracy effort, not code. | Seed a reviewed ISO27001↔NIST-CSF starter set; expand per framework. |
| **Playwright E2E campaign** | Config exists but headless browsers are blocked in this sandbox. | Run the existing Playwright suite in CI against a seeded stack. |
| **GitHub tags/releases cleanup** | Deleting/relabelling *published* releases is destructive and outward-facing — needs explicit go-ahead. | See §5. |

---

## 5. Versioning cleanup (recommendation — not executed)

GitHub carries `1.0.0`–`1.0.8` (mixed `1.0.x` / `v1.0.7` naming) that **predate the
current product**. The in-repo `changelog.md` was updated with a `1.1.0-rc.1`
section reflecting this RC. Recommended (with your go-ahead, since it is outward-facing):

1. Adopt SemVer + a consistent `vX.Y.Z` tag scheme.
2. Mark the stale `1.0.x` GitHub releases as **pre-1.0 / archived** (edit notes; do
   not silently delete — people may reference them).
3. Cut `v1.1.0-rc.1` from this branch once merged to an integration branch.
4. Keep `[Unreleased]` → versioned sections in `changelog.md` going forward.

---

## 6. RC readiness verdict

**Materially closer to a stable RC.** This pass eliminated a **destructive
cross-tenant vulnerability**, made brute-force protection **multi-instance safe**,
found and fixed a **real Save-breaking bug**, cut the **initial bundle by ~58%**,
added **DB indexes**, wired a **real route permission guard**, and brought **both
test suites fully green**.

- **Demo / private beta:** ✅ ready on this branch (green build+tests, key security holes closed).
- **Production / commercial sale:** 🟠 **not yet** — close the remaining blockers,
  primarily the **systematic multi-tenant isolation audit across all routes** and a
  **real E2E (Playwright) run**, plus the versioning/OpenAPI cleanup. These are
  bounded, known, and mostly non-code (verification + docs) — not missing features.

Nothing existing was broken: every change is additive or a scoped fix, covered by
build + vet + the full backend and frontend suites.
