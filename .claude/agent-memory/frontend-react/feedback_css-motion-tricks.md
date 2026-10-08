---
name: css-motion-tricks
description: Two CSS-only tricks used to implement OpenRisk's motion tokens without JS timing — peer-selector depth limit workaround and asymmetric enter/exit transition durations.
metadata:
  type: feedback
---

Two non-obvious CSS techniques, confirmed working in this codebase (Tailwind
v4, `@theme`-generated utilities) while building #751 phase 2's Checkbox
drawn-check glyph and DataTable search-clear motion.

**Tailwind's `peer-*` / `group-*` variants only reach a DIRECT sibling (or
descendant of that one sibling via `group`), never a nested descendant of a
sibling.** `peer-checked:opacity-100` on a `<span>` that is itself the
input's sibling works; the same utility written on a `<path>` nested two
levels inside that span does nothing, because the generated selector is
`.peer:checked ~ .peer-checked\:X`, and `~` only matches same-parent
siblings. Fix: put the `peer-checked:[--custom-prop:value]` on the actual
sibling, and have the deeply-nested descendant read the custom property via
`var(--custom-prop, fallback)` in its own `style` — custom properties inherit
through the whole subtree regardless of sibling-combinator reach.

**A CSS `transition` can have a different duration/easing for entering a
state than for leaving it, with no JS**, because the browser uses the
transition-duration/-timing-function of the state being transitioned INTO,
not the one being left. Declare the "exit" values on the base rule and
override them on the state selector (`:checked`, `peer-checked:`, etc.) for
"enter". `shared/ds/Field.tsx`'s invalid border already relied on this
implicitly; `shared/ds/Checkbox.tsx`'s drawn check (`--glyph-dur`/
`--glyph-ease` custom properties, draw on `--dur-base`/`--ease-out`, undraw
on `--motion-exit`) is the first place it's used deliberately and documented.

**Why:** the phase 2 motion spec forbade `requestAnimationFrame` and any new
animation library — everything had to be `transition`/`animation` so the
global `prefers-reduced-motion` rule in index.css (which does
`* { transition: none !important; animation: none !important; }`) covers it
automatically. Both tricks keep working correctly under that global kill:
reduced motion just makes the state change instant, never stuck mid-animation.

**How to apply:** reach for the custom-property bridge whenever a `peer-*`/
`group-*` state needs to drive something more than one level deep in the DOM
(icons inside icon wrappers, nested SVG paths). Reach for the asymmetric-
duration trick whenever a bistable UI element (checkbox, toggle, badge) needs
a different feel entering vs leaving a state, before reaching for JS state or
a second stylesheet rule keyed on a data attribute.
