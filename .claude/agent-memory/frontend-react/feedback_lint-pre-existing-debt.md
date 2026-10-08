---
name: lint-pre-existing-debt
description: Files like CreateRiskModal.tsx and EditRiskModal.tsx fail eslint --max-warnings=0 even on master (unused import, pre-existing `any`) — do not fix unrelated pre-existing errors while passing through on an unrelated feature.
metadata:
  type: feedback
---

`frontend/src/features/risks/CreateRiskModal.tsx` (unused `useMemo` import)
and `frontend/src/features/risks/components/EditRiskModal.tsx` (three
pre-existing `any` — `risk: any`, `(data: any) =>`, an untyped asset map
callback) fail `npx eslint <file> --max-warnings=0` on a clean `master`
checkout, confirmed via `git stash` before touching either file during #751
phase 2.

**Why:** the repo's constitution says zero `any` and eslint-clean, but also
"smallest correct change, no opportunistic refactors" and "never merge a PR
that isn't yours to redesign." Fixing `EditRiskModal`'s `risk: any` requires
knowing the real `Risk` shape used across many call sites — a bigger,
unrelated change than whatever feature touches that file.

**How to apply:** when a file you need to edit for an unrelated feature
already fails lint/tsc on master, confirm that with `git stash` (or diff
against master) before assuming you broke something, then leave the
pre-existing errors alone and say so explicitly in the handback report
(cite the exact `git stash` comparison) rather than silently fixing or
silently ignoring them. Do not let pre-existing debt block or expand an
unrelated commit.
