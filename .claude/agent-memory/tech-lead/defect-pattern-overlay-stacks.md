---
name: defect-pattern-overlay-stacks
description: The frontend has two independent Escape/overlay stacks; popups that join one stack silently rely on event-phase ordering to beat the other, and suspend the ds Modal's Tab trap
metadata:
  type: project
---

Two Escape stacks coexist in the frontend (seen in the #935 review, 2026-10-15):
`layerStack` in `shared/ds/useDismissableLayer.ts` (ds Modal/Drawer, capture listener, also traps Tab)
and `escStack` in `shared/useBackTo.ts` `useEscapeToClose` (hand-rolled overlays such as MembersView `Overlay`, a bubble listener on document).

**Why:** a popup that registers on `layerStack` (the #935 Listbox `useEscapeLayer`) beats `escStack` dialogs only
because its capture-phase `stopPropagation` on document stops the document bubble listener. Nothing tests that, and
moving either listener to capture/window breaks it. Pushing the light layer also makes the ds Modal skip its Tab trap
while the list is open, so Tab from an open list that is the dialog's last control leaves the dialog.

**How to apply:** for any new popup/overlay, ask which stack it joins, require a test against BOTH stack kinds, and
check that Tab is still trapped while it is open. Unifying the two stacks is a debt candidate. See [[defect-pattern-bundle-splitting]].
