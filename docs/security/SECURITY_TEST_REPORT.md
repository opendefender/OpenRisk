# Security test report

| | |
|---|---|
| Date | 2026-11-26 |
| Base commit | `9f9ea53a` (develop), branch `security/1319-cross-tenant-suite` |
| Issue | #1319 (part of #229) |
| Threat model | [`THREAT_MODEL.md`](THREAT_MODEL.md) |
| Gap register | [`ASVS_GAP_REGISTER.md`](ASVS_GAP_REGISTER.md) |
| Sign-off | Requested from the owner: D-130 in [`docs/DECISIONS.md`](../DECISIONS.md) |

This report records what was run, how, and what it found. It is evidence for the
launch gate, not a penetration test: no one attacked the system by hand beyond
the automated probes described here. Findings that are tracked in private
security advisories are named by their advisory id only, as `SECURITY.md`
requires, until their fix is released.

## 1. Cross-tenant suite (live, Postgres)

**What it is.** `backend/internal/security/crosstenant/`. Two organisations
register through the real API (`POST /api/v1/auth/register`) on the real server
binary, booted on Postgres 16 and Redis 7 with `TENANT_GUARD=enforce`. Tenant B
gets one object of each seeded kind through the API, and every text field carries
a per-run canary. Tenant A's token is then pointed at B's ids.

**What counts as a pass** (D-129):
- A's request on B's id answers 404 (preferred) or 403.
- It answers the same status as for a random UUID on the same route, so no
  existence oracle.
- B's object reads the same (same body hash) before and after A's attempt.
- No A response contains a B id or B's canary.
- B's own token gets 2xx on the same route and body: the positive control. A
  failed control fails the probe as vacuous; it is never skipped.
- The boot log holds no `tenantguard` line.

Any 2xx, 5xx, 401 or 429 fails. A 400 or 422 fails unless that probe allows it,
with a reason and a row below. `TestHarness_DetectsLeak` feeds 18 known-bad
responses to the checker and requires each one to be rejected.

**Coverage gate.** A ratchet (`ratchet_test.go`, runs in unit CI) reads the route
table from `cmd/server/main.go` and fails when a route has neither a probe nor a
written exemption, when a group has no passing probe, when a probe names a route
that no longer exists, or when an exemption cites a register row that is not in
this file.

