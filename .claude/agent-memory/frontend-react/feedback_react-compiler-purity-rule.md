---
name: react-compiler-purity-rule
description: eslint-plugin-react-hooks v7's `react-hooks/purity` rule (error) rejects calling an impure function — Date.now(), Math.random(), etc — directly in a component's render body, even inside a render-time "adjust state while rendering" conditional; the fix is to capture the impure read inside a useEffect (via a ref, not state) and let render stay pure.
metadata:
  type: feedback
---

Building #751 phase 4's OfflineBanner "hold visible for at least 2000ms"
logic, `eslint-plugin-react-hooks` v7 flagged `setOpenedAt(Date.now())` —
called inside a render-time `if (wants && !held) { setHeld(true);
setOpenedAt(Date.now()); }` block, the exact "adjust state while rendering"
shape documented in [[feedback_set-state-in-effect-lint]] — with `Error:
Cannot call impure function during render` / `react-hooks/purity`. This is a
**different** rule from `set-state-in-effect`: it fires on the *impure call
itself* (`Date.now()`), not on where a `setState` happens to sit. A lazy
`useState(() => Date.now())` initializer was NOT flagged (React treats a lazy
initializer as running once, outside the "render must be pure/repeatable"
contract); the same call inlined into a later conditional in the render body
WAS flagged.

**Fix:** move the impure read into a `useEffect` (effects are explicitly
allowed to be impure — that is what they are for), and store the captured
value in a `useRef` rather than `useState`, since a plain state-setting
`useEffect` body would then trip `set-state-in-effect` instead. Shape used:
```tsx
const [held, setHeld] = useState(wants);
if (wants && !held) setHeld(true); // pure: no Date.now() here anymore

const openedAtRef = useRef(0);
useEffect(() => {
  if (wants) openedAtRef.current = Date.now(); // impure read, inside an effect: fine
}, [wants]);

useEffect(() => {
  if (wants || !held) return;
  const remaining = Math.max(0, MIN_MS - (Date.now() - openedAtRef.current));
  const timer = setTimeout(() => setHeld(false), remaining);
  return () => clearTimeout(timer);
}, [wants, held]);
```

**Why:** the React Compiler's purity requirement is that render must be
idempotent/repeatable for the same props+state — a value that changes just
from being *called again* (Date.now, Math.random, reading a mutable global)
breaks that even if the call site is otherwise a legitimate state-adjustment.

**How to apply:** whenever a "detect a condition became true, timestamp it,
settle back later" pattern (this exact shape recurs — see
[[feedback_set-state-in-effect-lint]] for the sibling rule) needs a captured
`Date.now()`/similar, do NOT call it directly in the render body even inside
an already-legitimate render-time state-adjustment conditional. Capture it in
a `useEffect` + `useRef`, never `useState`, and reserve the lazy
`useState(() => ...)` initializer form for true one-time-at-mount values only.
