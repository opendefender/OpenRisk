---
name: defect-pattern-bundle-splitting
description: What breaks when frontend code moves out of the entry chunk (lazy i18n catalogues, lazy shell pieces, sideEffects) — found reviewing #966, 2026-10-14
metadata:
  type: project
---

Reviewing #966 (initial JS budget 180 KB gzip, measured by scripts/check-bundle-budget.mjs
from index.html refs only) surfaced these recurring shapes:

1. **Locale chosen outside uiStore.** Lazy catalogues (only fr.json eager since #966) are
   loaded by `setLang`/main.tsx. Any screen that picks its own locale and calls
   `translate(catalogs, lang, …)` directly (VendorQuestionnairePage: `view.language` +
   local `chosenLang`) gets French for `en`. Grep `translate(catalogs` and `useState<LocaleCode`.
2. **Test setup masks it.** src/test/setup.ts preloads every catalogue, so no vitest can
   see an unloaded-catalogue render. Such a bug needs a `vi.resetModules()` test.
3. **Only one ErrorBoundary (root, in main.tsx).** Every `lazy()` piece in the shell
   (bell panel, tour, `?` overlay) turns a chunk-load failure into a whole-app error
   screen. Lazy shell pieces need a local null-fallback boundary.
4. **Budget blind spot.** The budget counts the FR critical path. An EN user's blocking
   catalogue fetch (en chunk, ~18 KB gz, serial before render) is invisible to it.
5. **fr.json is ~22 KB gz of the entry and grows with every feature.** Budget headroom
   is eaten by translations; namespace-splitting the catalogue is the structural fix.

`"sideEffects": ["*.css"]` was checked and safe at the time: no bare non-CSS imports in
src; module-level registrations (registerTenantStore, motion.ts MotionGlobalConfig,
useBackTo Escape listener, api interceptors) all live in modules whose exports are
used by their consumers. Re-check whenever a new top-level side effect is added.

Remedies that landed on the #966 branch (verify on master before citing): `useCatalog(lang)`
in hooks/useI18n.ts (screens picking their own locale must call it), a catalogue version
counter so a late load re-renders, and `shared/system/QuietBoundary` for optional lazy shell
pieces. Route content (NotFound/AccessDenied) deliberately stays on the root boundary.
ProductTour stays mounted every boot because it owns the `openrisk:tour` replay listener.
English modulepreload (point 4) was documented, not built.

**How to apply:** on any PR that moves code out of the preload, run the five checks above.
Related: [[defect-pattern-tenant-isolation]].
