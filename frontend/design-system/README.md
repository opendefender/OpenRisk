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

## Updating

The canonical file is upstream. To take a change:

1. Land it in `opendefender-website/design-system/openrisk.tokens.css`.
2. Copy the file here verbatim.
3. Apply the same value in the product's own token files.
4. `npm run check:design-system && node scripts/check-contrast.mjs`.

Never edit this copy to make the check pass. That inverts the direction of the
contract and the next person to sync from upstream will silently revert you.

The canonical Tailwind preset (`design-system/tailwind.preset.js` upstream) is
deliberately **not** vendored: the product's `tailwind.config.js` has to carry
the legacy aliases and the accent property-fork, so it cannot be a thin consumer
of the preset. It implements the same names; the token values are what this
check guards.
