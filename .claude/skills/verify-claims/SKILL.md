---
name: verify-claims
description: Verify every OpenRisk public claim against the codebase and refresh the Marketing Claim Matrix. Use before any release, milestone close, site deploy or README change.
argument-hint: "[optional: surface or path to verify]"
---

# Claim verification: ${ARGUMENTS:-all public surfaces}

Delegate to `product-verifier`.

Scope: `README.md`, `ROADMAP.md`, `docs/`, marketing site content, release
notes, and the issues closed in the current milestone.

Require the full classification table and the final
`RELEASE: GO` / `RELEASE: NO-GO` line.

List every `MOCKED` and `ABSENT` claim **first**, above the summary, with the
surface it appears on and the exact line to remove or rewrite. Open a
`type:docs, priority:P1-high` issue for each.
