<!-- Copyright (c) 2026 OpenDefender Contributors
     SPDX-License-Identifier: BUSL-1.1 -->

# W1-01 — OpenRisk Design System

The reference for building an OpenRisk screen. The test it has to pass: a new
screen should be buildable from `src/shared/ds` alone — no new colour, no new
spacing step, no new button, no new animation.

Companion documents:

- `docs/ui/W1-01_VISUAL_DEBT_AUDIT.md` — the measured pre-change state
- `docs/W1-01_DESIGN_SYSTEM_LIVE_PROOF.md` — what was actually run and what it returned

---

## Design Philosophy

**An instrument, not a dashboard.**

OpenRisk is used for hours at a time by people whose job is to look at a lot of
data and decide what to do about it. Everything below follows from that:

1. **Density is a feature.** The base font size is 14px, not 16px. A risk
   register, a control table and a vulnerability queue all have to put a usable
   number of rows on screen. Airy spacing is a landing-page value.
2. **Hierarchy comes from rules and surface steps, not shadows.** A shadow means
   "this floats above the page and can be dismissed". Anything that cannot be
   dismissed does not get one. This is why there are three border tokens and
   only four elevation steps.
3. **Nothing decorative moves.** Motion exists for feedback, orientation and
   continuity. There are no ambient animations, no glows, no pulses except the
   one that marks a live critical state.
4. **Colour means something or it is neutral.** The semantic palette is reserved
   for verdicts. A chart of assets per environment is not a verdict, so it uses
   the categorical ramp — reusing red/amber/green there makes a neutral chart
   read as an alert.
5. **The interface never lies.** A control that looks pressable is pressable; a
   badge that looks like a status is one; an empty region says which of the four
   reasons it is empty for.

## Visual Direction

### The neutral is warm, and it is true

Both ramps are true neutrals, very slightly warm: they read as paper and
graphite rather than as a blue screen. It is the quietest decision in the system
and it does the most work — it is what makes an OpenRisk surface recognisable
without a logo on it, and it costs nothing at render time.

This replaces the cool ~228° ramp W1-01 originally shipped. That ramp was
defensible on its own terms, but it put the accent, the risk scale and every
chart series on a field that was already competing with them, and it was not
what the canonical design system specifies. See "The canonical contract" below.

### The keyline

The signature device is a 2px accent rule marking the active element — the
selected nav item, the active tab, the focused panel (`--keyline-w`). It
replaces the filled-pill / gradient / glow vocabulary that makes most dashboards
interchangeable, and it costs one border instead of a background.

### What was deliberately removed

| Removed | Why |
| --- | --- |
| `linear-gradient(135deg, …)` on the primary button | The strongest "generic template" signal a UI can send, on the most-used control |
| `0 3px 12px var(--accent-glow)` under the same button | Decoration on a control, in a product where a glow should mean something |
| `glow`, `glow-lg`, `glow-red`, `glow-orange`, `neon-glow`, `or-float` utilities | Five ways to make something shimmer; zero product meanings for it |
| The accent gradient on `Avatar` | Put brand decoration into every row of every table showing an owner |
| 20px backdrop blur on overlays | Reduced to 4px: a full-screen filter pass per frame, for an effect the scrim already achieves |

## Brand Principles

The mark is a möbius ring in blue→violet on deep navy, with the line
*SEE · UNDERSTAND · ACT*. The system takes three things from it and leaves the
rest alone:

- the cool navy field, generalised into the neutral ramp;
- blue as the accent, violet as the second visualisation hue;
- the tagline as an ordering principle — **see** (the data is legible),
  **understand** (hierarchy makes the important thing obvious), **act** (the
  next step is always a real control).

The gradient in the logo stays in the logo. It is an identity mark, not a UI
texture.

---

## Design Tokens

Four layers. Each value is defined exactly once.

```
src/styles/primitives.css   raw scales, theme-independent
        ↓
src/styles/tokens.css       semantic colour + elevation, per theme
        ↓
src/styles/components.css   component-level tokens
        ↓
src/shared/ds/*             primitives → screens
```

`tailwind.config.js` is the delivery mechanism: every utility resolves to one of
these variables, which is what makes a component follow the theme with no
per-component work.

### Colors

