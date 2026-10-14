---
name: website-repo-license-gap
description: opendefender-website is a sibling repo at /home/alex/Documents/projects/opendefender-website; it holds the canonical design tokens and has NO LICENSE file at all
metadata:
  type: reference
---

The marketing site repo `opendefender-website` is checked out at
`/home/alex/Documents/projects/opendefender-website` on this machine. It is the **upstream** for
`frontend/design-system/openrisk.tokens.css` in OpenRisk — the OpenRisk copy is vendored,
byte-identical below its 2-line header.

Two things to know before touching anything design-system-licensing related:

- That repo has **no `LICENSE` file** and **no `license` field in `package.json`**. Its source
  files carry `SPDX-License-Identifier: AGPL-3.0-only` headers with nothing behind them. Under
  default copyright it is all-rights-reserved.
- Sole author, `Alexandre Dembele <alexandredembele16@gmail.com>` — the same person as OpenRisk's
  sole author, so common ownership. Relicensing either copy is within OpenDefender's rights.

As of 2026-09-01 the OpenRisk copy is `Apache-2.0` (D-014/D-016, issue #452) while the upstream
copy — and `design-system/tailwind.preset.js` beside it — still say `AGPL-3.0-only`. Same bytes,
two declarations. Not a violation, but an auditor with both repos open sees a contradiction.
Fixing it upstream is `website-dev`'s, in that repo, and had no issue as of that date.

Related: [[openrisk-copyright-ownership]]
