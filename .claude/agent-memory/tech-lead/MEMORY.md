# Tech Lead — Memory Index

- [Tenant isolation defect patterns](defect-pattern-tenant-isolation.md) — the three shapes of cross-tenant leak already shipped, and which one CI still misses
- [W1-02 entity drawer boundaries](w1-02-entity-drawer-boundaries.md) — the 200a/200b/200c cut and the two real gaps on the branch
- [Agent hook defect patterns](defect-pattern-agent-hooks.md) — hooks run unsandboxed; transcript-chosen roots, silent cwd fallbacks
- [Bundle-splitting defect patterns](defect-pattern-bundle-splitting.md) — lazy i18n/shell pitfalls from #966: locale outside store, root-only ErrorBoundary
- [CI retirement traps](defect-pattern-ci-retirement.md) — required-context names on master, TruffleHog cron, CodeQL v3 EOL, monthly selfhost red (#650)
- [ADR 0001 — entity registry](adr-0001-entity-registry.md) — first ADR in the repo; the two decisions awaiting the owner
- [ADR 0005 — router matrix](adr-0005-router-matrix.md) — accepted 2026-10-15; source-reading authz tests go blind on move; C2 must fix #987
- [SQL backfill never runs at boot](defect-pattern-sql-backfill-not-run.md) — backfills miss DB_HOST boots (#757); late chain replay mutates live rows (#1001: 0056, 0076)
- [Overlay Escape stacks](defect-pattern-overlay-stacks.md) — two Escape stacks (layerStack vs escStack); popups rely on event phase; Tab trap suspended
- [Transcript token metrics](defect-pattern-transcript-metrics.md) — #1003: subagent usage is streamed (take max per id); gh api -F is an implicit POST; /pulls/N/reviews
- [Risk status vs lifecycle drift](defect-pattern-risk-status-drift.md) — PATCH status skips LifecycleState; legacy mapping ignores status when phase set (#757)
- [Merge drops one side](defect-pattern-merge-side-drop.md) — fe9422c5 lost #1059 repo methods AND #1062 helm mfa key; check every file both parents touched
- [OpenAPI misses middleware codes](defect-pattern-openapi-middleware-codes.md) — #1120: handler-faithful specs omit IP/tenant 429, CSRF 403, MFA guard 403/503
- [Vacuous CI checks](defect-pattern-vacuous-ci-checks.md) — gates green on empty fixtures/ignored status; helm lint ignores required; ZAP 2.17 baseline facts
- [Architecture drift baseline Oct 2026](architecture-drift-2026-10.md) — measured counts: gorm in domain, app→infra, 3 use-case layers, tr() i18n bypass, SyncEngine placeholder
- [Agent tool-grant traps](defect-pattern-agent-tool-grants.md) — #1004: Bash(pattern) in tools undocumented; memory adds Write/Edit; reviewer cwd; public memory
- [Routing/gate script traps](defect-pattern-routing-scripts.md) — #1005: pipefail+grep -q SIGPIPE on big diffs, cwd-relative pathspecs, no-merge-base = empty
- [ADR 0008 canonical model review](adr-0008-canonical-model.md) — 2026-11-03 approve-with-changes; edge envelope, derived-min, Control id, 0004 provenance
