---
name: router-extraction-posture
description: ADR 0005 / #991 router-out-of-main review (2026-10-15) — recording Mounter is evidence-only; conditions each child PR (C1-C17) must keep
metadata:
  type: project
---

Reviewed ADR 0005 + spike `spike/992-rbac-roles-router` on #992 (2026-10-15): SECURITY PASS, confirmed enforcement unchanged.
Comment: https://github.com/opendefender/OpenRisk/issues/992#issuecomment-6187611426

Open non-blocking findings to re-check on every #991 child PR:
- MEDIUM: `router.New(r, prefix, access, …)` access/prefix are caller-asserted; Inventory splice must derive them from the AST position of the Register call in main.go.
- LOW: Verify skips HEAD and USE routes; terminal `.Use` around the Mounter escapes until C17.
- LOW: `Check{}` = deny-all but records `perm()`; both fields set = role only. Must panic at mount.
- LOW: guard variables (57 exist, e.g. `riskDelete`) passed in the handler list are enforced but unrecorded; nil/Guard{} compiles.

**Why:** the issue's risk is a guard silently dropped or reordered during a 460-route move; source-reading tests go blind (not red) when routes leave main.go.
**How to apply:** for each child PR check: C1 merged first with a route-count ratchet; Register call at the same line; Use chain main.go:972-1035 and IPRateLimit:507 untouched until C17; no request-time reader of Matrix (else auth redesign, D-entry). Related: [[recurring-defect-areas]], [[ci-security-gates-posture]].