Defined in `tokens.css`, per theme. Components name a **role**, never a colour.

| Role | Tokens |
| --- | --- |
| Surface ramp | `--surface-0` canvas · `--surface-1` cards · `--surface-2` raised · `--surface-3` hover-on-raised · `--surface-sunken` · `--surface-overlay` scrim |
| Text | `--text-primary` · `--text-secondary` · `--text-muted` · `--text-inverse` · `--text-on-solid` |
| Border | `--border-subtle` divider · `--border-default` · `--border-strong` · `--border-control` (the edge that identifies a control, ≥3:1) |
| Accent | `--accent` the MARK (rules, keylines, fills; 3:1) · `--accent-500` the TEXT step, exposed as `accent-strong` and as `text-accent` (4.5:1 on every surface) · `--accent-hover` · `--accent-soft`/`-line` tints · `--accent-solid` for fills. Live per theme AND per variant |
| Semantic | `--success` / `--warning` / `--danger` / `--info`, each with `-surface` and `-text` |
| Solid fills | `--danger-solid` · `--success-solid` · `--warning-solid` · `--info-solid` · `--accent-solid` — identical in both themes, so a destructive button does not change meaning when the theme flips |
| Risk scale | `--risk-low` · `--risk-moderate` · `--risk-high` · `--risk-critical` · `--risk-extreme` |
| Visualisation | `--chart-1…8` categorical · `--chart-grid/axis/label/track` · `--graph-node/-stroke/edge/edge-active/dimmed` |
| Focus | `--focus-ring-color` (theme-scoped) · `--focus-ring-width` · `--focus-ring-offset` |

Two distinctions that are easy to get wrong:

- **`--danger` vs `--danger-solid`.** The first is tuned to be readable *as text*
  on a neutral surface, which makes it too light to sit *under* text. Use
  `-solid` for a fill.
- **`--border-strong` vs `--border-control`.** The first is the strongest
  decorative rule; the second is the boundary of an actual control and meets
  WCAG 1.4.11's 3:1.

### Typography

Base 14px, ratio ≈1.2. Every step carries its line height, so `text-sm` is a
complete typographic decision rather than half of one.

| Token | Size | Used for |
| --- | --- | --- |
| `--text-2xs` | 10.5px | Table headers, badges, metadata, timestamps |
| `--text-xs` | 12px | Field labels, chips, secondary controls |
| `--text-sm` | 13px | Body copy, table cells, buttons |
| `--text-base` | 14px | The default; dense reading |
| `--text-md` | 16px | Card and dialog titles |
| `--text-lg` | 19px | Section headings |
| `--text-xl` | 23px | Page titles |
| `--text-2xl` | 28px | Screen titles |
| `--text-3xl` | 34px | The single largest number on a dashboard |

Weights: 400 / 500 / 600 / 700 — four, deliberately. Leading:
`none · tight 1.15 · snug 1.3 · normal 1.45 · relaxed 1.6`. Tracking:
`display -0.02em` on large sizes, `caps 0.06em` on uppercase micro-labels.

**Numeric data** — scores, CVSS, counts, percentages, currency, dates — is set
in `--font-mono` with `tabular-nums` so columns align and a changing value does
not reflow its neighbours.

### Spacing

4px grid: `0 · 1px · 2 · 4 · 6 · 8 · 12 · 16 · 20 · 24 · 32 · 40 · 48 · 64`.
Half-steps exist only below 8px, where 2px is a real distinction (icon-to-label)
and 6px is not reachable from the grid.

Tailwind's default spacing scale is the same 4px grid, so `p-3` *is*
`--space-3`. The violation to look for is `p-[13px]`, not `p-3`.

### Radius

`xs 4 · sm 6 · md 10 · lg 14 · xl 18 · full`. Capped at 18px: a 24px corner
reads as a phone app, a 0px corner reads as a spreadsheet, and OpenRisk sits
between the two on purpose.

### Elevation

Four steps plus an overlay, theme-aware — dark needs deeper, softer shadows
because a shadow on a dark surface has less luminance range to work with.

| Token | Use |
| --- | --- |
| `--elev-0` | Flat. Most things. |
| `--elev-1` | A card that needs separating from a busy canvas |
| `--elev-2` | Dropdown, popover, tooltip |
| `--elev-3` | Drawer |
| `--elev-overlay` | Modal |

