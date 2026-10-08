---
name: perf-audit
description: Measure and fix OpenRisk performance against the declared budgets — API p95, SQL query plans, N+1, Core Web Vitals, bundle size. Use before a release or when something feels slow.
argument-hint: "[endpoint, page, or 'all']"
---

# Performance audit: ${ARGUMENTS:-all}

Delegate to `performance-engineer`. Measure before touching anything.

## Backend
`EXPLAIN ANALYZE` the queries behind the target. Check that every tenant-scoped
query has `tenant_id` as the leading column of a usable index. Hunt N+1 by
counting queries per request, not by reading code.

## Frontend
Lighthouse on a throttled 4G mid-range mobile profile. Bundle analysis on the
initial route.

## Report — no claim without a number

| Surface | Budget | Before | After | Guard added |
|---|---|---|---|---|

End with `PERF: PASS` or `PERF: FAIL — <n> budgets exceeded`, and open an issue
per exceeded budget.
