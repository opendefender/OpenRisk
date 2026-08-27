---
name: openrisk-claim-matrix
description: The "invent nothing" rule and the Marketing Claim Matrix format for OpenRisk. Load before writing, reviewing or approving any README, ROADMAP, documentation, marketing copy or release note.
---

# The Invent Nothing Rule

No statement describing what OpenRisk does may ship unless the implementing
code exists and a user can reach it.

## The matrix — `docs/MARKETING_CLAIM_MATRIX.md`

| ID | Claim (EN) | Claim (FR) | Status | Evidence (file:line + test) | Surface | Verified on |
|---|---|---|---|---|---|---|

Status: `VERIFIED` · `PARTIAL` · `MOCKED` · `ABSENT` · `PLANNED`

## Rules

- `PLANNED` claims are future tense and visually marked on the surface where
  they appear. "Will support X in Q1" is honest. "Supports X" is not.
- `MOCKED` and `ABSENT` claims never appear on a public surface. No exception.
- Evidence is a file path plus a passing test. A design file, a screenshot, or
  a green build is not evidence that a feature works.
- A UI whose endpoint 500s because its table is excluded from `AutoMigrate` is
  `MOCKED`, not `PARTIAL`.
- A widget with a graceful fallback for a nonexistent endpoint is `MOCKED`.
- Re-verify the whole matrix before every release, site deploy and milestone close.
- Anyone may add a claim; only `product-verifier` may set `VERIFIED`.