`--shadow-sm/md/lg/overlay` are aliases of the same four values, kept for the 21
existing call sites.

### Motion

Five durations, four curves, five composed shorthands.

| Token | Value | Use |
| --- | --- | --- |
| `--dur-instant` | 90ms | Press feedback |
| `--dur-fast` | 120ms | Hover, colour changes |
| `--dur-base` | 180ms | Enter/exit of a small element, dialog |
| `--dur-slow` | 260ms | List and content reveals |
| `--dur-panel` | 400ms | A panel travelling the width of the screen |

`--ease-out` for anything entering or settling; `--ease-in` for anything leaving
(an element that accelerates away reads as dismissed rather than yanked);
`--ease-inout` for a two-way transition; `--ease-emphasized` — a slight
overshoot — reserved for the signature keyline move.

Composed: `--motion-hover`, `--motion-press`, `--motion-enter`, `--motion-exit`,
`--motion-panel`. A component writes one token instead of pairing a duration and
a curve by hand and getting a different pair each time.

Keyframes (`index.css`): `or-fadein`, `or-fadeup`, `or-rise` (dialog),
`or-scalein`, `or-slidein` / `or-slidein-left` (drawer), `or-shimmer`
(skeleton), `or-pulsedot` (live critical state), `or-shake` (rejected field).

### Breakpoints

`sm 640 · md 768 · lg 1024 · xl 1280 · 2xl 1536 · 3xl 1920`. Mirrored in
`--bp-*` so a `matchMedia` call reads the same numbers the utilities do. `3xl`
is the master-detail threshold: at 1920px a detail panel opens *beside* the list
rather than over it.

### Z-index

Named layers, replacing the ten arbitrary integers previously in use
(59, 60, 65, 70, 75, 80, 85, 90, 95, 120):

```
base 0 · raised 10 · sticky 20 · nav 30 · header 40 · dropdown 50
drawer 60 · modal 70 · popover 80 · toast 90 · tooltip 100
```

The ordering is the contract. The one rule that matters: **tooltip outranks
modal**, because a tooltip on a control inside a dialog has to be readable.

### Density

`--den-row`, `--den-cell-y`, `--den-gap`, `--den-font`, switched by
`data-density` on `<html>`: `comfortable` (default, 40px rows) · `compact`
(32px) · `spacious` (48px). A security analyst working a queue switches to
compact and gets ~25% more rows out of the same table component.

---

## Themes

Applied as `data-theme` on `<html>`, so everything inherits — including anything
rendered through a React portal into `document.body`. Switching is one attribute
write; measured style recalculation is **1.6ms p50** (see the live proof).

### White Mode

The canvas is deliberately **not** white: `--surface-0` is `#f6f4ef` and cards
are white, so a card reads as a card without needing a shadow to prove it.
Shadows are shallower and tighter than in dark — on a light canvas a large soft
shadow reads as blur, not as height. Semantic colours are darker than their
dark-theme counterparts so they stay readable on white, and their `-surface`
tints are very light.

### Dark Mode

A four-step ramp from `#111111`, each step visible against the one below it
without a border. Not black: `#000` removes the ability to go darker, which both
a sunken well and a scrim need. Shadows are deeper and softer. The scrim behind a
modal stays dark in *both* themes — a pale scrim reads as a rendering glitch.

### System Mode

`index.html` applies the theme in an inline script **before first paint**,
reading the persisted store, so there is no flash of the wrong theme. The store
supports `light` / `dark` / `system`; `system` follows `prefers-color-scheme`.

One hole was closed in W1-01: the bootstrap's `catch` path set `data-theme` but
not `data-variant`, and `--accent` was only defined in the variant blocks, so
that path left the accent undefined. Each theme block now defines a default
accent.

### Accent variants

`azure` and `iris`, chosen by the user in Settings, applied as `data-variant`.
They are a **product** feature and are not part of the canonical design system,
which declares one accent; they live below the contract in `tokens.css` and
override only the accent.

**Each has two definitions, one per theme.** They previously had one, which is
why every `text-accent` in the product measured 3.65:1 on white: a hue bright
enough to read on a dark surface is by construction too light to read on a pale
one.

