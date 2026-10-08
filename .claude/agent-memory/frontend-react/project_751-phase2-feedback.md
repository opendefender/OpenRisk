---
name: 751-phase2-feedback
description: Status of issue #751 phase 2 ("Feedback & validations") on branch feat/751-phase2-feedback — what shipped, what was deliberately deferred, and why.
metadata:
  type: project
---

Branch `feat/751-phase2-feedback`, cut from `master` at `e1aafd5` so phase 1
(#819) stays independently reviewable. The binding spec (art-director +
ux-designer, arbitrated) was posted to the scratchpad and on issue #751; its
"motion budget" (durations/easings/translate-scale caps, `pathLength=1` for
SVG draws, everything as CSS transition/animation so the global
`prefers-reduced-motion` rule in `index.css:243`-ish covers it) is the
authority for any future motion work on this issue's family.

**Shipped, five atomic signed commits, `feat/751-phase2-feedback`:**
1. `Button` `feedback="success"` gets a drawn check (`shared/ds/Button.tsx`,
   new `shared/ds/useSuccessFeedback.ts`), wired on CreateRiskModal,
   EditRiskModal, ChangePasswordCard, OrganizationProfileForm — fires only
   after the mutation resolves, reverts after ~1.8s.
2. `Shake` promoted from `features/auth/fields.tsx` to `shared/ds/Shake.tsx`
   (fields.tsx now re-exports it, six existing auth call sites unchanged);
   `Field` gets a `shakeKey` prop. Wired on `CreateRiskModal`'s title +
   description fields; description textarea moved onto `Field`/`Textarea` as
   the spec's stated prerequisite.
3. `Checkbox`'s check glyph is now a drawn `pathLength=1` path instead of
   lucide's `Check` — see [[feedback_css-motion-tricks]] for the
   custom-property bridge this needed.
4. New `shared/ds/DeleteButton.tsx` (trigger-only, mandatory `aria-label`,
   danger tint on hover/focus only — not permanently red). Adopted on
   `MembersView.tsx` (revoke, still gated by `DangerConfirm`),
   `RemediationPage.tsx` and `AuditsPage.tsx` (both still gated by their
   existing undo-toast `remove()`). ~30 other `Trash2` triggers in the repo
   are explicitly follow-up debt, not touched.
5. `DataTable.tsx` search-clear now shows a falling/fading snapshot of the
   cleared text and returns focus to the input.

**Deliberately NOT done, with reasons (do not re-litigate without a new
decision):**
- Sub-action checklist UI on mitigations — deferred to its own issue, needs a
  new screen, not phase-2 polish.
- Two-step/hold-to-delete pattern — ux-designer's call, would be a third
  confirmation pattern next to `DangerConfirm`/undo-toast.
- `AttributeSearchBar.tsx` and `UserPicker.tsx` search-clear motion — the
  spec named them but neither actually has the affordance being retrofitted
  (AttributeSearchBar only removes filter chips; UserPicker's × closes the
  whole panel, not just the query). Building one would be a new feature.
- Consolidating the three vital-delete dialogs (`AlertDialog`/
  `DangerConfirm`/`ImpactDialog`) — pre-existing debt, out of scope.

**Verified before commit each time:** `npx tsc -b`, `npx eslint <files>
--max-warnings=0`, relevant vitest files; at the end, full `npx vitest run`
(804 tests, all green) and `npm run budget` (200.4 KB vs a confirmed
`master` baseline of 199.9 KB — +0.5 KB, inside the ~1 KB allowance).

**Next:** motion-designer and qa-automation review, a live Playwright pass,
then open the PR with "Part of #751" — none of that was done in this
session (no push, no PR, per the task's explicit instructions).

**Review-fix round, five more atomic signed commits on the same branch
(2026-09-28), closing out the motion-designer/QA review of the above:**
1. `Checkbox`: the checked-glyph wrapper no longer carries `opacity-0
   peer-checked:opacity-100` at all — CheckGlyph's own dashoffset already
   hides it at rest, and gating an untransitioned wrapper opacity on top of
   that was what made uncheck snap instantly instead of undrawing. Opacity
   gating now applies ONLY to the indeterminate Minus glyph (which has no
   drawn state of its own); the two glyphs stay mutually exclusive because
   they render from the same ternary slot, never both mounted.
2. `DataTable.tsx`: dropped `motion-safe:animate-or-fadein` from the search
   `<input>` itself (only the snapshot overlay should animate — the input's
   border/background was flashing on every clear), then, separately,
   `clearedSnapshot` is now cleared on ANY value change (typed, or synced
   back from the URL), not just on the CSS `animationend` — so typing before
   the exit transition finishes can no longer render old text under new.
3. `CreateRiskModal.tsx` / `EditRiskModal.tsx`: removed the `flashSuccess`/
   `useSuccessFeedback` wiring outright. Both modals close in the same batch
   as their mutation resolving, and `Modal` renders `null` once closed, so
   the drawn check had no exit to paint on — dead code the first round
   shipped without a jsdom check that would have caught it. The toast stays
   the only confirmation on these two; `useSuccessFeedback`'s doc comment now
   says it is for forms that STAY mounted after saving (ChangePasswordCard,
   OrganizationProfileForm), which keep the wiring.
4. `primitives.test.tsx` gained the axe-core `describe` block it was missing
   (formControls.test.tsx and feedback.test.tsx already had one), covering
   `Button feedback="success"`, `DeleteButton`, and an invalid `Field` with
   `shakeKey` set — the three review-flagged surfaces.

Every fix above got a regression test proven to fail on the pre-fix code via
an explicit `git stash` round-trip (documented in the handback, not just
asserted). Full `npx vitest run`: 812/812 green (was 804 before this round).
`npm run budget`: still 200.4 KB, byte-identical before and after this
round's diff (confirmed by building both trees) — the failing 180 KB gate is
pre-existing on `master`, not something this round moved.
