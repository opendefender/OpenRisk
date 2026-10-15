# 0005 — The router leaves main(), and every route says who may call it

Status: **accepted**
Proposed: 2026-10-14, on #992
Accepted: 2026-10-15 by tech-lead review, on #992
Gate for: #991 (stays `status:needs-refinement` until this ADR is accepted)
Spike: branch `spike/992-rbac-roles-router` (not for merge), commits `c67761dd`, `6d50bc08`
Related: PR #987 (#985), #575, #529, #336

## Context

An auditor asks "who is allowed to delete a risk?" and the only honest answer today is to read
`backend/cmd/server/main.go`. Every test that talks about route guards reads source text. None of
them builds the router, because the router is assembled inside `func main()`
(`main.go:119-3354` on `origin/master` at `260b4d98`), between the database connection and the
graceful shutdown.

The facts below were counted on that commit, with a `go/ast` walk of every
`<router>.<Method>("<literal path>", …)` call rather than with grep. Where they differ from the
issue's first count, the AST count is the one this ADR uses.

**1. 460 routes, in four places.**

| Registered on | Routes | Notes |
|---|---:|---|
| `protected` | 368 | |
| `api` | 25 | 15 before the auth gate, 10 after it |
| `app` | 13 | `/metrics` plus 12 paths written out in full as `/api/v1/...` |
| six `protected.Group(...)` | 54 | score-engine 8, incidents 16, notifications 8, rbac/users 7, rbac/roles 8, rbac/tenants 7 |

The seventh group is `api` itself (`app.Group("/api/v1")`, `main.go:495`).

**2. `protected` and `api` are the same router.** `protected := api.Use(middleware.Protected(...))`
(`main.go:973`) returns the group it was called on. The variable name says nothing about whether
a route is authenticated. What decides it is the route's **position in Fiber's stack** relative to
that `Use`. So `api.Get("/stats/risk-matrix", …)` at `main.go:2072` is authenticated, because it is
registered after line 973, and `app.Post("/api/v1/scanner/agents/register", …)` at `main.go:898` is
public, because it is registered before. The existing extractor already documents this
(`internal/security/routes/extract.go`, the `case "Use"` comment).

**3. Registration order matters for security.** Fiber walks its stack in registration order. The
five `protected.Use(...)` calls (`main.go:979-1035`: cache invalidation, tenant quota, MFA policy,
actor stamp, mutation audit) apply only to routes matched after them. Static paths must come before
`:param` siblings (`/risks/unmapped` before `/risks/:id`, `main.go:1289`). Any extraction that
reorders mounts can change behaviour without changing a single guard.

**4. The guard is always the first handler.** Over the 460 mounts, the handler chain after the path
has one of six shapes: `h` 144, `hh` 17, `hhh` 1 (no permission or role guard), `Gh` 266, `GFh` 30
(guard, then a plan feature gate), `GGh` 2 (two guards, both required, e.g. `main.go:1719`). No
route puts a permission or role guard after another handler. 298 routes carry a route-level guard,
and 15 more inherit `adminRole` from the `/rbac/users` and `/rbac/roles` groups.

**5. Construction and mounting are interleaved.** About 114 of the 123 `database.DB` references sit
inside the route section. No function receives `app`, `api` or `protected` as an argument: the router
objects never leave `main()`.

**6. Two routes exist only when Redis is configured** (`main.go:2895-2902`, `POST
/vulnerabilities/scan` and `GET /assets/:id/inventories`).

