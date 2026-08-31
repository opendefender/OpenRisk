# OpenRisk — Owner Decision Register

Anything an agent may not decide alone lands here. `po-openrisk` consolidates,
recommends, and surfaces these in the daily brief. Run `/decide` to clear them.

## Open

### D-014 — Relicense `design-system/` to Apache-2.0?
**Context** — `frontend/src/features/ai/` and
`frontend/src/features/reports/BoardReportPage.tsx` are
`LicenseRef-OpenRisk-Commercial` and import the AGPL design system. This is
resolvable today by copyright ownership — OpenDefender holds the copyright in
both, so it may combine them — but a third-party auditor doing a procurement
review sees commercial code importing AGPL code and has to be talked out of the
obvious reading. Relicensing `design-system/` to Apache-2.0 removes the argument
instead of answering it.
**Options** — **A** relicense `design-system/` to `Apache-2.0` and add
`design-system/NOTICE` · **B** leave it AGPL and document the ownership argument
in `LICENSING.md` · **C** leave it AGPL and move the two EE consumers off the
design system.
**Recommendation** — **A**. The design system is the artifact you *want* copied
and extended; its value is protected by trademark, not by copyleft. Copyleft on
it buys nothing and costs an audit conversation on every enterprise deal. C is
the worst option: it forces the EE surfaces to reinvent primitives, which is the
exact fragmentation issue #439 exists to end.
**Cost of delay** — Low but non-zero, and it grows. Every new file added under
`design-system/` is one more header to change later, and #443/#444 are about to
add a lot of them. Doing it before the primitive layer lands is materially
cheaper than after.
**Reversible?** — **No, not practically.** Apache-2.0 is irrevocable for anything
already published; the project can relicense future versions but cannot claw back
what shipped. This is why it is your call and not the agent's.
**Status** — OPEN

### D-015 — How is CLA acceptance enforced: `cla-assistant` or DCO?
**Context** — `CLA.md` now exists (this branch) and acceptance is recorded by a
`Signed-off-by` trailer. Nothing enforces it: there is no CLA check, no DCO
check, and no PR template in `.github/`. A CLA nobody verifies protects the
dual-licensing right about as well as no CLA, which is what makes the Enterprise
Edition defensible.
**Options** — **A** DCO check — a GitHub Action asserting the `Signed-off-by`
trailer on every commit; no third party, no data leaves GitHub, no account to
administer · **B** `cla-assistant` — a hosted bot storing signatures against
GitHub identities; a real audit trail, but a third-party service holding
contributor identity data · **C** `cla-assistant` self-hosted — the audit trail
without the third party, at the cost of running it.
**Recommendation** — **A now, revisit at the first corporate contribution.** The
trailer is a real, timestamped, per-commit record in git history and costs one
workflow file. B is the stronger artifact but is a **new external service** and
touches contributor personal data, which is exactly the kind of call the
constitution routes to you rather than to an agent.
**Cost of delay** — Accrues per merged PR. Every external contribution merged
without a recorded acceptance is one more contribution whose dual-licensing
status rests on the CONTRIBUTING.md text alone.
**Reversible?** — Yes. Switching enforcement later is a workflow change; already
merged commits keep whatever record they have.
**Status** — OPEN

## Resolved

<!-- Append: date · decision · rationale · issues unblocked -->

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
