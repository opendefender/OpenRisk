<!-- Copyright (c) 2026 OpenDefender Contributors
     SPDX-License-Identifier: AGPL-3.0-only -->

# W1-01 — Visual debt audit (pre-change inventory)

Taken on branch `feat/w1-01-openrisk-design-system` before any change, so the
"after" numbers in `docs/W1-01_OPENRISK_DESIGN_SYSTEM.md` can be compared
against something real rather than against a claim.

Method: `grep`/`rg` sweeps over `frontend/src` (243 `.tsx` files), plus reading
`src/styles/tokens.css`, `src/index.css`, `tailwind.config.js`,
`eslint.config.js` and every file under `src/shared/` and `src/components/ui/`.

## 1. What already exists and is good

The colour layer is genuinely solid and must not be thrown away:

| Asset | State |
| --- | --- |
| `src/styles/tokens.css` | Semantic colour tokens for both themes, applied via `[data-theme]` on `<html>` |
| `scripts/check-contrast.mjs` | 68 token pairs verified against WCAG AA in both themes — **0 failing at baseline** |
| `eslint-rules/no-raw-colors.js` | Error-level guard against raw hex, with a shrink-only ignore list of 76 legacy files |
| `e2e/visual/` | Playwright visual-regression harness + axe, snapshots per theme |
| `--den-*` density tokens | `comfortable` / `compact` / `spacious` already drive `.or-table` |

W1-01 therefore is **not** "add tokens". It is: finish the layers that were
never built, make the tokens reachable from components, and delete the second
(and third) way of building a button.

## 2. Findings

### F1 — Tokens are defined twice, with conflicting values

`--radius-*` is declared in `styles/tokens.css` **and** re-declared in
`index.css`. `index.css` imports `tokens.css` at the top, so the later block
silently wins:

| Token | `tokens.css` says | `index.css` says | Actually applied |
| --- | --- | --- | --- |
| `--radius-sm` | `6px` | `7px` | `7px` |
| `--radius-xl` | `20px` | `18px` | `18px` |
| `--radius-full` | `9999px` | *(absent)* | `9999px` |

Two files claiming to be the source of truth for the same token is the exact
failure mode a token system exists to prevent.

### F2 — Three token layers are declared but have zero consumers

| Layer | Declared in | Usages in `src/**/*.tsx` |
| --- | --- | --- |
| Type scale `--text-2xs … --text-3xl` | `index.css` | **0** |
| Spacing `--space-1 … --space-12` | `index.css` | **0** |
| Elevation `--elev-1 … --elev-4` | `index.css` | **0** |

Meanwhile `--shadow-*` (the *other* elevation scale, in `tokens.css`) has 21
usages. So elevation has two scales, one of which is dead.

### F3 — Visual constants are written inline, at scale

| Ad-hoc pattern | Occurrences |
| --- | --- |
| `text-[13px]`-style arbitrary font sizes | **1559** |
| `rounded-[10px]`-style arbitrary radii | **471** |
| `style={{ … }}` inline style objects | **1464** |
| Raw hex colours (in 15 files, all on the ESLint ignore list) | 119 |

### F4 — No z-index scale: stacking order is guessed per file

Values found in use: `59, 60, 65, 70, 75, 80, 85, 90, 95, 120` (arbitrary), plus
Tailwind `z-10/20/40/50`. `InfoHint` uses `z-[59]` and `z-[60]` next to each
other; the DataTable menus jump to `120` to escape everything else. Nothing
records what layer a number means, so every new overlay picks a bigger number.

### F5 — Two parallel primitive families, plus raw elements everywhere

| Family | Components | Importers |
| --- | --- | --- |
| `src/components/ui/` | `Button`, `Input`, `Badge`, `Drawer` | 33 files |
| `src/shared/ui.tsx` | `Btn`, `Chip`, `IconBtn`, `Card`, `Skeleton`, `ErrorState` | most feature screens |
| raw elements | — | **512** `<button>`, **181** `<input>`, **84** `<select>` |

They disagree on everything: `Btn` is `h-9 / 13px / rounded-[10px]`,
`components/ui/Button` is `py-2 / text-sm / rounded-lg`.

### F6 — `components/ui/Badge` is a copied shadcn component with no system behind it

Its variants reference Tailwind classes that **do not exist in this project's
config**: `text-primary-foreground`, `bg-secondary`, `bg-destructive`,
`text-destructive-foreground`, `ring-ring`, `text-foreground`. Those classes
compile to nothing, so `variant="destructive"` renders with no background and
inherited colour — the intent is invisible. This is the §60 failure mode in the
brief: a library component pasted in without its design system.

### F7 — Button primary fails contrast in White mode

`components/ui/Button` primary is `bg-primary` + `text-text-primary`. In light
theme that is `#161618` on `#1f62e0` ≈ **3.1:1** — below AA. It passes in dark
only because `--text-primary` happens to be near-white there. The token
`--text-on-solid` exists for exactly this and is not used.

### F8 — `Btn` has the API the brief calls out as an anti-pattern

`Btn({ primary?: boolean; danger?: boolean })` — magic booleans instead of
`variant`. It also has no `loading` state, no size, and no `tertiary`, so
screens that need those build their own `<button>` (see F3/F5).

### F9 — Decorative effects that read as "generic SaaS template"

- `Btn` primary is a `linear-gradient(135deg, …)` with `box-shadow: 0 3px 12px var(--accent-glow)` — a glow on the most-used control in the product.
- `tailwind.config.js` ships `glow`, `glow-lg`, `glow-red`, `glow-orange`, `neon-glow` (a `text-shadow` pulse), `or-float`.
- `Avatar` fills with a 135° accent gradient.

### F10 — Motion vocabulary is partly tokenised, partly not

`--dur-fast/base/slow` and `--ease-out/in/inout` exist and are used by `.or-table`
and the DataTable, but component-level animation is written as string literals
(`animation: 'or-fadeup .35s ease'`), and Tailwind `duration-200/300/500` appears
alongside them. There is no enter/exit pairing and no `prefers-reduced-motion`
story beyond one global `* { animation: none !important }` kill switch.

### F11 — No chart or graph colour contract

Recharts series colours are chosen per chart file. `FwBadge` hardcodes a
framework→hex map (`ISO27001: '#7c6cff'`, `NIST: '#0a84ff'`, …). There is no
categorical palette, so two charts on the same screen can use the same hue for
different meanings.

## 3. Baseline measurements (for the after-comparison)

```
tsc -b                        clean
eslint .                      360 problems (341 errors, 19 warnings)  [pre-existing]
node scripts/check-contrast.mjs   68 pairs checked, 0 failing
no-raw-colors ignore list     76 files
```

## 4. What W1-01 must therefore deliver

1. One canonical token file; delete the duplicate declarations (F1).
2. Make the dead layers reachable — as Tailwind utilities, not just CSS vars (F2, F3).
3. A z-index scale with named layers (F4).
4. One primitive family; the other becomes a thin compatibility shim, then shrinks (F5, F8).
5. Fix the two real defects found by the audit (F6, F7).
6. Remove the decorative glow/gradient vocabulary from the core controls (F9).
7. Finish the motion layer: enter/exit pairs, reduced-motion that degrades rather than deletes (F10).
8. A visualisation palette shared by every chart and the topology graph (F11).
