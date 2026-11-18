# 0012 — Control status engine: vocabulary, freshness windows, and an engine-written evaluation cache

Status: accepted (agent decision under D-089, recorded as D-119 in `docs/DECISIONS.md`)
Proposed: 2026-11-18, design gate of #555
Builds on: ADR 0008 D4 (freshness rule) and D6 (`declared_status` is a claim; the computed status is derived and not stored in the contract), D-108 Q4/Q6, D-113 (exceptions), D-118 (results and evidence on a result)

## Context

#555 computes what a control's status really is from the chain merged in #553/#1415/#1416 and the
exceptions of #1428. Five forces decide the shape:

1. **The worst lie is a PASS on stale or missing proof.** ADR 0008 D4: an absent signal never reads
   as a good one, freshness reads `observed_at`, never `created_at`.
2. **Status changes with the clock alone.** An exception starts or expires, evidence passes its
   `valid_until`, a result ages out of its window. No write happens at those instants, so any stored
   status is wrong from the moment one of them passes until something recomputes it.
3. **Every result recorded over HTTP is a person's statement** (`openrisk_user`, D-118 point 7).
   No collector writes results yet. A user-entered "pass" with nothing attached is an assertion,
   and the auditor persona of #555 is "trust that the status was computed, not asserted".
4. **Writes must not rescan the catalogue.** One evidence write recomputes only the controls it
   covers (#555 criterion 5).
5. **The score readers still read `compliance_controls.status`** (D-108 Q4). Moving them moves
   residuals and SmartScore; that is #1465, not this ADR.

## Decision

### 1. Vocabulary — five values

`pass` · `fail` · `insufficient_evidence` · `exception_active` · `not_applicable`.

#555 listed four; `not_applicable` is added. Without it an out-of-scope control reads
`insufficient_evidence` forever, and ADR 0003 already excludes `not_applicable` from both sums, so
every roll-up would have to special-case it. Unknown is `insufficient_evidence`, never `pass`.

### 2. Per-test verdict

The in-scope tests of a control are the live tests of its live implementations whose status is not
`retired`. For each, the **latest result** is the first in D-118's order
(`observed_at DESC, recorded_at DESC, id DESC`).

| Latest result | Verdict |
|---|---|
| none | unknown |
| `inconclusive` | unknown |
| `fail`, any age | **fail**: an old failure is never evidence of a pass |
| `pass`, not fresh (point 4) | unknown |
| `pass`, fresh, source class not `user_entered` | pass |
| `pass`, fresh, `user_entered`, with at least one fresh covering evidence attached to **that** result | pass |
| `pass`, fresh, `user_entered`, nothing fresh and covering attached | unknown |

Covering is `Evidence.Covers` at the instant (accepted, unexpired), plus D-118 point 8's integrity
rule: a file counts only with a `content_hash`. Evidence attached to an older result of the same
test is history: it is listed nowhere and counts for nothing.

### 3. Control status, in this order

1. The control does not resolve live in the caller's tenant: `ErrNotFound` (no status).
2. `applicability = not_applicable`: `not_applicable`.
3. The **underlying status**: `fail` if any in-scope test's verdict is fail; else `pass` if there is
   at least one in-scope test and every verdict is pass; else `insufficient_evidence`.
4. If at least one exception is `ActiveAt(now)` (D-113: not revoked, started, not expired) and the
   underlying status is not `pass`: `exception_active`. Otherwise the underlying status.

So an active exception waives a `fail` or an `insufficient_evidence`, never shows as `pass`, and
never hides a real `pass`. An expired, revoked or not-yet-started exception is not credited (#555
criterion 4). The underlying status is always returned beside the status, so "fail, waived" is
readable.

Control-level `evidenced_by` edges (the transitional ones the mirror and reconciler write) feed
freshness and the source list, **never a pass**: a pass needs a test. Consequence, stated: every
legacy split control reads `insufficient_evidence` until a test with a result exists.

### 4. Freshness

A signal is **fresh** at `now` when its `observed_at` is set, is not later than `now + 5 min`
(D-118's skew), and `now < observed_at + W + W/10`, where `W` is its window and `W/10` the grace
(a monthly run a day late is not stale; a P1D test has 2.4 h).

| Signal | `observed_at` | Window `W` |
|---|---|---|
| latest result of an in-scope test | the result's | the test's `cadence` (ISO 8601, calendar arithmetic for `M` and `Y`); `P90D` when the test has none |
| evidence attached to that latest result | `evidences.collected_at` (ADR 0008 D7) | the same test's window |
| control-level evidence | `evidences.collected_at` | `P365D` (an annual cycle: the usual policy review interval and the certification surveillance interval; no clause of the standard fixes it) |

Evidence is fresh only if it is also covering at `now`. A zero `collected_at` is absent, so not fresh.

**Freshness percentage** = fresh signals / counted signals × 100, two decimals. Counted signals are
the latest result of each in-scope test plus every live evidence in the two evidence rows above
whose review is not `rejected` (a rejected artifact is not proof and somebody said so; pending and
expired ones count and are not fresh). With no counted signal the percentage is 0 and the count is 0.

No tenant override of the defaults in this slice: it needs a settings field, UI and its own
tenant-isolation review. The defaults are constants in one file and the override is a later issue.

### 5. Live on read; the cache is engine output, not a source

- `GET /compliance/internal-controls/:controlId/status` computes live at the request instant, in
  one read-only snapshot, in at most five queries. `evaluated_at` is that instant. It never reads
  the cache, so it can never serve a status the clock has already overtaken.
- `control_status_evaluations` holds one row per control, `(tenant_id, control_id)` primary key,
  written only by the engine: status, underlying status, fresh and counted signals, percentage,
  `evaluated_at`, `next_change_at` (the earliest instant the clock alone can change the result) and
  `stale`. No route writes it, no DTO carries it. No user-facing read trusts it in this slice;
  #1465 decides whether score readers join it.
- **Incremental trigger.** Every write that can change an input sets `stale = true` on the rows of
  the controls it touches, **inside its own transaction**, so a commit and its invalidation cannot
  be separated by a crash. Evidence resolves its controls through `canonical_edges`
  (control-level, and test-level through the test's implementation); a result, an attachment, a
  test or an implementation through its control; an exception by its `control_id`. A deleting
  write marks before it removes the edges it would resolve through.
- **Scheduler.** A worker claims rows that are stale, rows whose `next_change_at` has passed, and
  live controls with no row (cold fill), in bounded batches. It clears `stale` when it claims,
  recomputes each control with the same tenant-scoped loader as the read, and upserts guarded by
  `evaluated_at` so an older computation never overwrites a newer one. A write that lands after
  the claim marks the row again and it is picked up on the next tick. A row whose control is gone
  is deleted.
  The claim also leases what it takes: `next_change_at` is set to the claim instant plus five
  minutes, so a crash before the save is retried when the lease ends and a second replica does not
  take the same rows. A live control with no row gets a placeholder row at claim time
  (`evaluated_at` = the epoch means never computed, `stale` false), so a write that lands during the
  computation finds a row to mark. Narrow race: a write committing while the placeholder insert is
  still uncommitted finds no row to mark; only the cache is affected and the window is
  milliseconds wide.

## Consequences

Easier:
- The status a user reads is right at the instant they read it, with no job having run.
- A pass always names the run and the proof behind it; the source list is the audit trail of the
  computation.
- An evidence write costs one `UPDATE` in its transaction and a recompute of only its controls.

Harder:
- Every write path of the chain, the exceptions and the evidence register carries one mark call.
  A path that forgets it leaves only the cache stale, never a user-facing read, and the cold-fill
  and `next_change_at` sweeps do not catch it: the implementation PR lists every write path and
  tests each mark.
  One writer is known to be unmarked: `controlchain/reconcile.go`, the boot reconciler, inserts
  control-level `evidenced_by` edges and backfills `mitigated_by` edges without marking. A control
  whose edge only the reconciler wrote keeps a stale cache until another write or its
  `next_change_at`; the live read is unaffected. #1466 tracks it.
- A mark locks the evaluation rows it touches in `control_id` order before it updates them, so two
  writes marking the same controls in opposite order wait instead of deadlocking.
- Legacy controls read `insufficient_evidence` until tests exist. That is the honest reading and the
  reason #1465 needs a transition rule.
- Manual passes need evidence attached to the run.

## Alternatives rejected

- **Serve the cache on read when not stale and before `next_change_at`.** Saves four queries per
  read; a single missed invalidation then serves a wrong PASS. Five queries are inside the budget.
- **No cache at all.** Criterion 5 asks for an observable incremental recompute, and #1465 needs a
  joinable per-control status; computing N controls live per score read is the N+1 this codebase
  forbids.
- **Recompute synchronously inside each writer's transaction.** Adds five queries and the whole
  engine to every evidence write, in legacy repositories, for a result no user-facing read uses.
- **Post-commit hook or the realtime outbox as the trigger.** A crash between commit and hook loses
  the invalidation; the outbox (ADR 0010) holds the tenant's counter row for the rest of the
  transaction, which every evidence write would then pay.
- **Freshness from `valid_until` alone.** `valid_until` is how long a document stays valid, not when
  the world was observed; old proof with a far expiry would read fresh, which D4 forbids.
- **A user-entered pass counts on its own.** Turns the computed status back into a declared one.
- **A stale fail decays to `insufficient_evidence`.** Turns a known failure into an unknown.