Each variant also declares `--accent-500` alongside `--accent`. A variant that
set only the latter would leave every accent-coloured *label* in the product on
the default ultramarine while its rules and fills turned violet. Both variants
already clear 4.5:1 on every warm surface at their existing values, so the two
steps coincide for them — `check-contrast.mjs` is what proves that, per variant,
per theme, rather than the sentence you are reading.

### The canonical contract

The token *values* are not this repository's to choose. They are authored in
`opendefender-website/design-system/openrisk.tokens.css`, which is the single
source of truth for both surfaces — the console and the marketing site — and a
verbatim copy is vendored here at `frontend/design-system/openrisk.tokens.css`.

The product does not import it. It cannot: the console carries three things a
shared contract has no room for — the azure/iris accent variants, the
`--select-caret` glyph, and ~2000 call sites' worth of legacy aliases. So the
copy is **compared against** instead:

```
npm run check:tokens        # drift + contrast, both blocking in CI
```

`check-design-system.mjs` asserts that every token the canonical file declares
has the same value in the same scope here. Tokens the product adds on top are
its own business and are ignored. Six differences are deliberate and recorded in
that script with their reasons; an unlisted difference fails the build.

Two of those six are worth naming, because they are the only places this product
knowingly departs from the shared contract, and both depart *upward*:

| Token | Canonical | Here | Why |
| --- | --- | --- | --- |
| `--border-control` | `#6b6a66` / `#8a867a` | `#767570` / `#7c786d` | Canonical measures 2.87:1 and 2.92:1 against `--surface-3`, which is where a control sits while it is hovered. Raised to clear WCAG 1.4.11 on every surface a control can rest on. |
| `--graph-edge` | a decorative dim | `var(--border-control)` | In the console a graph edge *carries the topology* — it is the only thing saying which asset feeds which — so it is held to 3:1 like any other meaningful non-text mark. |

The direction of the contract matters: fix the product, or fix upstream and
re-vendor. Editing the vendored copy to make the check pass inverts it, and the
next person to sync from upstream silently reverts you.

---

## Component Primitives

All in `src/shared/ds`, exported from one barrel.

### Buttons

`variant`: `primary` · `secondary` (default) · `ghost` · `destructive` · `link`
`size`: `sm` 28px · `md` 36px · `lg` 44px
`loading` · `feedback: 'success' | 'error'` · `icon` · `iconPosition` · `fullWidth`

- One primary per view. Hierarchy is carried by fill: primary is the only filled
  accent, destructive the only filled red, everything else is a surface or
  nothing.
- `loading` disables and sets `aria-busy`, and **keeps the label** — a spinner
  that replaces the label leaves a screen-reader user with an unnamed button.
- An **icon-only button requires an accessible name in the type**: no children
  means `aria-label` is mandatory, checked by the compiler rather than by review.
- `type` defaults to `"button"`. A button in a form that should submit says so.

### Forms

`Field` owns the wiring; `Input`, `Textarea` and `Select` are just controls.

One generated id is threaded to the label, the description and the error.
`aria-describedby` carries the description *and* the message, in that order —
"what this is" before "why it failed" is the order that makes a rejection
understandable. `aria-invalid` on error, `role="alert"` so it is announced.
Required is marked **in the accessible name**, not only by a red asterisk.

States: `default` · `invalid` · `warning` · `success`, plus native `disabled`,
`readonly`, and a `loading` slot on `Input`.

`Select` is a native `<select>` on purpose: it gets the platform's keyboard
behaviour, mobile picker and screen-reader support for free.

### Badges

`intent`: `neutral` · `accent` · `success` · `warning` · `danger` · `info` ·
`experimental` · `unavailable`.

A badge takes an intent and nothing else. **The mapping from a business enum to
an intent lives in `badgeIntents.ts`**, as an exhaustive lookup table — so a new
enum value is a compile error rather than a silent fall-through to neutral, and
a reviewer can check the table against the domain without reading render code.

`experimental` is outlined rather than tinted: "not finished" is a different
*kind* of statement from "this is warning-severity". `unavailable` is the badge
that says "no data here" and must never be mistaken for a healthy zero.

### Tables

