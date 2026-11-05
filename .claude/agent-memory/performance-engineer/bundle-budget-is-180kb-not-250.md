---
name: bundle-budget-is-180kb-not-250
description: The real blocking CI bundle budget is 180 KB gzip, not the 250 KB ceiling named in CLAUDE.md
metadata:
  type: project
---

CLAUDE.md states the JS bundle budget as "< 250KB gzipped" but that number is
the historical, never-relaxed ceiling. The actual blocking gate today is
`frontend/scripts/check-bundle-budget.mjs`'s default `BUDGET_KB = 180`, wired
into `.github/workflows/ci.yml` in two places (lines ~320 and ~513, both run
`npm run budget` with no `BUNDLE_BUDGET_KB` override). D-018 in
`docs/DECISIONS.md` (2026-09-01) ratcheted 250 → 248 → the dedicated-issue
target of 180, and that 180 was hit by #462/#758 and became the standing
default. Measured 2026-10-30 on a real build: 161.9 KB, 18.1 KB headroom.

**Why:** Citing 250 KB as "the budget" in a report would understate how close
the project actually is to its real ceiling, and would miss that the gate is
already stricter than the constitution's own number.

**How to apply:** When auditing or discussing bundle size, always read
`check-bundle-budget.mjs`'s `BUDGET_KB` default and confirm what the CI
workflow actually passes as `BUNDLE_BUDGET_KB` before quoting a number — don't
take CLAUDE.md's figure at face value. The script itself can be run standalone
with plain `node` (no npm install) against any copied `dist/` to re-measure.