**Coverage** (505 routes in 60 groups, counted by the ratchet's own classifier):

| Route class | Total | Probed live | of which allowed (row) | Exempt (row) | Exempt, nothing to name | Auto-exempt¹ |
|---|---|---|---|---|---|---|
| Parameterised | 236 | 139 | 5 | 82 | 1 | 14 |
| Collection GET | 154 | 113 | 0 | 7 | 0 | 34 |
| Write without an id in the path | 115 | 20 | 10 | 20 | 66 | 9 |
| **Total** | **505** | **272** | **15** | **109** | **67** | **57** |

¹ The isolation registry records the route as public by design, machine-authenticated
or self-scoped.

The suite probes every parameterised and collection route except the 109
exemptions listed against a register row in
`backend/internal/security/crosstenant/exemptions.go` and the four groups
exempted as a whole (`ai`, `controls`, `control-exceptions`, `integrations`).
It does not prove "all routes".

**Run it**
```
make test-cross-tenant          # throwaway Postgres and Redis in Docker, then scripts/ci/cross-tenant.sh
```
CI runs it in the `cross-tenant-suite` job of `.github/workflows/ci.yml`. The job
is required by the summary job.

**Result on this branch.** Five consecutive local runs on a fresh database:
```
--- PASS: TestCrossTenant_Seeds (1.10s)
--- PASS: TestCrossTenant (3.81s)
ok  	github.com/opendefender/openrisk/internal/security/crosstenant	4.914s
```
The other four runs were identical. None logged a `tenantguard` line.

**The suite is not vacuous.** With the production fixes below stashed, the same
script exits 1. Ten probes fail, and the guard logs two refused queries.

### Findings of the suite

| Id | Finding | Severity | Status |
|---|---|---|---|
| F1 | Cross-tenant read of score aggregates | medium (advisory) | Fixed in #1319, GHSA-m6gp-q4m7-27c6 |
| F2 | Cross-tenant read through a stored foreign reference | medium (advisory) | Fixed in #1319, GHSA-3x9h-57v5-w5q3 |
| F3 | Cross-tenant e-mail disclosure by user id | medium (advisory) | Fixed in #1319, GHSA-39cg-g3rv-985p |
| F4 | Incident update and delete answered 500 and 204 for a foreign id; no data reached | low | Fixed in #1319: typed `ErrNotFound`, 404 |
| F5 | Five mitigation routes answered 400 with the typed error text for a foreign id; no data reached | low | Fixed in #1319: one 404 body |
| F6 | Cross-tenant read and write of scoring configurations | medium (advisory) | Fixed in #1319, GHSA-m6gp-q4m7-27c6 |
| F7 | Request strings outlived their request: Fiber returned views of the fasthttp buffer, so a value a handler kept could be overwritten by the next caller's request. Seen once as a canary echo; F6's store kept such a key | high (class) | Fixed in #1319: `Immutable: true` in `fiber.Config` (D-129) |

`SECURITY.md` treats any flaw that lets one tenant read or change another tenant's
data as critical, whatever its CVSS score. The severities above are the CVSS
ratings recorded in the advisories.

Functional defects found on the way, none of them a cross-tenant leak: #1169,
#1516, #1517, #1518 and #1519.

## 2. SAST

| | |
|---|---|
| Tool | gosec v2.29.0, the version pinned in `.github/workflows/security-scanning.yml` |
| Command | `cd backend && gosec -no-fail -quiet -fmt json -out gosec.json ./...` |
| Scope | 886 files, 174,456 lines, test files excluded (gosec default) |
| Result | 47 issues: 12 high, 14 medium, 21 low. 3 `#nosec` |

Triage of the high and medium issues, read in the code:

| Rule | Where | Verdict |
|---|---|---|
| G101 ×3 | a header name, the login timing dummy password, the dev scanner key | False positive. Each has a justified `//nolint:gosec`. The dev key is refused in production by the production-secrets gate (#1317) |
| G115 ×4 | thehive id, password hasher digest length, score history quarter | False positive: bounded values, annotated |
| G404 ×2 | `pkg/crq/montecarlo.go` | Accepted: seeded RNG for reproducible simulations, not security use |
| G703 | `demoseed.locateFixture` | Accepted: path from operator environment, demo seeding only |
| G118 ×2 | `trigger_scan.go:167`, `sync_engine.go:79` | Accepted: background jobs deliberately detached from the request context |
| G304 ×10 | `pkg/storage/local.go`, key loading, CLI tools | False positive: storage keys are server-generated and `resolveKey` rejects escapes; the others read operator-supplied paths |
| G302, G301 | storage file 0640, CTI data dir 0755 | Low: RR-SAST-1 |
| G401, G501 | md5 for cache keys | False positive, annotated |

The 21 low issues (G104 unchecked errors, G706 log injection on structured fields)
were not triaged one by one; see RR-SAST-2.

## 3. SCA

| Tool | Command | Result |
|---|---|---|
| govulncheck v1.10.0 | `cd backend && govulncheck ./...` | Exit 3. Two reachable standard-library vulnerabilities with go1.26.9 (`go.mod`): GO-2026-6859 (`net/http`, fixed in 1.26.11) and GO-2026-6809 (`crypto/x509`, fixed in 1.26.10). One in imported packages and two in required modules, not reachable |
| npm audit | `cd frontend && npm audit --omit=dev` | 0 vulnerabilities in production dependencies |
| npm audit | `cd frontend && npm audit` | 5 in development dependencies only (1 high `nanoid`, 4 moderate in vitest and `@humanfs/node`) |

The Dependency gate (`.github/workflows/dependency-gate.yml`, #1318) runs both
on every pull request. It has been red on develop since 2026-11-24 because of the
two Go advisories. Both are tracked in #1207.

## 4. DAST

See section 4 of the appendix below. It is filled in from the CI run on this
branch.

## 5. Residual-risk register

Every finding not fixed above, and every exemption or allowance in the suite.
"Owner" is the issue that closes the row. "Decision" is accept until that issue
lands, or the stated reason. The owner's acceptance of this register is D-130.

| Id | Risk | Severity | Owner | Decision |
|---|---|---|---|---|
| RR-01 | 64 routes and 4 groups have no seed or no successful control call yet (compliance sub-resources, vulnerabilities, notifications, automation runs, bulk operations, report jobs, control exceptions, integrations, scanner-adjacent kinds). Their isolation rests on the recorded decision in `isolation/registry.go`, not on a test | medium | #1521 | Accept until #1521 |
| RR-02 | Five routes answer 200 with an empty body for a foreign parent id instead of 404. No foreign data is returned, and a random id gets the same answer | low | #1520 | Accept until #1520 |
| RR-03 | 17 `/rbac` routes are unprobed: registration writes no `user_tenants` row, so a new root gets 403 there and no control can succeed | medium | #1518 | Accept until #1518 |
| RR-04 | 5 routes fail for every caller on Postgres, so no control exists (dashboard queries, `/audit-logs`, `/rbac/users/stats`) | medium | #1169, #1519 | Accept until fixed |
| RR-05 | 17 routes and the `ai` group need an artefact the harness does not create (uploaded file, rendered document, second approver, AI provider) | low | #1521 | Accept |
| RR-06 | 2 server-sent-event streams are not probed | low | #1521 | Accept |
| RR-08 | 1 route's own throttle makes repeated control runs answer 429 | low | #1521 | Accept |
| RR-09 | 2 dead incident-risk routes parse a UUID as an integer and answer 400 to everyone | low | #1519 | Accept until #1519 |
| RR-10 | 2 bulk preview routes answer 200 with "missing" for a foreign id. The body is the same as for a random id, and nothing is read or written | low | #1520 | Accept |
| RR-11 | 2 routes refuse foreign body ids with 400 rather than 404. The answer is the same for a random id, and nothing is written | low | #1520 | Accept |
| RR-12 | 3 writes store a foreign body-addressed reference as is (201). It is never read back, because reads filter by tenant | medium | #1520 | Accept until #1520 |
| RR-13 | Scoring config ids are chosen by the caller. A write, compute or classify naming another tenant's id acts on the caller's own copy or on the default. B's config was verified unchanged; the unit tests pin the rest | low | — | Accept: by design after F6 |
| RR-14 | Telemetry consent is one row for the whole deployment, and any tenant's admin can change it | medium | #1522 | Accept until #1522 |
| RR-SCA-1 | GO-2026-6859 and GO-2026-6809, reachable, in the Go standard library | high | #1207 | Fix: bump to go1.26.11 |
| RR-SCA-2 | 5 frontend development-only advisories (`nanoid`, vitest) | low | Dependabot | Accept: not shipped |
| RR-SAST-1 | Uploaded files are written 0640 and the CTI data directory 0755. Group and other read access on a shared host | low | #1521 | Accept: the image runs as one non-root user, `openrisk` (`Dockerfile:51`) |
| RR-SAST-2 | 21 low gosec issues not triaged one by one | low | #1521 | Accept |
| RR-SCOPE-1 | Not covered by the suite: cookie sessions (Bearer only), a principal in both tenants, the attacker's own object carrying B's id in the body, and the MFA enrolment branch (a new root is in its 7-day grace period) | medium | #1521 | Accept until #1521 |

## Appendix: how to reproduce

```
make test-cross-tenant
cd backend && go test ./internal/security/...                        # ratchet + harness checks
cd backend && gosec -no-fail -fmt json ./...                         # SAST
cd backend && govulncheck ./...                                      # SCA, Go
cd frontend && npm audit --omit=dev                                  # SCA, npm
gh workflow run dast.yml --ref <branch>                              # DAST, see .github/workflows/dast.yml
```
