# OpenRisk — Owner Decision Register

Anything an agent may not decide alone lands here. `po-openrisk` consolidates,
recommends, and surfaces these in the daily brief. Run `/decide` to clear them.

## Open

None. Every entry in this register is resolved as of 2026-08-31.

## Resolved

<!-- Append: date · decision · rationale · issues unblocked -->

### D-015 — CLA acceptance is enforced by a DCO check · 2026-08-31
**Decided** — Option A. A GitHub Action asserts the `Signed-off-by` trailer on
every commit. No third-party service, revisit at the first corporate contribution.
**Rationale (owner)** — Matches the recommendation. The trailer is already a real,
timestamped, per-commit record in git history, so the gap was enforcement rather
than evidence, and enforcement costs one workflow file. `cla-assistant` is a new
external service holding contributor personal data — the stronger artifact, but
not worth the data-processing question before a single corporate contribution has
arrived.
**Consequence** — A DCO workflow is added under `.github/workflows/` and made a
required check. `CONTRIBUTING.md` must state that `git commit -s` is mandatory.
Switching to `cla-assistant` later is a workflow change; commits merged under DCO
keep their trailer as their record, so nothing has to be re-signed.
**Unblocked** — #453 opened to execute it (`status:ready`). Enforcement is not live until
the owner flips the required-check setting by hand; #453 says so in its DoD.

### D-014 — `design-system/` is relicensed to Apache-2.0 · 2026-08-31
**Decided** — Option A. `design-system/` becomes `Apache-2.0` and gains a
`design-system/NOTICE`. The AGPL-3.0-only core and the
`LicenseRef-OpenRisk-Commercial` EE boundary are unchanged; only the design
system moves.
**Rationale (owner)** — Matches the recommendation. The design system is the
artifact the project *wants* copied and extended, and its value is protected by
trademark rather than by copyleft, so copyleft on it bought nothing and cost an
audit conversation on every enterprise deal. Option C — moving the two EE
consumers off the design system — was the worst available, because it forces the
EE surfaces to reinvent primitives, which is the exact fragmentation epic #439
exists to end.
**Consequence** — Irrevocable for anything published under it; future versions
can be relicensed but what ships cannot be clawed back. `LICENSING.md` gains a
`design-system/` row, every file under it takes an `Apache-2.0` SPDX header, and
`design-system/NOTICE` is created and thereafter records each vendored
third-party component with its upstream commit. `frontend/src/features/ai/` and
`frontend/src/features/reports/BoardReportPage.tsx` may import it with no
ownership argument required. Note the register's own path ambiguity: the vendored
design system lives at `frontend/design-system/` and no top-level
`design-system/` exists — the relicensing applies to the directory that exists.
**Unblocked** — #452 opened to execute the relicensing (`status:ready`, milestone `ds-v1`),
and it gates #443 and #444. Both of those remain `status:blocked` — D-014 was one of four
blockers on #443, not the only one; #446 must still land first, two dependencies are still
undeclared, and two spec gaps are still open. See the readiness report on #443.

### D-013 — Residual score is additive, in the SmartScore mould · 2026-08-31
**Decided** — Option A. A new `pkg/scoring` function computes residual from
`risk_control_mappings`. `Risk.Score` is untouched and stays frozen. One ADR is
written before PR 2 of #438 opens.
**Rationale (owner)** — Matches the recommendation. Option B answers a different
question — portfolio money, not this risk's posture — and putting a CFO figure
into a first-run teaching step is the wrong number in the wrong place. Option C
removes the one moment in the funnel where the product demonstrates something a
spreadsheet cannot, which is the reason step A5 exists.
**Consequence** — The ADR is the long pole, not the code, and it must land before
PR 2 opens. Reversible only while nothing persists to `Risk.ResidualRisk`; once
tenant rows carry computed residuals, changing the formula silently restates
their history, so the ADR fixes the formula before the first write.
**Unblocked** — #438, step A5 of PR 2.

