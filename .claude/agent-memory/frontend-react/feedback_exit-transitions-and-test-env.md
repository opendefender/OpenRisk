---
name: exit-transitions-and-test-env
description: Durable facts kept from the #751 phase 4–5 status files — a ds Modal/Drawer exit only plays if the caller keeps it mounted, sonner's real a11y behaviour, and jsdom's clipboard
metadata:
  type: feedback
---

Drawn from the #751 phase 4 and 5 status files (2026-09-29).

- **The exit transition needs a mounted component.** ds `Modal`/`Drawer` play
  their exit only when the caller passes `open={false}`. A caller that writes
  `{flag && <Modal open … />}` or returns `null` before the drawer cuts the
  exit. Render unconditionally and drive `open`.
- **sonner (2.0.7) facts, read from its source and pinned by tests in
  `ThemedToaster.test.tsx`:** every toast type shares one `aria-live="polite"`
  region, so no toast is ever `role="alert"`; hover pauses the dismiss timer,
  keyboard focus on a toast action does not. Re-check after a sonner bump.
- **jsdom's `navigator.clipboard` is a real fake.** Assigning or
  `defineProperty` at module scope loses to it; `vi.spyOn(navigator.clipboard,
  'writeText')` per test, after `render`, works.
- **A single flaky test in a full parallel run** is resource contention until
  proven otherwise: rerun the suite and the file alone before calling it a regression.

Related: [[matchmedia-spy-restore-trap]], [[shake-remounts-detach-node-refs]].