The `DataTable` in `src/shared/datatable` remains the table primitive (server and
client modes, virtualisation, URL-backed facets, saved filters, column
management, CSV export, full keyboard grid). W1-01 did not rewrite it; it
supplies the tokens it consumes — `--den-*` for row rhythm, `--table-*` for the
parts that do not change with density, and the named z-layers its portalled row
menu and column picker use.

### Charts

`src/shared/ds/chart.ts`. Exported as `var()` strings, so an SVG attribute
follows the theme with no re-render and no JS reading computed styles.

- `categorical[]` / `seriesColor(i)` — a **category**, wrapping rather than
  running out
- `severity` — a **verdict**; this one *is* the risk scale, so a "critical" bar
  is the same red as a "critical" badge three rows above it
- `trend` — positive / negative / neutral / track
- `chartAxis`, `chartGrid` (horizontal only), `chartTooltip`
- `chartAccessibleProps()` — forces the caller to state whether the chart carries
  meaning or is decorative beside a table that already gives the numbers

The first four categorical hues stay separable for the common 3–4 series case
including for the two most frequent colour-vision deficiencies, which is why
green and red are not adjacent. Light mode has its own ramp: the dark values
wash out on white, which is the half of chart theming that gets skipped.

### Graphs

`graph.node` · `nodeStroke` · `edge` · `edgeActive` · `dimmed`. Node severity
comes from the risk scale; these are the states that are *not* severity. Edges
and node strokes meet 3:1 because they carry the topology. Selection dims the
rest rather than hiding it, so the shape of the graph stays legible.

### Empty States

`shared/EmptyState` — `first-use` · `no-results` · `error` · `no-permission`,
with icon, title, description, primary and secondary action, and an optional
documentation link. An empty state says what to do next.

### Loading States

`SkeletonRows` (defaults to the current density's row height, so real rows land
where the placeholders were) · `Skeleton` · `LoadingState` (a spinner, only for
when the shape of what is coming is *not* known) · `Button loading`.

Prefer a skeleton whenever the layout is known: it reserves space, so nothing
jumps when data lands.

### Error States

`ErrorState` — title, description in the user's terms, `detail` in a collapsed
`<details>` (support asks for it; users ignore it), retry with its own loading
state, optional secondary action. `aria-live="assertive"`: a failure that
replaces content the user asked for should interrupt.

### Permission States

`PermissionDenied` — names the resource **and the permission that would grant
access**, which turns a dead end into something a user can ask an administrator
for.

### Audit States

`AuditEntry` — actor, action, target, timestamp. The actor and action are the
only things at full contrast; the timestamp is muted, monospaced and tabular so
the column aligns. An audit trail is scanned far more often than it is read.

### Modals

Header / scrolling body / **pinned footer**, capped at `--modal-max-h`. A dialog
laid out as one scrolling column puts its submit button below the fold on a
laptop, which this codebase has fixed by hand at least three times.

Sizes `sm 380 · md 520 · lg 720 · xl 960`. `dismissable={false}` while a
submission is in flight. Portalled to `document.body`, so no ancestor's
`overflow` or `transform` can clip it.

### Drawers

The detail counterpart: a modal *asks* something, a drawer *shows* something
that belongs to the list behind it. Slides from the edge it will return to,
because that continuity is what a modal's fade cannot express. `left` / `right`,
sizes `sm 380 · md 480 · lg 620 · xl 780`. Same dialog contract as Modal.

### Navigation

Sidebar, nav item, breadcrumbs, page header and command palette are the existing
components in `src/components/layout`; W1-01 supplies their tokens
(`--sidebar-w`, `--header-h`, `--keyline-w`, the `nav`/`header` z-layers) rather
than rewriting them.

### Tooltips

Opens on hover **and on keyboard focus**, survives the pointer travelling onto
it (`safePolygon`), dismisses on Escape (WCAG 1.4.13). It **describes** its
trigger via `aria-describedby` — it never becomes the trigger's name. Positioned
with floating-ui: flip, shift, offset. Not a place to put content: if the
interface needs a paragraph to be understandable, fix the interface.

### Tabs

