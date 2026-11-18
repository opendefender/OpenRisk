---
name: adr-0012-control-status-engine
description: #555 design gate (D-119, ADR 0012, 2026-11-18) — 5-value status vocabulary, pass needs evidence when user-entered, live read + engine-only cache, score readers split to #1465
metadata:
  type: project
---

Gate of #555 decided on 2026-11-18 (D-119, ADR 0012). Both docs were drafted uncommitted and are meant to land with the #555 PR.

- Vocabulary has five values. `not_applicable` was added to the issue's four.
- Precedence: `fail` beats everything, at any age. An active exception turns a non-pass into `exception_active` and never hides a pass.
- A pass needs a fresh latest result. A `user_entered` result (every HTTP result) also needs fresh covering evidence on that same result.
- Freshness window: the test cadence, else P90D. Control-level evidence uses P365D. Grace is W/10.
- GET `/compliance/internal-controls/:id/status` always computes live, in ≤5 queries.
- `control_status_evaluations` is written by the engine only. No user read trusts it. Writers mark `stale` inside their own transaction, and a worker claims rows through `tenantguard.CrossTenant`.
- Score readers were split out to #1465 (needs-refinement). Every legacy split control reads `insufficient_evidence` until it has a test, so re-pointing the readers needs a transition rule.

**Why:** the worst lie is a PASS on stale proof, and status changes with the clock alone. A cache served on read can always be overtaken by time.

**How to apply:** review the #555 PR against D-119.
- Check that every write path marks stale, and that delete paths mark before removing edges.
- Check that the endpoint never reads the cache.
- #1465 must not ship without an ADR for the transition.

Also learned on this gate:
- Briefs can carry stale dependency state. The brief said PR #1459 was still open, but it had merged into `develop` the day before. Check PR state yourself.
- Worktrees can sit hundreds of commits behind `develop`. Read through `git archive origin/develop` into the scratchpad. For a shared doc like DECISIONS.md, edit develop's copy so the diff against `origin/develop` is only the addition.

Related: [[defect-pattern-tenant-isolation]], [[defect-pattern-sql-backfill-not-run]].
