---
name: openrisk-frontend-charter
description: OpenRisk frontend engineering charter — feature structure, Zustand patterns, the three UI states, modal layout, switch defaults, i18n, accessibility gates and the frontend definition of done. Load before implementing or reviewing React/TypeScript code.
---

# OpenRisk Frontend Charter

## Structure

```
/src/features/[module]/    pages, components, hooks, stores for that feature
/src/shared/               design system, global hooks, utils, primitives
/src/services/             typed API client generated from OpenAPI
/src/locales/              fr.json, en.json — both always
```

## The crash patterns this codebase has already shipped — never repeat them

1. **Switch without `default` on a union.** `RiskBadge` and `StatusDot` crashed
   the app to a white page on an unexpected casing. Every switch on a string
   union has a `default` and normalizes its input first.
2. **Store-wide dependency inside a loader.** A loader depending on the whole
   store while calling a setter inside it = infinite render loop. Use stable
   selectors, always.
3. **Unstable callbacks in a connect effect.** Inline callbacks in the deps of
   an SSE/WebSocket `connect` recreate it every render. Put them in refs.
4. **Modal centred without `max-h`.** The submit button falls below the fold.
   Every modal: `max-h-[90vh]`, flex column, fixed header, scrollable body,
   pinned footer. Verify at 600px viewport height.
5. **Toast storms on a missing endpoint.** A failing stream logs in dev; it
   never raises a user-facing toast per retry.

## Shared primitives — reuse, do not recreate

`useSoftDelete<T>` (hide locally, 5s undo toast, API call fires after the
window) · `DangerConfirm` (impact readout + safer alternative, for vital
irreversible actions) · `InfoHint` (progressive disclosure tooltip) ·
`EmptyState` (icon + title + sub + **actionable CTA**, never a dead end) ·
`GlobalShortcuts` · `CommandPalette` · the single `<DataTable>`.

Routine content deletion → soft delete + undo.
Vital irreversible action → `DangerConfirm` with an impact breakdown.
Never a bare `window.confirm` in new code.

## Mandatory per view

Loading (skeleton, never a full-page spinner) · error (actionable copy) ·
empty (CTA). Optimistic updates on critical mutations. Zod on every form.

## Accessibility gates — merge blockers

Keyboard-operable end to end · focus always visible · focus never trapped ·
Escape closes every overlay and returns focus to its trigger · severity never
encoded by color alone · contrast 4.5:1 text and 3:1 UI · no keystroke
hijacking when an input, textarea, select or contenteditable has focus.

## Definition of Done — frontend

1. `npx tsc -b` clean
2. `npx vite build` clean
3. `npx eslint . --max-warnings=0`
4. FR + EN keys present in `/src/locales`
5. Three UI states implemented
6. axe-core pass on the touched screens, zero serious/critical
7. Verified at 414px and at 600px viewport height