Full WAI-ARIA pattern: roving tabindex (Tab enters the tablist once and moves
*on* to the panel), Left/Right with wrap, Home/End, activation follows focus.
The active tab is marked by the keyline. Only the active panel is rendered —
hiding with CSS leaves the other panels' content reachable by screen readers —
and only the active tab carries `aria-controls`, since the others' panels do not
exist.

`Tabs` takes an explicit `id` that every `TabPanel` repeats as `tabsId`. See
"Known Exceptions" for why that friction is deliberate.

---

## Interaction and Motion

Five principles, in priority order:

1. **Purpose.** Every animation is feedback, orientation, continuity or
   hierarchy. Nothing is decorative.
2. **Speed.** Anything done dozens of times an hour — hover, press, opening a
   menu — is on `--dur-fast` or `--dur-base`. Only a panel crossing the screen
   gets 400ms.
3. **Consistency.** Two components that do the same thing use the same token.
   That is what the composed shorthands are for.
4. **Restraint.** No infinite animations except `or-pulsedot`, which marks a live
   critical state and is therefore information.
5. **Accessibility.** `prefers-reduced-motion: reduce` disables animation and
   transitions globally. Entrances use `both` fill and never start from
   `opacity: 0` in a way that would leave content invisible once animation is
   off — that is asserted by a test.

Performance: transform and opacity only. No animation touches layout, so a table
of 200 rows with an action button each does not reflow on hover.

---

## Accessibility

Target: **WCAG 2.2 AA**.

| Criterion | How it is met | How it is verified |
| --- | --- | --- |
| 1.4.3 Contrast (text) | Token pairs tuned per theme | `check-contrast.mjs` (arithmetic) + axe (composited) |
| 1.4.11 Non-text contrast | `--border-control`, focus ring, chart series, graph edges all ≥3:1 | `check-contrast.mjs` |
| 1.4.1 Use of colour | Every badge carries text; links are underlined; required is in the accessible name | Unit tests |
| 2.1.1 / 2.1.2 Keyboard | Focus trap in overlays, roving tabindex in tabs, native controls elsewhere | Unit + Playwright |
| 2.4.3 Focus order | Focus moves into a layer on open and returns to the trigger on close | Unit test |
| 2.4.7 / 2.4.11 Focus visible | One `:focus-visible` outline, one token, never removed | Playwright asserts `outlineStyle !== none` |
| 2.5.8 Target size | Smallest control is 28px | Playwright measures every button at 390px |
| 3.3.1 Error identification | `aria-invalid` + `role="alert"` + text | Unit test |
| 4.1.2 Name, role, value | Dialogs labelled; icon-only buttons require a name *in the type* | Compiler + unit tests |
| 2.3.3 Animation from interactions | Global reduced-motion kill switch | Playwright with `reducedMotion: reduce` |

Contrast is checked twice, and the two are not redundant: the script verifies the
token **pairs** arithmetically, axe verifies what the browser actually
**composited** — the only way to catch a component putting a token on a surface
it was never meant for.

---

## Responsive Behaviour

Breakpoints above. The failure that matters for a data product is horizontal
overflow — one fixed-width control turns every screen into scroll-to-read, and it
is invisible on the desktop the author is using. Every gallery is checked for
overflow at 1440 / 834 / 390 px in both themes.

Existing responsive behaviour is unchanged: the sidebar becomes an off-canvas
drawer below `lg`, wide tables scroll inside `overflow-x-auto`, and at `3xl` the
detail panel docks beside the list.

---

## Product Screen Migration

**39 files** moved off `src/components/ui/*`, which is deleted. `shared/ui`'s
`Btn`, `IconBtn`, `Skeleton`, `SkeletonRows` and `ErrorState` are now adapters or
re-exports over the design system, which carries the ~100 screens that import
from there.

| Area | State |
| --- | --- |
| `src/components/ui/*` (Button, Input, Badge, Drawer) | **Deleted.** All 39 importers migrated |
| `shared/ui` Btn / IconBtn / ErrorState / Skeleton | Adapters over `ds`; no second implementation |
| `DangerConfirm`, `ImpactDialog` | Rebuilt on `Modal` |
| `EmptyState`, `AccessDenied` | Tokenized; on the lint allowlist |
| Assets, Compliance, Mitigations, Risks, Users, Settings, Reports, Login/Register, Incidents, Tokens, Roles | Controls and forms migrated |
| Everything else | Renders through the migrated `shared/ui` adapters, but still contains its own arbitrary values — see Known Limitations |

