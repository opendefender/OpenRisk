---
name: defect-pattern-risk-status-drift
description: PATCH /risks status writes legacy Status without LifecycleState; anything that reasons about a risk's state from that endpoint misreports it (#757)
metadata:
  type: project
---

`UpdateRiskUseCase` (update_risk.go, `risk.Status = *input.Status`) writes the legacy
`Status` directly. It never calls `SetState`, and `Risk.BeforeSave` does not reconcile.
`Risk.State()` prefers `LifecycleState`, so after an inline status edit in the register
(RiskRegisterPage `InlineStatus`), the Status column and the lifecycle stepper disagree.

There is a second trap. `RiskStateFromLegacy(status, phase)` looks at the phase first,
for every status except mitigated/closed/accepted/draft. So `open` and `in_progress`
on a phased risk map to the same state. Routing that endpoint through
`SetState(RiskStateFromLegacy(...))` is NOT a drop-in fix: it would turn "in progress"
back into the phase's state, and the derived Status would revert the user's click. Bulk
`change_status` already behaves this way.

Seen 2026-10-16 on #757: owner state-change e-mails built From/To with the legacy
mapping. They claim "moved to Mitigated" while the stepper still shows Assessed, and an
open→in_progress click sends nothing.

**How to apply:** any new consumer of "where a risk stands" (notices, reports, KPIs)
must read `Risk.State()`, and must treat a change made through the update endpoint as
a change of status, not of lifecycle state. The real fix is an explicit
status→state intent mapping, and it needs its own issue.
