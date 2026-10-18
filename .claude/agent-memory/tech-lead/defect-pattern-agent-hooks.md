---
name: defect-pattern-agent-hooks
description: Recurring defects in .claude/hooks (gate, checkpoint, guard) — fail-open fallbacks and hooks executing code chosen by agent-controlled input
metadata:
  type: project
---

Hooks run unsandboxed as the user. Any hook that picks a directory from agent-controlled
data (transcript, tool_input) and then runs `go test`, `npm run`, `npx` there is a sandbox
escape. Seen 2026-10-14 in `_roots.sh` (#958 round 2): a transcript `cd /tmp/evil` made the
gate run evil's `npm run lint:ceiling`.

**Why:** the gate was moved from session cwd to transcript-derived roots to fix a fail-open
(gating the main checkout while agents edit `../OpenRisk-wt-<n>`); the fix widened trust.

**How to apply:** when reviewing hooks, check (1) roots are restricted to worktrees sharing
the project's `git rev-parse --git-common-dir`; (2) every "nothing parsed" path is loud, not a
silent fallback to cwd (that is the original fail-open); (3) the main checkout does not become
a root via `git -C <main>`, relative `cd`, or memory/DECISIONS writes; (4) hook suites run from
`git archive <branch> .claude` snapshots, never inside a live worktree. Related:
[[defect-pattern-tenant-isolation]].
