---
name: cleanup-prs-leave-ratchets-loose
description: When a cleanup PR delivers a stale backlog issue, check whether the count-ceiling test guarding it was tightened; #645 deleted 108 orphans but left the <=110 ratchet
metadata:
  type: project
---

A cleanup can merge through a different issue and leave the original backlog issue
stale, with its guard test still pinned to the old count. #303 ("Delete the 110
orphaned modules") was delivered by PR #645 / #637 on 2026-09-14: orphans went
110 → 2, and both survivors are justified (vitest setupFiles, an ambient .d.ts).
But `frontend/src/__tests__/deceptive-ui.test.ts:110` still said
`toBeLessThanOrEqual(110)`. I rewrote #303 into the residual (exact `orphanFiles`
equality, S, P2, ready) instead of recommending closure.

**Why:** a ceiling ratchet that is never tightened lets the gain erode silently.
Closing the issue as delivered would have recorded a safeguard that did not exist.
The #303 body was also wrong on the stack: it prescribed `go mod tidy` for
frontend TS modules.

**How to apply:** when an issue's scope has already been done by another PR,
grep for the numeric pin that guards it (`toBeLessThanOrEqual(N)`, baseline files)
before choosing between "close as delivered" and "rewrite to residual". Prefer an
exact-list assertion over a smaller ceiling, since a ceiling lets additions offset
removals. Related: [[verify-agent-claims-before-drafting]], [[comments-claiming-guarantees]].
