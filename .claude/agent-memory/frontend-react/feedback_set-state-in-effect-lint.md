---
name: set-state-in-effect-lint
description: eslint-plugin-react-hooks v7's react-hooks/set-state-in-effect rule (error, not warning) rejects a direct setState call inside a useEffect body unless it happens inside a timer/subscription callback — the fix is React's "adjust state while rendering" pattern, not a bigger effect.
metadata:
  type: feedback
---

`eslint-plugin-react-hooks` is on `^7.0.1` in this repo (React-Compiler-era
rules), which ships `react-hooks/set-state-in-effect` as an **error**, not a
warning — it fails `--max-warnings=0`. Discovered building `SlotReel` and
`WidgetState`'s skeleton-reveal for #751 phase 3, both of which need to
detect "a prop changed" and start a CSS-timed roll/fade — the exact shape
`useCountUp` (the code being replaced) already had on **master**, unflagged
only because nobody had run this lint version against it yet (confirmed via
`git stash`: `shared/ui.tsx`'s old `useCountUp` already failed this same rule
on a clean master checkout, at `setValue(target)` inside its
`prefers-reduced-motion` branch).

**What trips it:** any `setState(...)` call written directly in a
`useEffect` callback body, i.e. not inside a `setTimeout`/event-listener
callback passed to something external. It does **not** matter whether the
call is guarded by an `if`, whether a ref is written next to it, or whether a
subscription (`setTimeout`) is set up later in the *same* code path — every
shape tried (nested `if`, ref-write-adjacent-to-setState mirroring an
existing passing file, merging two `setState` calls into one, moving the
ref-comparison into the `if` condition) still got flagged, as long as the
`setState` call itself was reached synchronously during the effect's own
execution.

**What does NOT trip it, confirmed working:**
1. `setState` called from *inside* a `setTimeout`/`addEventListener`
   callback — the rule's own message literally endorses this ("calling
   setState in a callback function when external state changes").
2. `setState` called **during render**, in the component body (not inside
   any hook), conditionally comparing the incoming prop to a **state**
   variable seeded via `useState(prop)` (not a ref) — React's own documented
   "adjust state while rendering" pattern. React re-renders immediately,
   before commit, with no extra effect tick. Example shape:
   ```tsx
   const [renderedValue, setRenderedValue] = useState(value);
   if (value !== renderedValue) {
     setRenderedValue(value);
     setOtherState(...); // fine — this line is NOT inside useEffect
   }
   ```
   A `useEffect` is still fine (and necessary) for the *settle-back-later*
   half of a "start now, stop after N ms" pattern, as long as its own
   `setState` calls live inside the `setTimeout` callback, never in the
   effect body itself.

**Why:** the rule is trying to push "derive from a prop, don't effect off
it" — a `useRef`-based previous-value comparison living inside a `useEffect`
looks, to its static analysis, indistinguishable from a case that should
just be computed at render time, even when the *intent* is legitimate
(kick off a timed CSS state machine on a real value change).

**How to apply:** whenever a component needs to react to a prop changing —
before reaching for `useEffect` + `useRef(prevValue)` + `setState` — reach
for `useState(prop)` + a render-time `if (prop !== state) { setState(...); ... }`
comparison instead. Reserve `useEffect` purely for the async half
(`setTimeout`/subscription), and make sure every `setState` inside it is
nested inside that callback, never a bare statement in the effect body. This
applies anywhere in this codebase doing the "detect a value change, animate,
settle later" shape — not just SlotReel/WidgetState.
