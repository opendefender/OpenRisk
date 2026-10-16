# Tech Lead — Memory Index

- [Tenant isolation defect patterns](defect-pattern-tenant-isolation.md) — the three shapes of cross-tenant leak already shipped, and which one CI still misses
- [Agent hook defect patterns](defect-pattern-agent-hooks.md) — hooks run unsandboxed; transcript-chosen roots, silent cwd fallbacks
- [Bundle-splitting defect patterns](defect-pattern-bundle-splitting.md) — lazy i18n/shell pitfalls from #966: locale outside store, root-only ErrorBoundary
- [CI retirement traps](defect-pattern-ci-retirement.md) — required-context names on master, TruffleHog cron, CodeQL v3 EOL, monthly selfhost red (#650)
- [ADR 0005 — router matrix](adr-0005-router-matrix.md) — accepted 2026-10-15; source-reading authz tests go blind on move; C2 must fix #987
- [SQL backfill never runs at boot](defect-pattern-sql-backfill-not-run.md) — boot is AutoMigrate only; backfills in migrations/*.sql miss first boot (#757)
- [Overlay Escape stacks](defect-pattern-overlay-stacks.md) — two Escape stacks (layerStack vs escStack); popups rely on event phase; Tab trap suspended
- [Agent tool-grant traps](defect-pattern-agent-tool-grants.md) — #1004: Bash(pattern) in tools undocumented; memory adds Write/Edit; reviewer cwd; public memory
- [Transcript token metrics](defect-pattern-transcript-metrics.md) — #1003: subagent usage is streamed (take max per id); gh api -F is an implicit POST; /pulls/N/reviews
