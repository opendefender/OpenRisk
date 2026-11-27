# Security test report

| | |
|---|---|
| Date | 2026-11-26 |
| Base commit | `ae28a16f` (develop, merged into the branch on 2026-11-27), branch `security/1319-cross-tenant-suite` |
| Issue | #1319 (part of #229) |
| Threat model | [`THREAT_MODEL.md`](THREAT_MODEL.md) |
| Gap register | [`ASVS_GAP_REGISTER.md`](ASVS_GAP_REGISTER.md) |
| Sign-off | Owner, 2026-11-27, with conditions: D-130 in [`docs/DECISIONS.md`](../DECISIONS.md). Before launch: #1207, #1518, #1520, #1521 |

This report records what was run, how, and what it found. It is evidence for the
launch gate, not a penetration test: no one attacked the system by hand beyond
the automated probes described here. Findings that are tracked in private
security advisories are named by their advisory id only, as `SECURITY.md`
requires, until their fix is released.

## 1. Cross-tenant suite (live, Postgres)

**What it is.** `backend/internal/security/crosstenant/`. Two organisations
register through the real API (`POST /api/v1/auth/register`) on the real server
binary, booted on Postgres 16 and Redis 7 with `TENANT_GUARD=enforce` and
`APP_ENV=test` (the non-production error handler; the rate limits are raised for
this job only, D-129; the MFA policy is the production one). Tenant B
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
| Write without an id in the path | 115 | 20 | 10 | 21 | 65 | 9 |
| **Total** | **505** | **272** | **15** | **110** | **66** | **57** |

¹ The isolation registry records the route as public by design, machine-authenticated
or self-scoped.

The suite probes every parameterised and collection route except the 110
exemptions listed against a register row in
`backend/internal/security/crosstenant/exemptions.go` and the four groups
exempted as a whole (`ai`, `controls`, `control-exceptions`, `integrations`).
It does not prove "all routes".

**What a collection probe proves.** Of the 113 collection probes, 26 are lists of
a seeded kind and 87 are aggregates (dashboards, stats, counts). An aggregate probe
proves that B gets a 2xx and that A's body holds no B id and no B canary. It cannot
see a numeric leak: a count or an average that includes B's rows. F1 was such a
leak; the evaluator did not see it, and the tenant guard log did. For aggregates,
the guard of numeric leaks is that log (`scripts/ci/cross-tenant.sh` fails on any
`tenantguard` line) plus the unit tests of each aggregate, not the probe.

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
| F1 | Cross-tenant read of score aggregates | critical (policy) · CVSS medium | Fixed in #1319, GHSA-m6gp-q4m7-27c6 |
| F2 | Cross-tenant read through a stored foreign reference, on write and on read | critical (policy) · CVSS medium | Fixed in #1319, GHSA-3x9h-57v5-w5q3 |
| F3 | Cross-tenant e-mail disclosure by user id, through two fields | critical (policy) · CVSS medium | Fixed in #1319, GHSA-39cg-g3rv-985p |
| F4 | Incident update and delete answered 500 and 204 for a foreign id; no data reached | low | Fixed in #1319: typed `ErrNotFound`, 404 |
| F5 | Five mitigation routes answered 400 with the typed error text for a foreign id; no data reached | low | Fixed in #1319: one 404 body |
| F6 | Cross-tenant read and write of scoring configurations | critical (policy) · CVSS medium | Fixed in #1319, GHSA-m6gp-q4m7-27c6 |
| F7 | One run in eight echoed a B canary in A's response on `POST /score-engine/configs`. Not reproduced in 12 later runs, and no path from the config store to that response was traced. Leading hypothesis: fasthttp aliasing (Fiber returns views of the request buffer, and F6's store kept such a string as a map key) | high (unconfirmed) | Hardened in #1319: `Immutable: true` in `fiber.Config` makes Fiber's accessors (`c.Params`, `c.Query`, `c.Get`, `c.Body`, ...) return copies (D-129). Raw fasthttp calls are not covered; the two in the code copy or hash at once (`asset_handler.go:100`, `pkg/cache/decoration.go:118`). A test pins the setting |
| F8 | Found in security review, not by the suite. A workflow step's `approver_user_ids` are stored unchecked, and the approval notifier loaded those users across tenants: it e-mailed and notified another tenant's user an attacker-titled request | medium | Fixed in #1319: the notifier resolves only active members of the request's tenant, test `approval_notifier_tenant_test.go` |

