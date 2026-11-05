---
name: architecture-drift-2026-10
description: Measured Clean Architecture drift on master 5d4e5b89 (2026-10-30) — GORM in domain, app→infra imports, three use-case layers, dead internal/api/http, tr() i18n bypass
metadata:
  type: project
---

Baseline measured 2026-10-30 on master `5d4e5b89`, so future reviews can tell whether drift is shrinking:

- `internal/domain`: 30 files import gorm (hooks `Risk.AfterSave` writes RiskHistory from the domain); 0 import Fiber.
- `internal/application`: 15 files import `internal/infrastructure/*` (6 in mitigation), 1 imports Fiber (realtime/mutation_bridge.go).
- Three use-case layers coexist: `internal/application` (42k lines), legacy `internal/service` (8k, 25 importers), handlers with `database.DB` (82 refs, team_handler worst; tx + N+1 there).
- `internal/api/http/handlers` is dead code (main.go:1511 says so). Real handlers: `internal/handler`.
- Only 2 use cases own a transaction (`RiskTxRunner`, `AssetTxRunner`).
- Frontend: JSON FR/EN key parity is exact but misleading — ~1,600 inline `tr('fr','en')` calls with ~130 local copies of the helper bypass the catalogue.
- SyncEngine starts unconditionally on a hard-coded tenant UUID and TheHive client falls back to mock incidents (main.go:422, thehive/client.go:66).

**Why:** the CLAUDE.md invariants say these are blockers, yet the counts show they were never enforced by tooling, only by review.
**How to apply:** in reviews, block any PR that raises these counts; push for a depguard/`go list -deps` CI guard instead of re-flagging by hand. Related: [[adr-0005-router-matrix]], [[defect-pattern-tenant-isolation]], [[defect-pattern-ci-retirement]].