### D-012 — `origin: "starter"` is a new `RiskSource` enum value · 2026-08-31
**Decided** — Option A. `SourceStarter RiskSource = "starter"` is added to the
enum and to `ParseRiskSource`.
**Rationale (owner)** — Matches the recommendation. `Risk.Source` is already the
field that answers "where did this risk come from", the column is `varchar(20)`
so no migration is needed, and the value inherits the existing validation. B adds
a second column answering the same question; C hides provenance in a
`CustomFields` blob that nothing indexes or validates, which PR 4 would then have
to grep to prove the starter rows are real.
**Consequence** — Reversible: an unused enum value is inert. PR 4 proves the
starter rows by querying `source = 'starter'`.
**Unblocked** — #438, PR 2.

### D-011 — `posture.revealed` is a non-catalogue key with its own assertion · 2026-08-31
**Decided** — Option A. `EventKeyPostureRevealed` is added,
`ValidateActivationSteps()` is left untouched, and a sibling
`ValidateNonChecklistEventKeys()` asserts that `posture.revealed` and
`aha.reached` are absent from `activationSteps`.
**Rationale (owner)** — Matches the recommendation. The mechanism the W1-05 brief
asked for already exists — `ActivationAhaReached` is a non-catalogue key today
and the validator iterates `activationSteps` only, so no amendment was ever
needed. What was missing is the assertion that a non-catalogue key never leaks
into the catalogue, which is exactly where a future edit would break it. Option C
would put a server-recorded outcome into a list of user chores and is refused by
the brief's own invariant 2.
**Consequence** — Reversible; an unused event key costs nothing to drop before
any row is written.
**Unblocked** — #438, PR 1.

### D-010 — Aha switches definition; `time_to_aha_seconds` gains an `aha_definition` label · 2026-08-31
**Decided** — Option A. `monitoring.TimeToAha` becomes a
`HistogramVec{aha_definition}`: v1 freezes, v2 starts empty, and
`SlowTimeToAha` in `deployment/monitoring/alerts.yml` selects
`aha_definition="v2"`.
**Rationale (owner)** — Matches the recommendation. Option B splices two
incomparable definitions into one P50 and makes the alert lie the moment the
switch lands; option C leaves the Posture Reveal decorative, which is the thing
the brief exists to fix. The label is what makes a wrong definition survivable.
**Consequence** — **Irreversible once v2 observations are written**: the v1
series cannot be reconstructed from them. Record for whoever implements this: the
real switch is executive-dashboard score computation → posture-summary
computation. The W1-05 brief's framing of it as "`first_risk` → `posture.revealed`"
is **wrong** and must not be copied forward — `aha.go:44` tests
`ScoreComputed && OwnDataPoints > 0 && ComplianceGaps > 0`, and `first_risk` is a
checklist step that has never defined the Aha. Carry forward one existing defect
that A must not inherit: `AhaReachedTotal.Inc()` sits *inside*
`ObserveTimeToAha` (`pkg/monitoring/activation.go:81`), so a tenant with no
signup anchor reaches Aha without being counted — `openrisk_reveal_reached_rate`
must not be derived from that counter.
**Unblocked** — #438, PR 1.

### D-009 — W1-05 base branch: resolved by events, #234 is merged · 2026-08-31
**Decided** — Option A, and it has already happened without the owner having to
act. The question was whether to merge #234 before cutting the W1-05 branches;
#234 merged as PR #437 on 2026-08-31 at 10:35Z.
**Rationale (owner)** — Not a judgement call any more. Verified rather than
assumed: `origin/234-w1-04-…` is **0 ahead of `master`** and 9 behind, and every
artefact the entry named is on `master` —
`backend/internal/infrastructure/repository/gorm_activation_repository.go`,
`tests/e2e/activation.spec.ts`, and `BackfillExistingMembers` in
`gorm_activation_backfill.go`, `membership/service.go` and `cmd/server/main.go`.
**Consequence** — All four W1-05 PRs are cut from `master`, not stacked. The
drift the entry warned about did not accumulate. Nothing to stack, nothing to
rebase.
**Unblocked** — #438, all four PRs.

