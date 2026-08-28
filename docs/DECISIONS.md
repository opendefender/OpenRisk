# OpenRisk — Owner Decision Register

Anything an agent may not decide alone lands here. `po-openrisk` consolidates,
recommends, and surfaces these in the daily brief. Run `/decide` to clear them.

## Open

### D-001 — Brand: OpenRisk
**Context** — The name has oscillated through development. Every marketing page,
domain, and package identifier written before this is settled will need rework.
**Options** — A: OpenRisk (descriptive, SEO-friendly, generic) · B: Karath
(distinctive, trademarkable, no organic search equity) · C: Karath as the
company, OpenRisk as the product.
**Recommendation** — C. Keeps the SEO value of the descriptive product name
while giving the company a defensible mark.
**Cost of delay** — Every marketing and docs issue is blocked or will be redone.
Grows with every page written.
**Reversible?** — No, in practice, once the site is indexed.
**Status** — OPEN

### D-002 — Cameroonian personal data framework (`cm-loi-2024-017`)
**Context** — Still a placeholder; the source text was never supplied. It is the
only remaining gap in the African regulatory catalogue that is the product's
main differentiator.
**Options** — A: supply the text and implement · B: ship without it and mark it
`PLANNED` on the site · C: drop the claim entirely.
**Recommendation** — A if you can source the text this month, otherwise B.
**Cost of delay** — A `PLANNED` gap in the headline differentiator.
**Reversible?** — Yes.
**Status** — OPEN

### D-003 — Two label taxonomies, never reconciled
**Context** — The repo carries both label families. The CLAUDE.md/Constitution
family (`area:backend|frontend|db`, `priority:P0-critical…P3-low`, all five
`status:*`) exists but had **zero issues using it**; the family every one of the
100+ backlog issues actually uses is `area:foundation|security|grc|…` and
`priority:P0|P1|P2|P3`. No issue in the backlog carries a `status:` label at all.
The Constitution family was created by `scripts/gh-labels.sh`, which is itself
new on `feat/w1-02-universal-entity-drawer` and **not on master** — so the two
families were never reconciled, rather than one being ignored.
**Options** — A: declare the wave family canonical and amend CLAUDE.md · B:
migrate 100+ issues onto the Constitution family · C: keep both, dual-labelling
every new issue (what #409-#412 do today).
**Recommendation** — A. Migrating 100+ issues to satisfy a document is the
expensive direction, and the wave family is what every board and filter speaks.
**Cost of delay** — Low per issue, but every new issue permanently doubles its
label set until this is settled.
**Reversible?** — Yes.
**Status** — OPEN

### D-004 — Audit trail: best-effort or guaranteed?
**Context** — `middleware.AuditMutations` writes with `_ = appender.Append(...)`
(`audit_mutations.go:78`), deliberately: a trail write must never fail the
business call the user just completed. The consequence is that a dropped write
makes the entity timeline and the global feed incomplete **with nothing saying
so**. For a product whose audit trail is the ISO 27001 A.8.15 / A.5.28 artefact
sold to COBAC- and BCEAO-supervised institutions, silent incompleteness is the
wrong default. The middleware arrived in fix commit `7f2b8fa` and was never
issue-tracked until now.
**Options** — A: stay best-effort, but make a dropped write observable (error log
+ counter) · B: make the write guaranteed via an outbox/queue · C: fail the
mutation when the trail write fails.
**Recommendation** — A now, B before any regulated-customer commitment. C is
wrong: failing a user's successful mutation in order to record it is worse than
the gap. A is already scoped as #410 criteria 40-41; B is real work with a real
cost and is not in any child issue.
**Cost of delay** — Low technically, high reputationally. This is the kind of
thing a supervisor asks about directly.
**Reversible?** — Yes.
**Status** — OPEN

### D-005 — ADR-0001 acceptance: panic-at-boot, and raw sequential ids
**Context** — `docs/adr/0001-polymorphic-entity-contract-and-registry.md` is the
project's first ADR and is `proposed`. Two of its decisions need the owner:
**D2** makes `Registry.Register` panic at boot on an incomplete registration
(fail-closed by construction, at the cost of a process that refuses to start);
**D4** keeps sequential integer ids raw on incident routes, mitigating
enumeration rather than hiding it. #410 cannot reach `status:ready` on its ADR
criteria until this is settled, though its other criteria can proceed.
**Options** — D2: A panic at boot · B log-and-refuse-the-type. D4: A keep raw ids
with an enumeration alarm · B opaque ids (migration, breaks existing links).
**Recommendation** — D2-A and D4-A. A registry that boots with an ungated type is
the exact defect this wave exists to prevent; opaque ids are a migration this
milestone cannot absorb.
**Cost of delay** — Blocks ADR acceptance, which blocks #410's criteria 1-2 and
the umbrella #200 DoD.
**Reversible?** — D2 yes. D4 no, once links are in the wild.
**Status** — OPEN

### D-006 — The open-core plan matrix was changed inside a `chore(agents)` commit
**Context** — `7a47f97 chore(agents): autonomous agent company v3` also rewrote
`backend/pkg/entitlements/entitlements.go`, moving the product from
"fewer features on Free" to "same product, throttled by volume". Free now
receives `FeatFinancialQuant: LevelBasic` (deterministic ALE), `FeatSmartScore`
and `FeatExecutiveDashboard`, with Monte-Carlo reserved for Pro. The change is
coherent and well argued in its own comments, and `pkg/entitlements` tests were
updated with it — but `internal/application/entitlements/service_test.go` was
not, so `TestAllowed_FeatureGate` fails on the branch and passes on `master`.
Pricing and licensing are owner decisions under the constitution, and this one
arrived inside a commit whose subject says it is about agent configuration.
**Options** — A ratify the new matrix, update the stale application-level test,
and record the pricing change in its own issue and commit · B revert the
entitlements change out of the `chore(agents)` commit and re-propose it on its
own · C keep the new matrix but restore Free to no financial quantification.
**Recommendation** — B. The matrix change may well be right, but it must not
ride in on a housekeeping commit: it changes what the product gives away, and
nobody reviewing "agent company v3" was reviewing pricing.
**Cost of delay** — The branch stays red on a test that looks unrelated to its
work, and Free currently ships a paid analytic in this build.
**Reversible?** — Yes, but only cheaply before a release goes out.
**Status** — OPEN

## Resolved

<!-- Append: date · decision · rationale · issues unblocked -->