`SECURITY.md` treats any flaw that lets one tenant read or change another tenant's
data as critical, whatever its CVSS score: F1, F2, F3 and F6 are critical by that
policy. The CVSS rating is the one recorded in the advisory.

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

| | |
|---|---|
| Tool | OWASP ZAP 2.17.0 baseline (passive), `.github/workflows/dast.yml`, gate `.zap/rules.tsv` (every passive rule is FAIL) |
| Stack | The real backend binary on Postgres 16 and Redis 7 in CI, ephemeral RS256 keys, `OPENRISK_DEPLOYMENT=self_hosted`, OpenAPI spec imported so every documented path is scanned |
| Run | `gh workflow run dast.yml --ref security/1319-cross-tenant-suite`: run 45823419745 on `53c5b0fb`, conclusion success |

| Scan | URLs scanned | Documented paths | Result |
|---|---|---|---|
| Signed out | 216 | 150 | `FAIL-NEW: 0 WARN-NEW: 0 INFO: 0 IGNORE: 0 PASS: 67` |
| Signed in (Bearer, GET only) | 136 | 88 | `FAIL-NEW: 0 WARN-NEW: 0 INFO: 0 IGNORE: 0 PASS: 67` |
| Cache-Control on signed-in GETs | 47 routes | | 43 answered 2xx, 0 cacheable |

No finding. Limits: the scan is passive (no active attack rules), and it sees only
the paths documented in `docs/openapi.yaml` (150 of them). See RR-DAST-1.

## 5. Residual-risk register