### D-008 — One Time to First Value number: 8 minutes · 2026-08-31
**Decided** — Option A. The committed public promise is **8 minutes** from
signup to the Aha moment.
**Rationale (owner)** — Matches the recommendation. Eight is the only number an
automated test asserts, so it is the only one defensible under RULE #12. Five
minutes through a five-step wizard, a framework import and a first risk is not
provable today; publishing it would be the exact failure the claim matrix exists
to prevent, on a launch-gate claim.
**Consequence** — `AHA_BUDGET_MS` in `tests/e2e/activation.spec.ts` stays at
8 minutes and its assertion is the proof. `docs/MARKETING_CLAIM_MATRIX.md` C-002
carries the promise and names the test; the same file states explicitly that the
12-minute `SlowTimeToAha` threshold in `deployment/monitoring/alerts.yml` is an
operational warning with deliberate headroom and **not** the promise.
`ROADMAP.md` 17.6 no longer says 5. Changing the number later means changing all
three in one commit.
**Unblocked** — #234 needed no label change: it was never blocked on this, and
shipped on 8 in PR #437. The decision confirms what is already in review.

### D-007 — #411 criterion 14: universal drawer keeps an "Open full view" action · 2026-08-31
**Decided** — Option B. Every register row opens the universal drawer, and the
drawer carries a prominent action that opens the rich view where one exists.
**Rationale (owner)** — Matches the recommendation. Navigation becomes uniform
without deleting capability: the 9-tab risk surface (details · lifecycle · score
· smart · financial · miti · ai · timeline · cti, inline in
`features/risks/RiskRegisterPage.tsx`), the Evidence approve/reject editor and
the Infrastructure scanner-config editor all stay reachable, one click deeper.
Option D was rejected because it ships a visible capability regression on the
product's most important screen; option A because the issue's own PO note
forbids mixed navigation, and forbids it for a good reason.
**Consequence** — The universal drawer gains an "open full view" affordance, a
design change beyond #411 as originally written: `art-director` and
`ux-designer` should confirm the pattern and its copy before it is built. The
per-register mapping in criterion 14 still needs the per-feature check for
Incidents, Compliance and Vulnerabilities; Assets and Inventory remain pure gain.
**Unblocked** — nothing yet, and this needs saying: **#411 is already CLOSED**
(a merged PR closed it while criterion 14 was unfinished) and still carries a
stale `status:in-progress`. Option B therefore has no home. It needs either a
reopen of #411 or a new issue carrying criterion 14 plus this decision; see the
comment posted on #411.

### D-001 — Brand: OpenRisk everywhere · 2026-08-28
**Decided** — Option A. OpenRisk is both the company and the product name.
Karath is not adopted.
**Rationale (owner)** — The descriptive name's organic search equity is worth
more than a defensible mark at this stage. Recommendation was C (Karath company
/ OpenRisk product); the owner chose A.
**Consequence** — This is the status quo, so no rework: every existing package
identifier, domain and page already says OpenRisk. The trademark position is
knowingly weak — "OpenRisk" is descriptive, so a competitor may use the phrase.
Revisit only if a mark becomes commercially necessary, and note this decision is
effectively irreversible once the site is indexed.
**Unblocked** — every `area:marketing` and `area:docs` issue.

### D-002 — Cameroonian framework `cm-loi-2024-017`: dropped · 2026-08-28
**Decided** — Option C. The claim is removed entirely rather than shipped as a
placeholder or marked `PLANNED`.
**Rationale (owner)** — Recommendation was A-or-B; the owner chose C. A
regulatory claim with no resolvable citation to official source text is exactly
the liability the compliance doctrine exists to prevent, and carrying it as
`PLANNED` still advertises coverage the product does not have.
**Consequence** — `cm-loi-2024-017` comes out of the framework catalogue and out
of any marketing or docs that reference it. The African catalogue's differentiator
narrows to the frameworks that are actually sourced. If the official text is
obtained later, it returns as a new issue with a full citation.
**Unblocked** — the framework catalogue can ship without a placeholder row.

