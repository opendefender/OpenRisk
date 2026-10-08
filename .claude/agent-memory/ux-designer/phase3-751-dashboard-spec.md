---
name: phase3-751-dashboard-spec
description: Findings from spec'ing issue #751 phase 3 (Dashboard & data) — KPI slot-reel counter, skeleton-reveal, texts-reveal, githubactivity/heatmap evaluation
metadata:
  type: project
---

Spec'd 2026-09-28 on branch `feat/751-phase3-dashboard` (read-only pass, no code written).
Full interaction spec delivered to caller via SubagentHandback that day — re-derive detail from
code if this memory and the code disagree, this is a snapshot.

**useCountUp exists in THREE copies, not two.** D-060's text (unmerged branch) names
`frontend/src/shared/ui.tsx:19` and `frontend/src/features/dashboard/shared.tsx:17`. A third,
independent copy lives at `frontend/src/features/onboarding/useActivation.ts:289` (900ms variant,
used by `RecognitionPage.tsx:97`, `PosturePage.tsx:205`). Phase 3 is scoped to "Dashboard & data"
only — recommend retiring the two dashboard copies into the new slot-reel counter and leaving
onboarding's copy alone (different phase, different scope), rather than a repo-wide consolidation.

**The issue's three KPI names ("risk score, open findings, % compliance") don't exist as one
triad anywhere in the code.** Real candidates, scattered across personas: risk score =
`ScoreGauge`/`tenantScore` (`DashboardPage.tsx:256`); "open findings" has no literal field —
closest are the "critical" tile in `DashboardPage.tsx`'s own KPI row (`:162-171`, :414-453) or
AnalystDashboard's "Ouvertes" open-vulnerabilities tile (`AnalystDashboard.tsx:62-63`); "%
compliance" is AuditDashboard's "Couverture moy." (`AuditDashboard.tsx:46-47`) or
`AuditDetailPage.tsx:149`'s `compliance_score`. Flagged as an open ambiguity for the PO to
resolve before wiring the counter to specific tiles — the counter component itself doesn't
depend on the answer.

**Naming collision risk for item 4 (githubactivity → risk heatmap).** OpenRisk already ships a
risk heatmap: `DashboardPage.tsx:460-553`, a 5×5 probability×impact matrix, explicitly commented
as "never a score band." A GitHub-style calendar/activity graph must NOT be called "heatmap" in
UI copy — recommended it as an "Activity"/incident-density strip near `WarRoomCard`
(`DashboardPage.tsx:856-905`, the existing "now" incident widget) instead, additive not
replacing. D-059 is binding: RareUI is reference-only, no port — item 4 output can only be a
mockup + written recommendation, escalated via `docs/DECISIONS.md` if the PO wants it built (it's
a new widget, not literally one of the three original phase-3 asks).

**Why:** GRC dashboards get screenshotted into board decks; the owner went against the PO's
"change once" recommendation and picked the in-house rolling counter anyway (D-060 A) — see
[[owner-overrides-on-scope-and-polish]] pattern (owner favours visible premium polish). The
motion budget and token locations for anything built here are in
[[token-inventory-and-motion-budget]] (art-director memory) — do not re-derive, but re-verify
line numbers before citing, they drift.

**How to apply:** when phase 3 implementation actually lands, check whether the PO resolved the
KPI-triad ambiguity and whether the onboarding `useCountUp` copy was touched (it shouldn't have
been, per scope). If a future task touches `WarRoomCard`, `HeatmapCard` or `TrendCard`, re-check
for the item-4 "Activity" strip before assuming it doesn't exist yet.