## Visual Regression Strategy

Two Playwright suites over a static harness — no backend, no session, no seeded
data, because a visual suite that goes red for unrelated reasons stops being
read.

- `e2e/visual/overlays.spec.ts` — registered overlays, both themes, plus a
  **coverage guard** that fails when an overlay exists in the app and is neither
  registered nor explicitly exempted, and a **staleness guard** on the exemption
  list. The exemption list may only shrink.
- `e2e/visual/design-system.spec.ts` — the gallery (`gallery.tsx`): every
  primitive in every variant and state, both themes, screenshot + axe, plus
  keyboard, reduced-motion, responsive-overflow, target-size and theme-switch
  checks.

The gallery matters because no product screen shows a disabled destructive
button, an invalid select and a warning badge at once — so nothing else would
catch one of them going unreadable in the theme the author was not using.

Snapshots wait on `document.fonts`, and the base URL is overridable via
`OPENRISK_BASE_URL`. Both exist because of real failures: fonts are fetched at
runtime, and a dev server left running from before a config change serves a
stylesheet missing the new utilities.

---

## Known Exceptions

1. **`Tabs` requires an explicit `id`.** It looks like friction. The alternative
   was worse: with the id generated privately inside `Tabs`, `aria-controls`
   pointed at panel ids no `TabPanel` could ever have, so the association was
   broken in every consumer. axe caught it; a person would not have.
2. **`Input` accepts `leadingIcon` / `trailingSlot` / `loading`.** These are
   control-internal affordances, not a second labelling mechanism. Labels,
   descriptions and errors only come from `Field`.
3. **`SkeletonRows` accepts `height`.** The density token is the default; a card
   list or timeline whose rows are not table rows needs to match what will
   replace it.
4. **The DataTable was not rewritten.** It is a large, well-tested primitive with
   capabilities nothing else here has. It consumes the tokens; rebuilding it was
   not in this wave's value.
5. **`--accent-soft` and `--accent-line` are not contrast-checked.** They are
   tints, not text colours; what sits *on* them is checked.
6. **Arbitrary values that reference a token** (`h-[var(--control-h-md)]`) are
   allowed and are not violations. That *is* using the scale; Tailwind simply has
   no utility for the property.

## Known Limitations

Stated plainly, because the alternative is a document that reads as finished.

1. **The bulk of the arbitrary values remain.** 1559 → 1518 arbitrary font sizes,
   471 → 455 arbitrary radii. W1-01 migrated the primitives, the two competing
   primitive families and the 39 files that used them; it did not sweep every
   screen. The lint rule is the mechanism for the rest, applied as a **growing
   allowlist** (`src/shared/ds/**` + four shared files) rather than an ignore
   list, so the rule is at error level for real. Adding a screen to that list is
   the unit of future work.
2. **506 raw `<button>` and 181 raw `<input>` elements remain**, mostly in
   feature screens that build one-off controls. They inherit the token layer
   (colours retint) but not the primitives' behaviour — a raw `<button>` has no
   loading state and no enforced accessible name.
3. **`no-raw-colors` still exempts 76 legacy files** (58 findings). Unchanged by
   this wave in either direction.
4. **The visual suite covers the gallery and two overlays.** 57 further overlays
   are listed in `NOT_YET_REGISTERED` — the honest record of what has no
   snapshot. It is enforced to only shrink.
5. **Product screens are not individually snapshotted.** The harness deliberately
   avoids the backend; screen-level visual proof would need the live stack.
6. **CSS grew 74.3 → 84.1 kB raw (14.1 → 16.1 kB gzipped)** — the new utility
   surface (type scale, z-layers, chart palette, motion). The JS budget is
   unchanged at 241.7 KB against a 250 KB ceiling.
7. **One pre-existing unit test fails** (`App.integration.test.tsx` — "should
   render Login page when not authenticated"). It fails identically on the parent
   commit; it is not from this wave.
8. **No Storybook.** The repository has none, and the brief says not to introduce
   one without justification. The gallery serves the same purpose for the cases
   that matter here — it is what the visual and axe suites drive.
