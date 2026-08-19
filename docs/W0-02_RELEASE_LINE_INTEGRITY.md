# W0-02 — Release-Line Integrity

## Executive Summary
This audit establishes the current state of the OpenRisk repository. The repository exhibits an exceptionally linear integration pattern: **all active feature branches and PRs have been merged into `master`**. 
The `master` branch is the singular, trustworthy release line. There is **zero branch divergence** remaining locally or remotely. The primary risks discovered lie in deployment manifests (hardcoded `latest` tags) and automated migration behavior (GORM AutoMigrate running on boot without safe multi-pod locking), which represent P1 operational risks.

## Current Mainline
- **Branch**: `master`
- **HEAD Commit**: `60e8804` (Merge pull request #249)
- **Status**: Clean and fully merged. This is the canonical source of truth.

## Mainline Provenance
- **Latest Tag**: `v1.1.0-rc.1`
- **Commits since tag**: 286
- **Construction**: The mainline has been constructed through a strictly disciplined, sequential merging of Pull Requests from `opendefender/OpenRisk`. 
- **Lost Commits**: None detected. All historical branches tracking work (e.g., `docs/launch-plan-2026`, `feat/bank-grade-quality-hardening`, `feat/w0-01-transformation-audit-v2`) are fully reachable from `master`.

## Remote Branch Inventory
All feature branches have been merged and deleted from origin, leaving only:
- `master` (HEAD)
- `backup` (Fully merged into master)
- `feat/w0-01-transformation-audit-v2` (Fully merged into master)
- `gh-pages` (Orphan branch for documentation website)

There are 8 local tracking branches (e.g., `docs/ux-audit-2026-08`, `fix/migration-consolidation`) which are marked `[gone]` on the remote and are 100% merged into `master`. 
**Disposition**: RETAIN `master` and `gh-pages`. RETIRE all others locally.

## Pull Request Inventory
- **Total PRs**: 186
- **Open PRs**: 0
- **Closed PRs**: 186 (of which 184 were successfully merged).
- **Disposition**: No human decision required for PRs. All capabilities are already in mainline.

## Branch Divergence Analysis
- **Divergence**: None. The history is fully unified. `master` represents the sum of all implemented capabilities.

## Feature and Capability Reconciliation
Since all branches are merged, capabilities are unified. No feature branches are competing to be merged.

## Duplicate Implementations
- **Scoring**: `backend/internal/domain/risk_scoring.go` (weights table struct) vs `backend/internal/domain/scoring/` (Clean Architecture compute engine). Not duplicates, but rather data model vs calculation engine.
- **Dashboards**: `DashboardPage.tsx` delegates to role-specific dashboards (`AnalystDashboard`, `ExecDashboard`, etc.). Not a duplicate, but role-based UX segregation.
**Disposition**: RETAIN all as intended architecture.

## Migration Inventory
- **System**: Additive SQL via `golang-migrate` running on top of `GORM AutoMigrate`.
- **Active SQL Migrations**: `migrations/` directory contains `0048` to `0056` (`.up.sql` / `.down.sql`).
- **Archive**: `migrations/_archive/` contains legacy `0001` to `0047` migrations. As per `migrations/_archive/README.md`, these are purely historical and invisible to the runner. `GORM AutoMigrate` is the schema authority.

## Migration Lineage
- The lineage in `migrations/` is clean and sequential (`0048` through `0056`).
- No duplicate IDs or concurrent migrations exist in the active set.

## Migration Ordering Risks
- **Critical Risk**: `GORM AutoMigrate` is executed automatically on boot (`main.go`). In a multi-pod environment, concurrent schema mutations by AutoMigrate can cause race conditions or crash fresh databases (documented in `backend/internal/infrastructure/database/database.go`).
- **Secondary Risk**: The SQL migrations (`0048`-`0056`) are run *after* AutoMigrate. If AutoMigrate fails, the app aborts. This is technically safer but the lack of distributed locking for AutoMigrate is a P1 deployment risk.

## Database / Domain / API / Frontend Integrity
- Integrations are unified on the `master` branch.
- Feature flags are explicitly listed in `.env.example` (e.g., `FEATURE_WEBHOOKS=true`, `FEATURE_SYNC_ENGINE=true`, `FEATURE_GAMIFICATION=true`). No orphaned flags were found in the scope of this audit.

## Feature Flags
| Flag | Location | Default | Status |
|---|---|---|---|
| `FEATURE_WEBHOOKS` | `.env.example` | true | Active |
| `FEATURE_SYNC_ENGINE` | `.env.example` | true | Active |
| `FEATURE_GAMIFICATION` | `.env.example` | true | Active |
| `MFA_REQUIRED_ROLES` | `.env.example` | admin,root | Active |

## Deployment Manifest Inventory
- **Docker Compose**: `docker-compose.yaml`, `docker-compose.test.yaml`, `docker-compose.staging.yaml`, `deploy/selfhost/docker-compose.yml`
- **Helm**: `helm/openrisk/` charts with `values-dev.yaml`, `values-staging.yaml`, `values-prod.yaml`
- **Divergence**: Massive use of mutable `latest` tags across Helm and Docker Compose manifests (`deploy/selfhost/docker-compose.yml`, `helm/values-dev.yaml`, `helm/values-staging.yaml`, `helm/openrisk/values.yaml`).

## Environment Drift
- **Intentional**: `docker-compose.staging.yaml` uses a separate `postgres-staging` configuration.
- **Accidental**: Version mismatches across compose files (`3.8` vs `3.9`) and hardcoded `latest` image tags causing non-deterministic deployments.

## Branch Disposition Matrix
| Branch | Current State | Unique Changes | Migration Risk | Regression Risk | Disposition |
|---|---|---|---|---|---|
| `master` | Canonical | N/A | High (AutoMigrate) | Low | RETAIN |
| `gh-pages` | Orphan | N/A | None | None | RETAIN |
| All other local refs | Merged | 0 | None | None | RETIRE |

## Regression Validation Plans
- Since no new branches are being merged, validation is restricted to ensuring the `master` branch passes existing CI pipelines and E2E tests.

## Reconciliation Plan
No action required for branch merging/rebasing.
- **Phase 1**: Retire local tracked branches (cleanup).
- **Phase 2**: Resolve `latest` tag mutation risk in manifests (Future wave).
- **Phase 3**: Implement distributed locking or a dedicated init-container for `GORM AutoMigrate` (Future wave).

## P0/P1 Risk Register
| ID | Risk | Severity | Evidence | Impact | Owner | Mitigation |
|---|---|---|---|---|---|---|
| R-01 | AutoMigrate Race Conditions | P1 | `main.go` calls `AutoMigrate` directly on boot | Crash loop on multi-pod scaling | UNASSIGNED — HUMAN DECISION REQUIRED | Extract AutoMigrate to init-container or CLI tool. |
| R-02 | Mutable `latest` Image Tags | P1 | `helm/values-dev.yaml`, `deploy/selfhost/docker-compose.yml` | Non-deterministic deployments | UNASSIGNED — HUMAN DECISION REQUIRED | Pin specific immutable tags in manifests. |

## Release Readiness
The mainline `master` is unified, but the operational deployment risks (R-01, R-02) block a safe zero-downtime release.

## Live Proof Record
- **Date**: 2026-08-19
- **Commit SHA**: `60e8804`
- **Branch**: `chore/w0-02-release-line-integrity`
- **Environment**: Local Linux Sandbox
- **Command Output**: `git branch --merged master` confirms all feature branches are successfully merged. `gh pr list --state open` confirms 0 open PRs.

## Known Limitations
- Could not verify if archived legacy migrations (`0001` to `0047`) perfectly match the `GORM AutoMigrate` generated schema on a fresh install, though the repository treats `AutoMigrate` as the ultimate authority.

## Recommended Next Actions
- Approve the retirement of local tracking branches.
- Assign owners to R-01 and R-02.
