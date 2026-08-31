# OpenRisk — Owner Decision Register

Anything an agent may not decide alone lands here. `po-openrisk` consolidates,
recommends, and surfaces these in the daily brief. Run `/decide` to clear them.

## Open

### D-007 — #411 criterion 14: the eight registers are not one pattern eight times
**Context** — Criterion 14 requires all eight registers to open the universal
drawer on row activation, and the issue's PO note is explicit that the milestone
is cut by dropping a whole issue, "never by shipping four registers of eight",
because inconsistent navigation is itself a defect. Surveying the eight on
2026-08-28 shows the criterion's premise — "one pattern applied eight times, one
commit each" — does not hold. The universal drawer offers four sections
(summary, relations, timeline, audit). Several registers currently open
something substantially richer, so wiring their rows to it **removes
capability**:

| Register | Current drawer | Swapping it costs |
|---|---|---|
| Risks | `RiskDrawer`, 9 tabs: details, lifecycle, score, smart, financial, miti, timeline, cti, ai — plus edit, export, create-mitigation | Financial quantification, SmartScore, CTI and AI tabs disappear from the product's most important screen |
| Evidence | `EvidenceDrawer`, 232 lines, **mutates** — approve/reject review, update validity | It is an editor. The issue's own rule retains editors (`MitigationDetailDrawer`, `ScanConfigDrawer`), and this one was mis-filed as a detail view |
| Infrastructure | `ScanConfigDrawer`, a scanner-config editor keyed by PROVIDER | The rows are scanner providers, not assets. Criterion 14 maps this register to `asset`; the mapping does not hold |
| Incidents | `IncidentDrawer`, 250 lines with actions | Needs a per-action check, not yet done |
| Compliance | `ControlDrawer`, 220 lines, 2 tabs (details/evidence) | Closest to like-for-like; the evidence tab maps onto relations |
| Vulnerabilities | `VulnDrawer`, inside the page | Needs a check |
| Assets / Inventory | `AssetHistoryDrawer`, 94 lines, no tabs, no mutations, opened from a "history" row action | **Nothing** — rows do not currently open a detail view at all, so the universal drawer is pure gain here |

So the eight are roughly: two clean wins, three needing a per-feature check, and
two that are clear regressions as specified.
**Options** — **A** wire only the registers where nothing is lost (assets,
inventory, and whichever of the middle three survive a check), retain the rest
with a stated reason under criterion 16, and accept mixed navigation · **B**
give the universal drawer an action that opens the rich view ("Open full risk
view"), so every row opens the drawer and depth stays one click away — uniform
navigation, no capability lost, but a design change beyond #411 · **C** keep
#411 to the two clean registers and move the rich ones to their own issue, where
each rich drawer's tabs are migrated into the universal drawer's sections
properly · **D** execute criterion 14 literally and accept the regressions.
**Recommendation** — **B**, and if it will not fit the milestone, **C**. D ships
a visible capability loss on the risk register. A is the thing the issue's own
PO note forbids, and it forbids it for a good reason.
**Cost of delay** — #411 cannot finish. Criterion 14 is its largest remaining
chunk and blocks the #200 umbrella DoD. Criteria 15 and 17 are done and shipped
regardless.
**Reversible?** — Yes, but B and C both cost more the later they start, because
every register wired the wrong way is rework.
**Status** — OPEN

## Resolved

<!-- Append: date · decision · rationale · issues unblocked -->

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

### D-008 — One Time to First Value number: 8 minutes · 2026-08-31
**Recommendation (po)** — Commit publicly to **8 minutes** from signup to the
Aha moment, and stop using 5 and 12 as if they meant the same thing.
**Context** — The tree currently promises three numbers.
`tests/e2e/activation.spec.ts:6-7,27,279` asserts under 8 min and
`docs/JOURNAL.md` states 8 min. `ROADMAP.md:139` and the original #234 text say
5 min. `deployment/monitoring/alerts.yml:195` warns at P50 > 12 min.
`docs/MARKETING_CLAIM_MATRIX.md` has no onboarding row at all, so none of them
is backed by anything. #234 is a `launch-gate`.
**Rationale** — 8 min is the only number an automated test actually asserts, so
it is the only one we can defend under RULE #12. Five minutes through a
five-step wizard plus a first risk plus a framework import is not provable
today. The 12-minute figure is an operational alert threshold with deliberate
headroom for a warning to be actionable; it is not a promise and must be
documented as not being one.
**Cost of delay** — Marketing can publish "5 minutes" while the test asserts 8
and the alert fires at 12. That is the exact failure mode the claim matrix
exists to prevent, on a launch-gate claim.
**Proceeding on** — 8 minutes, written into #234 criterion 5 so the work is not
blocked. An override to 5 changes one number in that criterion and nothing else
about the work.
**Owner call needed on** — whether the public promise is 5 or 8.