Every finding not fixed above, and every exemption or allowance in the suite.
"Owner" is the issue that closes the row. "Decision" is accept until that issue
lands, or the stated reason. The owner signed this register on 2026-11-27 (D-130), on condition that RR-SCA-1 (#1207), RR-03 (#1518), RR-12 (#1520) and #1521 close before the launch.

| Id | Risk | Severity | Owner | Decision |
|---|---|---|---|---|
| RR-01 | 64 routes and 4 groups have no seed or no successful control call yet (compliance sub-resources, vulnerabilities, notifications, automation runs, bulk operations, report jobs, control exceptions, integrations, scanner-adjacent kinds). Their isolation rests on the recorded decision in `isolation/registry.go`, not on a test | medium | #1521 | Accept until #1521 |
| RR-02 | Closed. Five routes used to answer 200 with an empty body for a foreign parent id instead of 404. Each now answers 404, with one body for a foreign and an unknown id (proof below) | low | #1520 | Closed by #1520 |
| RR-03 | 17 `/rbac` routes are unprobed: registration writes no `user_tenants` row, so a new root gets 403 there and no control can succeed | medium | #1518 | Accept until #1518 |
| RR-04 | 5 routes fail for every caller on Postgres, so no control exists (dashboard queries, `/audit-logs`, `/rbac/users/stats`) | medium | #1169, #1519 | Accept until fixed |
| RR-05 | 17 routes and the `ai` group need an artefact the harness does not create (uploaded file, rendered document, second approver, AI provider) | low | #1521 | Accept |
| RR-06 | 2 server-sent-event streams are not probed | low | #1521 | Accept |
| RR-08 | 1 route's own throttle makes repeated control runs answer 429 | low | #1521 | Accept |
| RR-09 | 2 dead incident-risk routes parse a UUID as an integer and answer 400 to everyone | low | #1519 | Accept until #1519 |
| RR-10 | 2 bulk preview routes answer 200 with "missing" for a foreign id. The body is the same as for a random id, and nothing is read or written | low | #1520 | Accept |
| RR-11 | 2 routes refuse foreign body ids with 400 rather than 404. The answer is the same for a random id, and nothing is written | low | #1520 | Accept |
| RR-12 | Closed. Four writes (six with `PUT /governance/workflows/:id` and `PATCH /compliance/audits/:id`) used to store a foreign body-addressed reference as is (201). Each now answers 404 for a foreign or unknown id and stores nothing (proof below) | medium | #1520 | Closed by #1520 |
| RR-13 | Scoring config ids are chosen by the caller. A write, compute or classify naming another tenant's id acts on the caller's own copy or on the default. B's config was verified unchanged; the unit tests pin the rest | low | devsecops (GHSA-m6gp-q4m7-27c6) | Accept: by design after F6. Revisit if configs move to the database |
| RR-14 | Telemetry consent is one row for the whole deployment, and any tenant's admin can change it | medium | #1522 | Accept until #1522 |
| RR-SCA-1 | GO-2026-6859 and GO-2026-6809, reachable, in the Go standard library | high | #1207 (widened to both on 2026-11-26) | Fix: bump to go1.26.11 |
| RR-SCA-2 | 5 frontend development-only advisories (`nanoid`, vitest) | low | Dependabot | Accept: not shipped |
| RR-SAST-1 | Uploaded files are written 0640 and the CTI data directory 0755. Group and other read access on a shared host | low | #1521 | Accept: the image runs as one non-root user, `openrisk` (`Dockerfile:51`) |
| RR-SAST-2 | 21 low gosec issues not triaged one by one | low | #1521 | Accept |
| RR-DAST-1 | DAST is passive only, and it covers only the routes documented in `docs/openapi.yaml`. No active scan (injection, fuzzing) was run against a live stack | medium | #1121 (documentation); active scan not planned | Accept for v1.0: the cross-tenant suite and the input-validation audit (#1316) cover the active classes that matter most |
| RR-SCOPE-1 | Not covered by the suite: cookie sessions (Bearer only), a principal in both tenants, the attacker's own object carrying B's id in the body, and the MFA enrolment branch (a new root is in its 7-day grace period) | medium | #1521 | Accept until #1521 |

### RR-02 and RR-12: closure proof (#1520)

Rule for every row: a foreign id and an unknown id give the same status and the
same body (`{"error":"<resource> not found"}`, the id is not echoed), and the
caller's own object keeps its previous answer. Each parent lookup is the
tenant-scoped finder (`tenant_id = ?` from the token), never a query by id alone.
The live suite no longer allows any of these answers: no `allow(` and no exemption
carries RR-02 or RR-12 (`backend/internal/security/crosstenant/`), so a 200 or 201
for a foreign id fails the run.

RR-12, body-addressed writes (404, nothing written):

| Route | Check | Unit tests |
|---|---|---|
| `POST /compliance/audits` (`framework_id`) | `complianceaudit.CreateAuditUseCase`, `frameworks.GetFrameworkByID(id, tenant)`; absent `framework_id` stays accepted | `create_audit_test.go`: `TestCreateAudit_Success`, `_NotFound_ForeignAndUnknownAlike`, `_Unauthorized` |
| `PATCH /compliance/audits/:id` (`framework_id` in the body) | `complianceaudit.UpdateAuditUseCase`, same `frameworks.GetFrameworkByID(id, tenant)` as the create path, before any field is assigned; an absent `framework_id` or an empty `framework_id` (clears it) needs no lookup | `update_audit_test.go`: `TestUpdateAudit_Success`, `_NotFound`, `_Unauthorized` |
| `POST /governance/approvals` (`entity_id`) | `governance.SubmitApprovalRequestUseCase`, `checkEntity`, through `approvalEntityOwnership` (`cmd/server/approval_entities.go`); empty `entity_id` stays accepted | `approval_ownership_test.go`: `TestSubmitApproval_OwnEntities_Success`, `_NotFound_ForeignAndUnknownAlike`, `_Unauthorized_NoTenantOrNoLookup` |
| `POST /incidents` (`risk_ids`) | `service.IncidentService.checkRiskIDs`: one query `tenant_id = ? AND id IN ?`, before `db.Create`; a non-UUID entry is 400 | `service/incident_risk_ownership_test.go`: `TestCreateIncident_RiskIDs_Success`, `_NotFound_ForeignAndUnknownAlike`, `_Unauthorized`; route: `handler/incident_risk_ids_test.go` |
| `POST /governance/workflows` (`steps[].approver_user_ids`) | `governance.checkApprovers`: `MemberLookup.GetMember(tenant, user)`, no member row is 404, a deactivated member is 400 | `approval_ownership_test.go`: `TestCreateWorkflow_Approvers_Success`, `_NotFound_ForeignAndUnknownAlike`, `_Unauthorized` |
| `PUT /governance/workflows/:id` (same field; added so a clean workflow cannot be edited to name a foreign approver) | same `checkApprovers`, only when the call supplies `steps` | `TestUpdateWorkflow_Approvers_Success`, `TestUpdateWorkflow_NotFound_ForeignAndUnknownApproverAlike`, `TestUpdateWorkflow_Unauthorized` |

Live suite probes: `probe_body.go` (audits, approvals, incidents), `probe_org.go`
(`POST /governance/workflows`, new). There is no live `PUT` approver probe: the probe
language puts B's id in every slot, so the workflow lookup (also B's id) answers 404 before
the approver is read. The unit tests above pin the `PUT` check.

