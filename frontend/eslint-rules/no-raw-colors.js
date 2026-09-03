// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * openrisk/no-raw-colors
 *
 * Forbids literal colours and raw Tailwind palette classes in UI code.
 *
 * This rule is the reason the "modal is dark while the app is light" class of
 * bug cannot come back. A class like `bg-zinc-900` is a constant: it renders
 * identically in both themes, so any component using one is permanently wrong
 * in whichever theme it was not written for. Tokens (`bg-surface-2`,
 * `text-fg-primary`) resolve through CSS variables and follow the theme.
 *
 * Reviewers cannot be relied on to catch this — 707 such classes reached the
 * overlays before anyone noticed — so it is enforced mechanically.
 *
 * Escape hatch: none is provided by the rule itself. A genuine one-off (a brand
 * asset, a third-party embed) should carry an eslint-disable-next-line with a
 * comment explaining why the colour cannot be a token, which makes the
 * exception visible in review rather than invisible in a diff.
 */

// Tailwind's default palette families. `white`/`black` are included because
// `bg-white` is exactly as theme-blind as `bg-zinc-50`.
const PALETTES = [
  'slate',
  'gray',
  'zinc',
  'neutral',
  'stone',
  'red',
  'orange',
  'amber',
  'yellow',
  'lime',
  'green',
  'emerald',
  'teal',
  'cyan',
  'sky',
  'blue',
  'indigo',
  'violet',
  'purple',
  'fuchsia',
  'pink',
  'rose',
];

// Utility prefixes that take a colour.
const PREFIXES = [
  'bg',
  'text',
  'border',
  'ring',
  'divide',
  'outline',
  'decoration',
  'shadow',
  'accent',
  'caret',
  'fill',
  'stroke',
  'from',
  'via',
  'to',
];

const PALETTE_CLASS = new RegExp(
  `(?:^|\\s|:)(?:${PREFIXES.join('|')})-(?:${PALETTES.join('|')})-(?:50|100|200|300|400|500|600|700|800|900|950)\\b`,
);

const ACHROMATIC_CLASS = new RegExp(`(?:^|\\s|:)(?:${PREFIXES.join('|')})-(?:white|black)\\b`);

// #rgb, #rrggbb, #rrggbbaa
const HEX = /#(?:[0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})\b/;
const FUNCTIONAL = /\b(?:rgba?|hsla?)\s*\(/;

/** Suggests the token that replaces a given raw class, where one maps cleanly. */
const SUGGESTIONS = {
  bg: 'bg-surface-1 / bg-surface-2 / bg-surface-3',
  text: 'text-fg-primary / text-fg-secondary / text-fg-muted',
  border: 'border-border-default / border-border-subtle',
  ring: 'ring-accent-400',
};

function suggestionFor(match) {
  const prefix = match.trim().replace(/^:/, '').split('-')[0];
  return SUGGESTIONS[prefix] ?? 'a token from src/styles/tokens.css';
}

export default {
  meta: {
    type: 'problem',
    docs: {
      description:
        'Disallow literal colours and raw Tailwind palette classes; use semantic design tokens so components follow the active theme.',
    },
    schema: [],
    messages: {
      paletteClass:
        'Raw Tailwind palette class "{{ match }}" does not respond to the theme. Use {{ suggestion }} instead (src/styles/tokens.css).',
      achromaticClass:
        'Raw class "{{ match }}" is theme-blind: it renders identically in light and dark. Use {{ suggestion }} instead (src/styles/tokens.css).',
      literalColour:
        'Literal colour "{{ match }}" cannot follow the theme. Use a semantic token from src/styles/tokens.css.',
    },
  },

  create(context) {
    /** Reports on any string literal or template chunk that carries a colour. */
    function check(node, value) {
      if (typeof value !== 'string' || value.length === 0) return;

      const palette = value.match(PALETTE_CLASS);
      if (palette) {
        context.report({
          node,
          messageId: 'paletteClass',
          data: { match: palette[0].trim(), suggestion: suggestionFor(palette[0]) },
        });
        return;
      }

      const achromatic = value.match(ACHROMATIC_CLASS);
      if (achromatic) {
        context.report({
          node,
          messageId: 'achromaticClass',
          data: { match: achromatic[0].trim(), suggestion: suggestionFor(achromatic[0]) },
        });
        return;
      }

      const literal = value.match(HEX) || value.match(FUNCTIONAL);
      if (literal) {
        context.report({
          node,
          messageId: 'literalColour',
          data: { match: literal[0] },
        });
      }
    }

    return {
      Literal(node) {
        check(node, node.value);
      },
      // Covers `className={\`... ${x} ...\`}`, where the colour usually hides.
      TemplateElement(node) {
        check(node, node.value.raw);
      },
    };
  },
};