**7. Six test files read `main.go` as text**, and they would go blind, not red, if routes left it:
`internal/handler/authz_route_coverage_test.go` (regex ratchet, #336/#529),
`internal/handler/authz_matrix_test.go` (`TestAuthzMatrix_ExtractionFindsTheMountedGuards`),
`internal/handler/authz_new_guards_test.go`, `internal/handler/vendor_routes_test.go`,
`internal/security/isolation/coverage_test.go` through `internal/security/routes.Extract`, and
`internal/domain/permission_catalog_sync_test.go` (regex `RequirePermission\("…"\)` over `main.go`;
PR #987 replaces it with an AST walk of every backend file). The spike moved eight routes out of
`main.go` and `TestNoStaleDecisions` failed at once (`decision "/api/v1/rbac/roles" matches no live
route`). The worse half is silent: `TestEveryParameterisedRouteHasAnIsolationDecision` kept passing
because the moved routes had simply vanished from its view. The four `internal/handler` files are
internal tests (`package handler`), so they cannot import a package that itself imports
`internal/handler`. The domain test is in `package domain`, which `internal/handler` imports, so the
same cycle applies to it.

**8. A guard closure cannot be identified from the router.** `app.GetRoutes()` returns
`[]fiber.Handler`. Built at one call site, `RequirePermission("risks:read")` and
`RequirePermission("risks:delete")` return closures with the same `reflect.Value.Pointer()`
(`0xece660` for both, measured on the spike branch; the value changes per build). Built at two call
sites they happen to differ, because the compiler inlined the constructor; with inlining off
(`-gcflags=-l`) all four are equal. `Pointer()` returns the code pointer of the func literal, never
its captured arguments. Neither behaviour is a contract.

## Decisions

### (a) Split granularity — one `Register<Module>Routes` per module, called from one `Build`

**Chosen.** New package `internal/api/http/router` (next to `internal/api/http/handlers`, as the
Clean Architecture layout places Fiber code). One file per module, each exporting
`Register<Module>Routes(m Mounter, d <Module>Deps)`. C17 adds
`router.Build(app *fiber.App, d Deps) *Matrix`, which installs the global middleware and calls every
`Register…` in today's order. `main()` keeps building dependencies and calls `Build` once.

**Rejected.** A single `buildRouter(deps)` function: it moves 3,000 lines into one other function
and recreates the problem one directory down. Keeping registration in `cmd/server/*_wiring.go`:
`package main` cannot be imported by the tests in fact 7.

### (b) Dependency injection — `Deps` holds handlers, never repositories; tests pass stubs

**Chosen.** Each `<Module>Deps` struct holds what the mount lines reference and nothing else:
handler pointers (`*handlers.RBACRoleHandler`, where `handlers` is `internal/handler`, as `main.go`
aliases it), and `fiber.Handler` values for anything today
computed at mount time (`cacheableHandlers.CacheRiskListGET(riskHandler.GetRisks)`, `featFinancial`,
`authLimits.Login`, inline closures). The composition root keeps calling those wrappers, so their
behaviour does not move. Repositories, services, `database.DB` and Redis stay in `cmd/server`.

The 114 in-section `database.DB` uses are **not** hoisted in the move PRs. Each move PR replaces only
the mount statements of its range with one `router.Register…(…)` call, at the same line, with the
handlers already in scope. Construction stays where it is, in the same order, so no worker, backfill
or goroutine starts earlier or later. C17 then moves the other way: the `Register…` calls and the
`Use` statements go **down** into one `router.Build` call after the last construction statement.
That is a diff of tens of lines, and it is safe because no construction statement reads `app`, `api`
or `protected` (fact 5; between lines 495 and 3354 they appear only in mounts, `Use`, `Group`, then
`app.Listen` and `app.Shutdown` after the last mount). Nothing is served before `app.Listen`, so mounting later
changes nothing a caller sees. C18 gathers the construction into `cmd/server/<module>_wiring.go`
functions shaped like the existing `score_wiring.go`, as a pure statement move.

A test builds the routes with pointers to zero-value handlers (`&handlers.RBACRoleHandler{}`, as the
spike does; not nil pointers, because a method value with a value receiver on a nil pointer panics
at mount time), plus a stub `fiber.Handler` for every handler field. The matrix test calls nothing.
No Postgres, no Redis, no `DATABASE_URL`. The spike does exactly this.

Conditional mounts keep their condition in the register function (`if d.Inventory != nil`). The
matrix test passes a non-nil stub so the full surface is listed, and the row carries the condition.

**Rejected.** `Deps` holding repositories and services: every test would have to build real handlers,
and that drags the database back in. A DI framework (wire, fx, dig): a new external dependency with
nothing to gain at this size, and out of scope for this issue without a D-entry.

### (c) Guard inspection — a recording `Mounter`; evidence only, enforcement unchanged

**Chosen.** Routes are mounted through `router.Mounter`, which wraps a `fiber.Router` and records one
matrix row per mount, at the moment of the mount:

```go
rbacRoles := protected.Group("/rbac/roles", router.Role("admin"))
rbacRoles.Delete("/:role_id", router.SessionOnly, h.DeleteRole)
risks.Delete("/:id", router.Require(router.Permission("risks:delete")), h.DeleteRisk)
```

A `Check` is the argument list of one guard. `Check.handler()` returns
`middleware.RequirePermission(perms...)` or `middleware.RequireRole(roles...)`, the existing
functions, unchanged, called with the same arguments. The Mounter puts the guard handlers first in
the chain (fact 4 shows that is where every guard is today) and calls the same `fiber.Router` method
`main.go` calls (`Get` for `Get`, which also adds HEAD). Group checks go through
`fiber.Router.Group(prefix, handlers...)` as today. The record and the enforcement are built from the
same `Check` value in the same call, so they cannot disagree.

`router.Verify(app, matrix)` then compares the matrix with `app.GetRoutes(true)`: a route mounted
around the Mounter, or a row with no route, fails. HEAD routes count too: one per GET row, and any
other HEAD fails (the spike skips HEAD entirely, so a bare `Head(...)` would slip past it).
`router.SessionOnly` makes "being signed in is enough here" something a reviewer reads at the mount
site. It must be a non-nil sentinel, and the Mounter panics on a nil or empty `Guard`, so `nil` cannot
stand in for a choice (in the spike `SessionOnly` is a nil `Guard`, and `Get(path, nil, h)` compiles
to the same thing).

**This mechanism is evidence only.** Enforcement stays in `middleware.RequirePermission`,
`middleware.RequireRole`, `middleware.Protected` and the `Use` chain, byte for byte. The matrix
is never read at request time. Removing the matrix would change nothing a caller sees. That keeps
this ADR inside "refactor": no D-entry is needed (criterion 4), and devsecops confirms or blocks
that reading on #992 (criterion 3).

**Rejected.**
- A registry keyed by handler, filled by an instrumented `RequirePermission`: fact 8. Closure
  identity in Go is not stable, and making it work means `unsafe` reads of the func value.
- A declarative route table (`[]Route{Method, Path, Perms, Handler}`) mounted by a loop: the table
  becomes the thing that decides which guard runs, which is the auth redesign the issue puts out of
  scope. It also cannot express the 30 chains that carry a plan feature gate, or the rate limiters and
  cache decorators in front of 18 more handlers, without growing a second middleware language.
- Behavioural probing alone (call every route with a permissionless token and read the 403 body):
  it is kept as a cross-check (see (e)), but `RequireRole` does not say which role it wanted, and an
  unguarded route would run its real handler with stub dependencies.

**Not covered by the matrix, said so in the matrix file header.** Plan gates (`RequireFeature`,
`RequireCapacity`) answer "which tenant", not "who", and stay plain handlers. Checks inside a
handler or use case (bulk operations, tenant deletion) show as `session only`, with the
reason at the mount site, as #529 already requires. A group `Use` applies to every later route
whose path starts with its prefix (Fiber v2.52 matches by plain string prefix, so `/rbac/users`
also covers a later `/rbac/users-x`), so a route mounted outside the group under the same prefix is
guarded more strongly than its row says. The matrix can under-report a guard. It cannot report one that is not
there.

### (d) PR staging — 18 child PRs, contiguous ranges, never more than 46 routes moved at once

Line ranges are on `origin/master` at `260b4d98`. Every one of the 460 mounts falls in exactly one
range (counted by the AST walk: 368 `protected` + 54 group + 25 `api` + 13 `app`). A range moves as a
block, in place, so registration order is unchanged by construction.

| # | Child PR | `main.go` lines | Routes | Estimate |
|---|---|---|---:|---|
| C1 | Before snapshot and single route inventory (see (e)); no route moves | — | 0 (snapshots all 460) | M |
| C2 | `router` package (Mounter, Check, Verify) + RBAC groups `/rbac/users`, `/rbac/roles`, `/rbac/tenants` (the spike, made real) | 2441-2490 | 22 | S |
| C3 | Public surface: `/metrics`, status, health, auth, OAuth/SAML, MFA challenge, scanner agents, invitations, vendor public link, vuln webhook | 480-970 | 28 | S |
| C4 | Entitlements, billing, telemetry, MFA enrolment, sessions/tokens, `/auth/me`, ownership | 971-1200 | 26 | S |
| C5 | Risks, risk categories, mitigations | 1201-1402 | 36 | S |
| C6 | Compliance frameworks/controls/audits/remediations, evidence | 1403-1600 | 43 | M |
| C7 | Board reports, assets, asset dependencies, vendors, TPRM, attack surface, saved views | 1601-1850 | 46 | M |
| C8 | Vulnerabilities, connectors, SmartScore routes, scoring weights | 1851-2050 | 38 | S |
| C9 | AI advisor, legacy `/stats` + export on `api`, `/score-engine` group | 2051-2094 | 21 | S |
| C10 | Users, teams, integrations, audit logs, custom fields, bulk operations | 2095-2200 | 28 | S |
| C11 | Reports, `/incidents` group, `/notifications` group | 2201-2310 | 37 | S |
| C12 | Action center, risk timeline, entities, analytics, dashboards | 2311-2440 | 30 | S |
| C13 | Organization, members, invitations, `/rbac/business-roles` | 2491-2700 | 21 | S |
| C14 | Scanner, realtime, inventory (Redis-conditional), CTI, search, score | 2701-3020 | 30 | S |
| C15 | Activation, onboarding, posture, automation | 3021-3210 | 33 | S |
| C16 | Governance | 3211-3354 | 21 | S |
| C17 | `router.Build`: the `Register…` calls and the global and `Use` middleware move down into one `Build` call, in today's order, the middleware passed in as handlers `main()` has already constructed with today's arguments; access cross-check; retirements (see (e), #987, #575) | 449-1035 (middleware) | 0 | M |
| C18 | Construction gathered into `cmd/server/<module>_wiring.go`, one commit per module, statement moves only | 119-3354 (non-mount statements) | 0 | M |

C1 merges first. C2 merges second, because it creates the package. C3 to C16 touch disjoint line
ranges of `main.go` and none of them edits the golden file, so they can be reviewed in any order.
C17 merges after them, C18 last. C18 is split from C17 because gathering the construction moves
most of the remaining 2,800 lines of `main()`, the one-PR move this staging exists to avoid. The
golden file shows it moved no route; the review checks that construction keeps its order (workers
and backfills start in the same sequence). If C18 is still too large to review, it splits per
module, in the C3 to C16 order. A child that finds its range has drifted on master re-derives it
from the golden rows, not from these line numbers.

Rules every move PR follows: no permission, role, group or path changes. No reordering. The golden
file is not touched, and the inventory test passing against it is the proof. One
`Register…Routes` per PR with a `TestXxxRoutes_Matrix` that prints its rows. `go test ./... -count=1`
from `backend/`, not a package subset.

**Rejected.** Moving by route group across the file (all `risks` routes in one PR): risks routes
sit in five non-adjacent places (`1277`, `1386`, `1920`, `2272`, `2325`), and gathering them reorders
the stack. Fewer, larger PRs: the largest range already holds 46 routes. The issue's ceiling is
"small enough to review", and a reviewer has to check each row against its old line.

### (e) The "before" matrix — an AST snapshot of today's `main.go`, frozen in C1

**Chosen.** C1 extends `internal/security/routes` (it already resolves group prefixes) so that, for
each mount, it also resolves:
- the route-level guards: inline `middleware.RequirePermission/RequireRole(...)` and guard variables
  (`riskDelete := middleware.RequirePermission("risks:delete")`, 57 of them);
- the group-level guards (`protected.Group("/rbac/users", adminRole)`);
- access, from the route's position relative to the `api.Use(middleware.Protected(...))` statement
  and its `/api/v1` prefix (a route after that statement but outside `/api/v1` is still public);
- the `if` condition around a mount, when there is one.

The walk fails, rather than skips, on any mount whose path it cannot resolve to a literal and on any
mount verb other than `Get/Post/Put/Patch/Delete` (none exist today, so this costs nothing).

It writes `internal/security/routes/testdata/route_matrix.golden`, one row per route in registration
order (`METHOD path access → checks`), plus one marker row per `Use`. A new test,
`TestRouteMatrix_MatchesGolden`, regenerates it and fails on any difference. C1 changes no
production code, so the golden file **is** today's matrix, reviewable as plain text.

C1 also creates one function, `routes.Inventory()`, and moves five of the six readers of fact 7
onto it. The sixth, the domain catalog test, cannot import it (fact 7) and stays a source scan; it is
handled under PR #987 below.
From C2 onward, `Inventory()` reads `main.go` with the AST walk, and wherever it meets a
`router.Register…Routes(...)` call it splices in the rows that function records when it is built
with stub `Deps`, at that position. Three rules keep the splice honest. A `router.Register…` call
`Inventory()` has no builder for fails the walk; it is never skipped. The prefix and access passed to
`router.New(...)` are literals, and `Inventory()` fails when the access the call claims differs from
the access its AST position gives. A conditional mount records its condition through the Mounter
(the spike has no such field yet; C14 adds it for the two Redis routes), so the golden row still
matches. Each move PR therefore compares old and new routes in the same
test, against a golden file it does not edit. #991 criterion 6 (identical matrix, diff pasted)
holds from the first move, and the isolation gate never goes blind. The route-reading parts of
the four `package handler` tests move to an external test package, to avoid the import cycle in
fact 7.

C17 adds the behavioural cross-check for the access column: build the whole router with the real
`middleware.Protected` and a throwaway RSA key, send one anonymous request per `authenticated` row,
expect 401 every time. Then the AST half of `Inventory()` is deleted, because nothing is left in
`main.go` to read.

**Rejected.** Capturing "before" from the built router: no test can build it before the extraction.
That is the whole problem. Capturing it by hand: not reviewable, and wrong as soon as master moves.
Capturing it as a runtime probe of the real server: needs Postgres and Redis, and `RequireRole` does
not say which role it wanted.

### (f) Public routes — listed, explicitly, as `public`

**Chosen.** The matrix lists all 460 routes, with access `public` or `authenticated`. The 28
public routes (15 on `api` before line 973, 13 on `app`) appear with no checks, printed `none` and
never `session only` (the spike's `Row.String` prints `session only` for any empty guard, so C3 fixes
it), and the golden file
carries them like any other row, so adding a public route is a visible diff. Access comes from stack
position (fact 2), never from the variable name. A test pins the public set to a reviewed allowlist,
the same ratchet #529 applies to session-only routes.

**Rejected.** Listing `protected` only: the auditor sees 432 of 460 routes and has to trust that the
other 28 were meant to be open. The issue asks for the whole surface.

## PR #987 and #575

- **PR #987 (#985) merges as is**, after its base #984. It is test-only, catches unresolvable
  `RequirePermission` arguments in every package (not only `main.go`). It does conflict with C2, and
  C2 resolves that. As written, #987 fails on the spike's `middleware.RequirePermission(c.Permissions...)`
  inside `Check.handler` (spread argument, and its forwarder exemption only covers functions, not
  methods). It also cannot see `router.Permission("…")`, so each moved guard drops out of the catalog
  check, `minGateCalls = 80` trips once enough guards move, and the realtime-stream check fails when
  `/realtime/events` moves in C14. So C2 adds `Permission` to its gate names and exempts
  `Check.handler` as a forwarder, in the same PR. If #987 has not merged by C1, the regex version
  goes blind the same way and C1 does the same work on it. C17 reduces it to a sanity check under #991 criterion 7: "every permission in the built-router
  matrix is in `PermissionCatalog`" replaces its route half. Its scan of `RequirePermission` and
  `PermissionGuard` calls outside the router stays, because the matrix cannot see those.
- **#575 is absorbed by C1.** C1's extractor resolves group prefixes and group guards, and
  `TestProtectedRoutes_UnguardedSetHasNotGrown` reads `Inventory()`, so the 54 group routes become
  visible to the ratchet. #575's criterion 4 (classify each newly visible unguarded route) is done in
  C1, and #575 closes with C1's PR.

## Invariants for #991

- Tenant isolation: no query is written or edited. Repository construction does not move until
  C18, and then only as a statement move. Every `tenant_id` filter is where it was.
- `PermissionCatalog`, roles, presets, the Score Engine and SmartScore are untouched.
- No new external dependency.
- The matrix test of #991 carries `TestRouteMatrix_Success` (the built matrix equals the golden),
  `TestRouteMatrix_NotFound` (a recorded row with no route fails `Verify`) and
  `TestRouteMatrix_Unauthorized` (a member without the permission or role gets 403 on a sampled row
  of each check kind, and an anonymous caller gets 401 on every `authenticated` row).

The devsecops review on #992 confirmed that (c) leaves enforcement unchanged, with no CRITICAL or
HIGH finding. It holds only if the children also do the following, which po-openrisk copies into
their acceptance criteria:
- C1 adds a route-count floor on `Inventory()` (460 today), so the readers in fact 7 fail if
  routes vanish from their view instead of passing in silence.
- C2: a malformed `Check` panics at mount time. That covers an empty `Check{}`, which today denies
  everyone but would print `perm()`, and a `Check` with both permissions and roles set, of which
  only the role would be enforced.
- C2 adds an AST test that refuses raw `middleware.RequirePermission/RequireRole` calls and guard
  variables in a Mounter handler list. Such a guard would be enforced and recorded as
  `session only`.
- C2 adds an import test: no package under `internal/middleware`, `internal/handler` or
  `internal/application` imports `router`. If the matrix is ever read to decide access, that is an
  auth redesign and needs a D-entry.
- C17 passes in the middleware values `main()` builds today, with the same configuration, and
  `Verify` on the full app compares the `Use` registrations against the golden marker rows.
- In production, `Build` refuses to start when a `Deps` field is nil, except for the fields that
  are conditional by design (the Redis-gated inventory handler).

## Consequences

Positive: the auditor's question is answered by a text file a test keeps true. A new route without
an explicit guard choice does not compile (`Get(path, Guard, …)` takes the guard first), and a `nil`
guard panics at mount time (see (c)), so it fails the first test that builds the router. A route
mounted around the Mounter fails `Verify`. `main()` shrinks to construction plus one `Build` call.

Negative: two ways to mount coexist until C17, and `Inventory()` is temporarily half AST, half
built. The `Check` type adds one indirection between the mount line and the middleware call. A
reviewer has to know that `router.Permission` *is* `middleware.RequirePermission`.

Neutral: plan gates and in-handler checks stay outside the matrix, as they are outside every test
today.

## Spike

Branch `spike/992-rbac-roles-router`, not for merge. It adds `internal/api/http/router/mount.go`
(Mounter, Check, Matrix, Verify), `rbac_roles.go` (the eight `/rbac/roles` routes) and
`rbac_roles_test.go`, and replaces the eight mounts in `main.go` with one call. `go build ./...`
passes. Run with no `DATABASE_URL` and no Redis:

```
$ env -u DATABASE_URL -u REDIS_URL go test -count=1 -v ./internal/api/http/router/
=== RUN   TestRBACRoleRoutes_MatrixIsReadOffTheBuiltRouter
    rbac_roles_test.go:50: GET    /api/v1/rbac/roles                       authenticated → role(admin)
    rbac_roles_test.go:50: POST   /api/v1/rbac/roles                       authenticated → role(admin)
    rbac_roles_test.go:50: GET    /api/v1/rbac/roles/:role_id              authenticated → role(admin)
    rbac_roles_test.go:50: PATCH  /api/v1/rbac/roles/:role_id              authenticated → role(admin)
    rbac_roles_test.go:50: DELETE /api/v1/rbac/roles/:role_id              authenticated → role(admin)
    rbac_roles_test.go:50: GET    /api/v1/rbac/roles/:role_id/permissions  authenticated → role(admin)
    rbac_roles_test.go:50: POST   /api/v1/rbac/roles/:role_id/permissions  authenticated → role(admin)
    rbac_roles_test.go:50: DELETE /api/v1/rbac/roles/:role_id/permissions  authenticated → role(admin)
--- PASS: TestRBACRoleRoutes_MatrixIsReadOffTheBuiltRouter (0.00s)
=== RUN   TestRBACRoleRoutes_MemberIsRefusedOnEveryRow
--- PASS: TestRBACRoleRoutes_MemberIsRefusedOnEveryRow (0.00s)
=== RUN   TestVerify_RejectsARouteMountedAroundTheMounter
--- PASS: TestVerify_RejectsARouteMountedAroundTheMounter (0.00s)
PASS
ok  	github.com/opendefender/openrisk/internal/api/http/router	0.020s
```

On the same branch `go test ./internal/security/...` fails `TestNoStaleDecisions`, which is fact 7
and the reason C1 comes first.

## Child issues for #991

po-openrisk creates these from the table in (d), one issue each, each with `Part of #991`, labels
`type:debt`, `area:backend`, `priority:P2-medium`, `tier:0-trust`, milestone `trust-v1`, plus
`type:security` on C1 and C17.

| Child | Title | Routes | Estimate |
|---|---|---:|---|
| C1 | test(authz): freeze today's route matrix from main.go and read routes from one inventory | 0 moved, 460 snapshotted | M |
| C2 | refactor(router): add the recording Mounter and move the RBAC groups | 22 | S |
| C3 | refactor(router): move the public surface | 28 | S |
| C4 | refactor(router): move entitlements, billing, telemetry and session routes | 26 | S |
| C5 | refactor(router): move risks and mitigations | 36 | S |
| C6 | refactor(router): move compliance and evidence | 43 | M |
| C7 | refactor(router): move board reports, assets, vendors, attack surface, saved views | 46 | M |
| C8 | refactor(router): move vulnerabilities and SmartScore | 38 | S |
| C9 | refactor(router): move AI, legacy stats and the score-engine group | 21 | S |
| C10 | refactor(router): move users, teams, custom fields, bulk operations | 28 | S |
| C11 | refactor(router): move reports, incidents, notifications | 37 | S |
| C12 | refactor(router): move action center, timeline, entities, analytics, dashboards | 30 | S |
| C13 | refactor(router): move organization and members | 21 | S |
| C14 | refactor(router): move scanner, realtime, CTI, search, score | 30 | S |
| C15 | refactor(router): move onboarding and automation | 33 | S |
| C16 | refactor(router): move governance | 21 | S |
| C17 | refactor(router): build the router in one call and check access behaviourally | 0 | M |
| C18 | refactor(server): gather dependency construction into per-module wiring files | 0 | M |

Total: 460 routes, each assigned once (C2 to C16 sum to 460; checked by bucketing the AST walk's
mount lines into the (d) ranges on `260b4d98`, none unassigned).
