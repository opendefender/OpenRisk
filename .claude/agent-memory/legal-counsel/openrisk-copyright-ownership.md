---
name: openrisk-copyright-ownership
description: OpenRisk has exactly one human author and zero Signed-off-by trailers, so OpenDefender holds all copyright and may relicense any file freely
metadata:
  type: project
---

As of 2026-09-01, OpenRisk has **no external contributors at all**. Verified, not assumed:

- `git log --format='%an <%ae>' | sort | uniq -c` → three identities, all OpenDefender:
  `alex-dembele` and `Alex DEMBELE` (same person, same email) and
  `OpenDefender Security Team <opendefender@github.com>` (4 commits).
- `git log --format='%B' | grep -c '^Signed-off-by:'` → **0**. Nobody has ever accepted the CLA,
  because nobody outside has ever submitted.
- A handful of commits carry `Co-authored-by: Claude ...` or `Co-authored-by: Copilot` trailers.
  These are **not** copyright claims — an AI tool is not an author under US or EU law.

**Why:** every licensing question in this repo (relicensing, dual-licensing, the AGPL-core /
commercial-EE structure, EE files importing AGPL files) reduces to "may the owner do this?" — and
the answer is yes, because the owner owns all of it. `CLA.md` §2 preserves the multi-licensing
right for whenever a first external contribution does arrive.

**How to apply:** re-run the two commands above before signing off on anything that changes a
licence — the fact is only true until the first outside PR merges, and that is precisely the event
that makes it stop being true. Do not assert ownership from this memory alone.

Two structural exposures that are NOT solved by ownership, because an auditor cannot see ownership
from the repo:
- EE (`LicenseRef-OpenRisk-Commercial`) files import AGPL files outside the design system —
  `features/automation/*` and `features/infrastructure/*` import `shared/ui.tsx`. Legal, but it
  gets raised. `LICENSING.md` should say why once.
- Phase 2 of `LICENSING.md` (moving EE under `ee/` behind a build tag) is unstarted, so a default
  build still compiles EE code into a Community binary.

Related: [[website-repo-license-gap]] · [[license-ci-gates-are-theatre]]
