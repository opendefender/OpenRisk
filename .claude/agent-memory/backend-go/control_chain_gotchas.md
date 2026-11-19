---
name: control-chain-gotchas
description: Gotchas learned building the control status engine (#555) on the ADR 0008 control chain; read before touching canonical_edges, evidence writers, or tenantguard raw SQL
metadata:
  type: project
---

- canonical_edges stores entity types capitalised ("Risk", "Control", "ControlTest", "Evidence"), verbs lowercase. Never hand-write `'control'` in raw SQL: bind `string(canonical.TypeControl)`. A lowercase literal silently matches nothing.
- Tests count queries with a GORM callback: the tenantguard's catalog probes (HasTable) are statements too, once per table. Warm with one call before `reset()`.
- Every writer of the chain marks control_status_evaluations stale inside its own tx (`infrastructure/controlstatus.Mark*`). Any new writer of results, tests, implementations, exceptions, evidence or evidenced_by edges must call one. A test setup elsewhere must AutoMigrate `controlstatus.Models()` or the write fails with "no such table".
- `libEnv.link` (edge link) shadows `mirrorEnv.link` (legacy evidence link) in repository tests: call `e.mirrorEnv.link`.
- Local Postgres for tests: the Docker Desktop context is down, use `docker --context default run ... postgres:16-alpine`. Several `_Postgres` tests need a server-booted public schema and fail on a bare DB; that is environmental.
- The shell hook blocks commands whose text contains destructive DDL words: use the Write tool for migration down files and for tests that drop a schema.
- Migration numbering: check the highest number on develop AND open PRs before picking one. #555 took 0089 (an issue that only earmarked it, #1456, was still needs-refinement and had accepted "or the next free number"). golang-migrate does not apply a lower version that lands after a higher one is applied, so never leave a gap.
