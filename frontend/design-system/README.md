<!-- Copyright (c) 2026 OpenDefender Contributors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# The canonical OpenRisk design system — vendored

`openrisk.tokens.css` in this directory is a **verbatim copy** of the canonical
design-system file authored in the `opendefender-website` repository at
`design-system/openrisk.tokens.css`. It is the value contract for both surfaces:
the OpenRisk console and the OpenDefender marketing site.

It is not imported at build time. The product's own token files
(`src/styles/primitives.css`, `tokens.css`, `components.css`) are what ship,
because the product carries three things the canonical file does not and should
not: the azure/iris accent variants, the `--select-caret` glyph, and ~2000 call
sites' worth of legacy aliases.

So the copy exists to be **compared against**, not consumed:

```
npm run check:design-system
```

fails when a token the canonical file declares has a different value in the
product. Every deliberate difference is listed in `scripts/check-design-system.mjs`
with the reason it exists — an unlisted difference is drift, and drift is what
produced the split this contract was written to end.

Two kinds of deliberate difference are recorded there. `ALLOWED` is a different
_value_; `RENAMED` is a different _name_ for the same value, which is what the
console needs for the text-colour roles: the canonical file calls them
`--text-primary|secondary|muted|inverse|on-solid`, and in Tailwind v4 `--text-*`
is the font-size namespace, so the console declares them as `--fg-*` (#442). The
values are still compared through the rename.

## Updating

The canonical file is upstream. To take a change:

1. Land it in `opendefender-website/design-system/openrisk.tokens.css`.
2. Copy the file here verbatim.
3. Apply the same value in the product's own token files.
4. `npm run check:design-system && node scripts/check-contrast.mjs`.

Never edit this copy to make the check pass. That inverts the direction of the
contract and the next person to sync from upstream will silently revert you.

**One exception, and it is the header.** Since D-014/D-016 this copy is
`Apache-2.0` (see `LICENSE` and `NOTICE` next to it), while the upstream file in
`opendefender-website` still carries the AGPL header it was written with. A
verbatim sync from upstream will therefore re-import the wrong
`SPDX-License-Identifier` — **keep the `Apache-2.0` header when you copy**, and
diff the body only. The proper fix is upstream: the same relicensing has to be
applied to `opendefender-website/design-system/openrisk.tokens.css`, which is a
separate repository and a separate issue owned by `website-dev`. Until that
lands, one file exists under two licences and this paragraph is the reason.

## Licence

This directory and `frontend/src/shared/ds/` are **Apache-2.0** — the exception
to the AGPL core. `LICENSE` and `NOTICE` here cover both directories;
`shared/ds/` has none of its own. See `LICENSING.md` at the repository root.

The canonical Tailwind theme is deliberately **not** vendored here: since the
Tailwind v4 migration (#441) the product's `frontend/src/styles/theme.css` — a
`@theme` / `@theme inline` pair, no `tailwind.config.js` — has to carry
the legacy aliases and the accent property-fork, so it cannot be a thin consumer
of the preset. It implements the same names; the token values are what this
check guards.
