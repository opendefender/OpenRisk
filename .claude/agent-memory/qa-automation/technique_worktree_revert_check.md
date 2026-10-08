---
name: technique-worktree-revert-check
description: How to prove a new test is meaningful (fails without the fix) without touching the tracked working tree
metadata:
  type: feedback
---

To check that a newly-added test actually fails against the pre-change code
(not a tautology), don't `git stash`/checkout inside the real working tree —
too easy to leave it dirty. Instead:

```
git worktree add --detach <scratchpad>/wt-before <parent-sha>
ln -s <repo>/frontend/node_modules <scratchpad>/wt-before/frontend/node_modules
cp <repo>/frontend/src/.../new.test.tsx <scratchpad>/wt-before/frontend/src/.../new.test.tsx
cd <scratchpad>/wt-before/frontend && npx vitest run <path> -t "<test name>"
git worktree remove --force <scratchpad>/wt-before   # cleanup, always
```

Symlinking `node_modules` avoids a full reinstall — vitest/vite resolve
through the symlink fine. This confirms the new assertions fail for the
right reason (missing attribute/behavior), not for an unrelated import error.

**Why:** the task explicitly forbade modifying tracked files in the real repo
even transiently; a worktree is a separate checkout so nothing in the primary
tree ever changes.
**How to apply:** any time asked to verify "would this test fail if the
change were reverted" — standard QA due diligence on every new test in a
review task.
