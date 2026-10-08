---
name: matchmedia-spy-restore-trap
description: "vi.spyOn(window, 'matchMedia').mockRestore() breaks every later test in the same file — the setup.ts base mock is already a vi.fn().mockImplementation(), and restoring a spy wrapped around it clears the implementation instead of bringing it back."
metadata:
  type: feedback
---

`frontend/src/test/setup.ts` defines `window.matchMedia` via
`Object.defineProperty(window, 'matchMedia', { writable: true, value:
vi.fn().mockImplementation(...) })` — it is already a mock, globally, for
every test file. Spying on it with `vi.spyOn(window, 'matchMedia')` to
temporarily fake `prefers-reduced-motion: reduce`, then calling
`.mockRestore()` afterward, does NOT bring back the original
`matches: false` behaviour. It leaves `window.matchMedia` as a bare `vi.fn()`
with no implementation, so every later call in the same test file returns
`undefined` — and code that does `window.matchMedia(query).matches` throws
`Cannot read properties of undefined (reading 'matches')`, usually inside
some unrelated LATER test, not the one that did the spying. Confirmed with a
minimal repro (spy inside test A, restore, plain call in test B — B gets
`undefined`) while building #751 phase 5's Modal/Drawer reduced-motion tests.

**Why:** `mockRestore()`'s contract is "remove the mock implementation,
restore the original (non-mocked) function." When the function being spied
on is ITSELF already a `vi.fn().mockImplementation(...)`, there is no
separate "original" to restore to underneath — restoring clears the only
implementation that existed.

**How to apply:** to temporarily change `window.matchMedia`'s behaviour in a
test (e.g. to simulate reduced motion), save the current reference in a
plain variable and reassign it directly:
```ts
const original = window.matchMedia;
window.matchMedia = vi.fn().mockImplementation((q) => ({ matches: q.includes('reduce'), ... }));
try { /* test */ } finally { window.matchMedia = original; }
```
Never `vi.spyOn(window, 'matchMedia').mockRestore()` against this repo's
setup mock specifically. This also generalizes: before relying on
`mockRestore()` for ANY global that a project's own test setup already mocks
(not just matchMedia), check whether that setup mock is itself a
`vi.fn().mockImplementation(...)` — if so, restore by direct reassignment
instead.