`entity_id` of an approval request, per entity type (`domain/approval_engine.go`):

| Entity type | Validated | Reason |
|---|---|---|
| `risk_acceptance` | yes, against the tenant's risks (`RiskRepository.GetByID`) | maps to `risks`; the risk lifecycle reads the approval |
| `compliance_control` | yes, against the tenant's controls (`GetControlByID`) | maps to `compliance_controls` |
| `security_exception`, `policy`, `access_request`, `vendor` | no, kept as an opaque label | no owning table or domain entity exists for these types, and no code reads `ApprovalRequest.EntityID` to join anything (it is only displayed and journaled), so a foreign value cannot be dereferenced |

If one of those four types later gets an owning table, it must be added to
`checkEntity` in the same change.

RR-02, parent-addressed reads (404 for a foreign or unknown parent):

| Route | Parent lookup | Unit tests |
|---|---|---|
| `GET /assets/:id/inventories` | `inventory.ListAssetInventoriesUseCase`, `AssetRepository.GetByID(id, tenant)`; an own asset with none answers 200 `[]` | `inventory/list_asset_inventories_test.go` |
| `GET /automation/rules/:id/executions` | `automation.ExecutionService.ListByRule`, rule repository `GetByID(id, tenant)`; the handler now maps a typed error to its status | `automation/executions_test.go` |
| `GET /compliance/frameworks/:frameworkId/progress` | `compliance.GetComplianceProgressUseCase`, `GetFrameworkByID(id, tenant)`; the id is no longer echoed | `compliance/progress_test.go`: `TestGetComplianceProgressUseCase_NotFound`, `_Unauthorized` |
| `GET /audit-logs/user/:user_id` | `service.AuditService.EnsureUserInTenant`: a membership row in the caller's organisation (any status) or an audit row already recorded for that user in the caller's tenant (a removed member keeps their history, 200). A non-UUID `user_id` keeps today's empty 200. The 403 for a non-admin is evaluated first | `service/audit_user_in_tenant_test.go`; route: `handler/audit_log_user_ownership_test.go` |
| `GET /risks/:id/control-mappings` | `risk.ControlMappingUseCase.ListForRisk`, `RiskRepository.GetByID(id, tenant)` | `risk/control_mapping_list_test.go` |

Live suite probes: `probe_assets.go`, `probe_governance.go`, `probe_org.go`, `probe_risks.go`.

Not covered, noted for follow-up (decision D-131):
- Risk ids already stored by earlier versions on an incident may be foreign. They are not
  rewritten here. `application/incident/postmortem.go` reads the first stored risk id
  (`firstRiskID`) toward a mitigation, which is a possible read of a stored foreign id
  and is mitigated, not closed, by the write check above.
- `asset_ids` and `impacted_assets` on `POST /incidents` are not checked (they were never
  part of RR-12).

## Appendix: how to reproduce

```
make test-cross-tenant
cd backend && go test ./internal/security/...                        # ratchet + harness checks
cd backend && gosec -no-fail -fmt json ./...                         # SAST
cd backend && govulncheck ./...                                      # SCA, Go
cd frontend && npm audit --omit=dev                                  # SCA, npm
gh workflow run dast.yml --ref <branch>                              # DAST, see .github/workflows/dast.yml
```
