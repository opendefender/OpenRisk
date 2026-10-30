---
name: bulk-operations-posture
description: bulk_operations service/handler security posture after #1215 (2026-10-30) review — tenant scoping OK, open pre-existing debts in the async processor
metadata:
  type: project
---

#1215 (commit d73766f8, 2026-10-30) added `serializer:json` to BulkOperation.FilterQuery/UpdateData so rows read back on Postgres. Reviewed: SECURITY PASS.

Tenant scoping verified: Get = id+tenant_id, List = created_by+tenant_id, getRisksByFilter/countResourcesByFilter start from tenant_id; filter keys are a fixed allow-list, values bound (no SQLi).

Open pre-existing debts (follow-ups, not yet issues as of 2026-10-30):
- GET /bulk-operations/:id is tenant-scoped but NOT created_by-scoped and has no permission gate — any member reads any member's job (filter/update_data now actually readable after #1215).
- processBulkAssign appends a Mitigation with no TenantID (uuid.Nil) and ignores mitigation_id.
- updateRiskFromData: no range validation (impact 0-10, prob 0-1), no score recompute, no audit trail; delete via goroutine, no tx.
- Handler echoes err.Error() on create (400) and list (500).
- isolation registry row /api/v1/bulk-operations/{id} still Pending; the pg test could pin it.

**Why:** bulk ops is a wholesale-mutation path; repeated area (#529, #1215).
**How to apply:** on any bulk_operation change, re-check these items first. See [[recurring-defect-areas]].
