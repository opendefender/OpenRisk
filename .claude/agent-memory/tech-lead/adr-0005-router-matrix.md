---
name: adr-0005-router-matrix
description: ADR 0005 (#992, gate for #991) accepted 2026-10-15 — recording Mounter is evidence only; source-reading authz tests go blind when routes leave main.go; C1-C18 staging
metadata:
  type: project
---

ADR 0005 (`docs/adr/0005-router-out-of-main-and-route-matrix.md`, #992) was accepted by
tech-lead review on 2026-10-15, after edits. It gates #991 (routes out of `main()`, plus a
route-to-permission matrix).

Decisions that hold: `router.Mounter` records one row per mount and builds the guard from
the same `Check` value. It is **evidence only**: enforcement stays in
`middleware.RequirePermission/RequireRole`, so no D-entry is needed. The before-matrix is an
AST golden file frozen in C1. `routes.Inventory()` splices the built rows in at each
`router.Register…` call. Move PRs C2-C16 replace mounts in place. C17 moves the `Register`
calls and `Use` statements *down* into `Build`. C18 gathers construction into
`*_wiring.go`; I split it out of C17 because it is the 2,800-line move.

**Recurring trap, found again here:** tests that read `main.go` as text go *blind*, not red,
when code moves. The author's list of readers missed
`internal/domain/permission_catalog_sync_test.go` and wrongly included
`saved_view_contract_test.go`. PR #987 (an AST walk over the whole backend) goes red on
`Check.handler`'s spread argument and cannot see `router.Permission(...)`, so C2 must teach
it both.

**Why:** the issue's main risk is a guard silently dropped in the move while source-reading
tests stay green.

**How to apply:** when reviewing any #991 child, grep for every test that reads
`cmd/server/main.go`. Check that the `SessionOnly` sentinel is non-nil, and that `Verify`
counts HEAD routes. Check that `Inventory()` fails closed on an unknown `Register` call, and
that the access a call claims matches its AST position. See
[[defect-pattern-tenant-isolation]] and [[defect-pattern-ci-retirement]].
