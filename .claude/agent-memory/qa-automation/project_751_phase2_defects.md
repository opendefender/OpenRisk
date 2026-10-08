---
name: project-751-phase2-defects
description: Two defects found reviewing #751 phase 2 (feat/751-phase2-feedback) — batched-unmount swallows Button success check, DataTable search-clear snapshot can overlap freshly typed text
metadata:
  type: project
---

Reviewed 2026-09-28 on branch `feat/751-phase2-feedback` (5 commits: success
check, Shake promotion, Checkbox glyph, DeleteButton, DataTable search-clear
snapshot). Full suite green (81 files/804 tests), tsc, eslint all clean per
frontend-react's own report. Two defects survived that pass, both real but
neither breaking the truth rule (check #2 — success never fires early/on
error) — reported as review findings, not fixed (QA does not silently fix).

**Defect 1 — success check never paints in CreateRiskModal/EditRiskModal.**
`frontend/src/features/risks/CreateRiskModal.tsx:160` and
`frontend/src/features/risks/components/EditRiskModal.tsx:151` call
`flashSuccess()` then `handleClose()`/`onClose()` synchronously, no `await`
between them. `Modal.tsx:96` (`if (!open) return null`) has no exit
transition. React 18 automatic batching merges both state updates (child's
`feedback='success'`, parent's `isOpen=false`) into one commit; the reconciler
never even renders the Button subtree with the new feedback value because the
Modal's new output is already `null`. Proved empirically with a
`useLayoutEffect` probe that never fires when the unmount is present, and does
fire in an otherwise-identical control without the unmount (React batches
updates from different components in the same synchronous continuation, not
just the same event handler — this is the mechanism, and it's easy to
mis-model as "should paint for one frame"; it does not). Net effect: the
checkmark is genuinely visible in ChangePasswordCard and
OrganizationProfileForm (neither unmounts on success) but is dead code in the
two modal forms. Not a truth-rule violation (never shows early/wrongly), but
undercuts the spec's stated intent ("plays... reverts after ~1.8s") for half
the wired call sites.
**Why:** relevant any time a transient "flash" UI state is set immediately
before a synchronous unmount/navigation in this codebase — the same batching
will swallow it. [[technique_worktree_revert_check]] pairs with this kind of
finding: reproduce with a minimal harness before trusting the reasoning.
**How to apply:** when reviewing any "show X then close Y" sequence, check
whether X's owner survives the same commit as Y's unmount; if not, the visual
never happens, so the "reverts after Nms" framing in commit messages/specs is
optimistic, not verified. Fix options (not decided, flag to the assignee):
delay the close by one tick, or accept the checkmark is decorative only on
the two persistent-page forms and drop it from the two modals.

**Defect 2 — DataTable search-clear snapshot can overlap freshly typed text.**
`frontend/src/shared/datatable/DataTable.tsx` `clearSearch` (~line 504) sets
`clearedSnapshot` to the old text and clears `searchDraft`, but nothing clears
`clearedSnapshot` when the user types again. The snapshot only unmounts on its
own `onAnimationEnd` (~line 549). Reproduced with a scratch test: click clear,
then `fireEvent.change` with new text before the exit fires — the old
snapshot span (`aria-hidden`, absolutely positioned over the same input
region) is still mounted, overlapping the newly typed text. In a real browser
this is bounded to the `--motion-exit` window (`--dur-fast` = 120ms,
`frontend/src/styles/primitives.css:80`), so it self-resolves fast, but for
that window a fast typist sees the old and new text overlaid. No existing
test in `DataTable.test.tsx`'s `describe('search clear', ...)` covers this
rapid clear→type sequence even though the commit message for 662ecce
explicitly claims it's handled.
**Why:** this is exactly the class of gap flagged before in
[[project_ds_menu_exit_transition]] — a short (~120ms) exit-animation window
where old and new UI state coexist, easy to miss because jsdom tests only
fire `animationEnd` manually and never simulate "user acts again before the
animation would have ended in a real browser".
**How to apply:** for any "old content fades out while new content is
underneath" pattern gated by a manual `onAnimationEnd` unmount, add a test
that types/acts again *before* firing `animationEnd`, not just one that fires
`animationEnd` and checks cleanup.