### D-003 — Label taxonomies: keep both · 2026-08-28
**Decided** — Option C. Both label families stay; every new issue carries both.
**Rationale (owner)** — Recommendation was A (declare the wave family canonical);
the owner chose C. Neither family is retired and no backlog migration happens.
**Consequence** — **Every new issue must be dual-labelled**, as #409-#412 already
are: one `area:` and `priority:` from the wave family, one from the Constitution
family, plus a `status:`. Agents opening issues must apply both. CLAUDE.md's
label table stays as written and is not amended. The cost is a permanently
doubled label set per issue; that is accepted deliberately.
**Unblocked** — `issue-triage` may proceed; dual-labelling is now the rule, not a
transitional state.

### D-004 — Audit trail: observable best-effort now, guaranteed later · 2026-08-28
**Decided** — Option A now, Option B before any regulated-customer commitment.
**Rationale (owner)** — Matches the recommendation. Failing a user's successful
mutation in order to record it (option C) is worse than the gap; but silent
incompleteness is the wrong default for the ISO 27001 A.8.15 / A.5.28 artefact
sold to COBAC- and BCEAO-supervised institutions.
**Consequence** — `middleware.AuditMutations` keeps `_ = appender.Append(...)`,
and a dropped write becomes observable through an error log and a counter. That
work is #410 criteria 40-41. The outbox/queue (option B) is **not** in any child
issue today and must be opened and scheduled before a regulated customer is
signed — it is a commitment, not an aspiration.
**Unblocked** — #410 criteria 40-41 · #412 criterion 22.

### D-005 — ADR-0001 accepted: panic at boot, raw sequential ids · 2026-08-28
**Decided** — Accept the ADR. D2-A (panic at boot on an incomplete registration)
and D4-A (keep raw sequential incident ids, mitigate enumeration rather than
hide it).
**Rationale (owner)** — Matches the recommendation. A registry that boots with an
ungated type is the exact defect W1-02 exists to prevent, and opaque ids are a
migration this milestone cannot absorb.
**Consequence** — `docs/adr/0001-polymorphic-entity-contract-and-registry.md`
moves to `Status: accepted`. D2-A was already implemented and verified in
`9fb2305`, so the code and the ADR now agree. D4-A is **irreversible once deep
links are in the wild**: incident URLs will carry sequential integers
permanently, and the enumeration mitigation (tenant filtering on every read,
identical 404s for forged and foreign ids) is what makes that safe. It is
declared in the registry as `EnumerationMitigation` and enforced by
`TestRegistry_SequentialIDRequiresAnEnumerationMitigation`.
**Unblocked** — #410 criteria 1-2 · the #200 umbrella DoD.

### D-006 — Plan matrix reverted out of the chore commit · 2026-08-28
**Decided** — Option B. The entitlements rewrite is reverted off
`feat/w1-02-universal-entity-drawer` and re-proposed as its own issue.
**Rationale (owner)** — Matches the recommendation. The new matrix may well be
right, but it changes what the product gives away, and nobody reviewing
"agent company v3" was reviewing pricing.
**Consequence** — `backend/pkg/entitlements/entitlements.go` and its test return
to their `master` state on this branch, which turns `TestAllowed_FeatureGate`
green again. The volume-throttled model is re-proposed on a dedicated issue so
it is reviewed as a pricing change, with the stale
`internal/application/entitlements/service_test.go` updated as part of THAT work.
**Unblocked** — `feat/w1-02-universal-entity-drawer` stops carrying a red test
unrelated to its own scope.
