---
name: app-wide-audit-2026-10-30
description: App-wide audit of master 5d4e5b89 on 2026-10-30 — child-create without parent tenant gate (mitigations), unscoped Preloads, notify SSRF still open, score-engine global state
metadata:
  type: project
---

App-wide audit of master 5d4e5b89, 2026-10-30. Verdict FAIL, 2 blocking findings, proven live on the throwaway `openrisk-audit` stack.

- CRITICAL: `POST /risks/:id/mitigations` does not check that the parent risk belongs to the caller's tenant. The legacy handler and `CreateMitigationPlanUseCase` have no risk port. Unscoped `Preload("Risk")` and `Preload("Mitigations")` then turn this into a two-way leak: B reads A's full risk, A's risk shows B's plan, and A's audit chain logs a phantom "create mitigation".
- HIGH: `pkg/notify` (Slack/Teams/webhook/SMS) still uses a bare `http.Client`. `/automation/channels/test` echoes the dial error, which gives an internal port-scan oracle. This was flagged on 2026-09-21 and is still unfixed. See [[ssrf-outbound-posture]].
- MEDIUM:
  - `/score-engine/metrics` aggregates risks across all tenants. The isolation registry marks `/score-engine/*` as Covered, which is false.
  - The score-engine config map is process-global.
  - A foreign `category_id` is accepted on a risk.
  - Viewer can DELETE `/reports/:id`, because the route is guarded by `compliance:controls:read`.
  - CSV formula injection in the governance export (via User-Agent).
  - `SCANNER_CREDENTIAL_KEY` has no production refusal and is missing from Helm.
- Clean at this audit: scanner collectors (now behind netguard), the typed-token gate (#944), SAML ACS (disabled), and GET IDOR across risks, assets and incidents.
- SyncEngine / `SYNC_ORGANIZATION_ID` is dead code: the TheHive config is never loaded.

**Why:** the dominant pattern has moved. By-id reads are now well gated. The leaks come from creates that take a parent ID (path or body) without a tenant check, combined with GORM Preloads that follow foreign keys without a tenant filter.
**How to apply:** in future audits, for every create that accepts a parent or foreign key, first create a canary object in tenant A, then reference it from tenant B and read back both sides. Also grep `Preload("` for scope funcs. Related: [[recurring-defect-areas]].
