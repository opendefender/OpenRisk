---
name: risk-acceptance-posture
description: #811 residual-risk acceptance expiry review 2026-10-16 — approval "spent" rule, legacy status bypass via update endpoint, Save() lost-update vs expiry worker
metadata:
  type: project
---

Review of feat/811-acceptance-expiry (2026-10-16): no CRITICAL/HIGH. Tenant filters on all new queries correct; the one cross-tenant list (ListAcceptancesDue) is justified, writes carry tenant_id.

Findings handed back:
- MEDIUM: UpdateRisk (application/risk/update_risk.go `risk.Status = *input.Status`) writes the legacy `status` column raw, so `{"status":"accepted"}` makes a risk look accepted (pill, filters, Action Center exclusion, automation IsRiskResolved) with no approval and no expiry. Create and bulk were closed by #811; update was not. Has been there since before #811.
- LOW: GormRiskRepository.Update is a full-row Save(): a user edit loaded before the hourly expiry can write residual_accepted back (it heals on the next sweep, and the expired notice goes out twice).
- LOW: the sweep notifier resolves recipients with userRepo.GetByID without a tenant filter.
- Note: legacy AcceptRiskUseCase (internal/api/http/handlers/risks.go) is dead code with no route. Delete it before someone wires it up.

**Why:** the legacy `status` column is still written directly in places and still read by the UI and automation. Every "you can only reach state X through the guard" claim has to cover status too, not just lifecycle_state.
**How to apply:** in any lifecycle-guard review, grep for `.Status =` writes and for readers of `status` (Action Center, automation), not only SetState. See [[recurring-defect-areas]].
