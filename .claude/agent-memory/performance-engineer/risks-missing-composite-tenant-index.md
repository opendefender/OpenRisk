---
name: risks-missing-composite-tenant-index
description: risks table only has a single-column tenant_id index; default list/count queries do Seq/Bitmap scan + full sort, or scan other tenants' rows
metadata:
  type: project
---

As of commit 5d4e5b89 (2026-10-30 perf audit), the `risks` table's only
tenant-scoped index is `idx_risks_tenant_id` (single column). The register's
default list query (`WHERE tenant_id = ? ORDER BY created_at DESC LIMIT 25`,
the actual default per `domain.NewRiskQuery()`) and the status-filtered
`COUNT` both pay for it:

- List: Bitmap Heap Scan over ALL of the tenant's rows + a top-N heapsort,
  instead of an Index Scan that stops at 25. Measured 16.27ms → 0.28ms
  (58x) after adding `(tenant_id, created_at DESC)` to a throwaway copy of
  the audit DB.
- Count with `status` filter: the planner picks the single-column
  `idx_risks_status` (global, not tenant-scoped) over `idx_risks_tenant_id`,
  and visits OTHER TENANTS' rows before rejecting them by tenant_id
  (`Rows Removed by Filter: 5000` in a 3-tenant, 30k-row test table). Measured
  9.6ms → 3.8ms after adding `(tenant_id, status)`.

**Why:** This is the concrete mechanism behind CLAUDE.md rule #2 ("tenant_id
as the leading column of its index, or the filter scans") — without the
composite, a busy tenant's query cost scales with the whole platform's row
count, not just that tenant's rows.

**How to apply:** The fix (two `CREATE INDEX` statements, see the perf audit
report / issue #3 draft) was proven in a throwaway DB, not applied to the
repo — no migration exists yet as of this memory. Before claiming it's fixed,
check `backend/migrations/` for an index named `idx_risks_tenant_created_at`
or `idx_risks_tenant_status`. The same systematic check (`pg_indexes` joined
against every column named `tenant_id`) found 72/74 tenant-scoped tables
already have a usable tenant_id-leading index — `risks` is the significant
exception, not the norm. See [[analytics-dashboard-n-plus-1]].
