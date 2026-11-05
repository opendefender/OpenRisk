---
name: analytics-dashboard-n-plus-1
description: GET /analytics/dashboard makes 116-117 sequential SQL round trips per call (day-loop + framework-loop aggregation)
metadata:
  type: project
---

`AnalyticsService.GetDashboardSnapshot` (`backend/internal/service/analytics_service.go:320`)
is the single worst N+1/aggregation-fanout in the codebase as of commit
5d4e5b89 (2026-10-30 perf audit). Two sub-methods are the cause:

- `GetRiskTrends` (`:145`) loops in Go over ~30 days, issuing 3-4 separate
  `COUNT`/`AVG` queries per day instead of one `GROUP BY date_trunc('day', ...)`.
- `GetFrameworkAnalytics` (`:274`) fetches distinct frameworks then loops,
  2 queries per framework — classic N+1.

Measured: 116 queries / 69-82ms at demo size (24 risks), 117 queries /
342-448ms at 10k risks for the tenant. The query *count* is constant (depends
on day-window and distinct-framework count, not row count), but each
individual query gets more expensive as the table grows, so the whole thing
degrades further with scale. Budget is p95 < 200ms; measured exceeds it by
1.85-2.2x even at demo-adjacent volumes.

**Why:** This dominates the dashboard's latency entirely — every individual
query is cheap (sub-10ms); the cost is 100+ sequential round trips, not query
plans. An index will not fix this; only collapsing to a handful of `GROUP BY`
queries will.

**How to apply:** Full details and the proposed fix (replace day-loop with
one `GROUP BY date_trunc`, replace framework-loop with one `GROUP BY framework`)
are in the perf audit report drafted as issue #2 in that session's output. If
this surfaces again, confirm with `docker logs` query-count-per-request before
assuming it's fixed — the fix was proposed, not applied, as of this memory.
See [[risks-missing-composite-tenant-index]] for the companion index finding
found in the same pass.
