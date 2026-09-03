// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Flags arbitrary Tailwind values for properties that have a token scale.
 *
 * `text-[13px]`, `rounded-[10px]`, `z-[70]`, `duration-[220ms]` — each of these
 * is a design decision taken privately by one component. The scales exist
 * precisely so those decisions are taken once; the audit that opened W1-01
 * counted 1559 arbitrary font sizes, 471 arbitrary radii and ten different
 * z-index integers, none of which any other file could see.
 *
 * What is NOT flagged, deliberately:
 *
 *   - `h-(--control-h-md)` and anything else whose arbitrary value is a
 *     token reference. That IS using the scale; Tailwind just has no utility
 *     for the property.
 *   - `max-w-[46ch]`, `w-[min(560px,92vw)]` and other layout values that are
 *     genuinely one-off geometry rather than a step on a scale. Width is not a
 *     token scale in this system and pretending otherwise would produce noise.
 *   - Anything in a test or the visual harness.
 *
 * The rule is applied to an ALLOWLIST of files (see eslint.config.js) rather
 * than to everything with an ignore list, because the migration is partial and
 * honest about it: a file is added to the list when it has been migrated, and
 * the list only grows. That way the rule is at error level for real, instead of
 * being dropped to 'warn' to accommodate the backlog — which is how the 1559
 * accumulated in the first place.
 */

/** utility prefix -> what to use instead. */
const SCALED = {
  text: 'the type scale (text-2xs … text-3xl)',
  rounded: 'the radius scale (rounded-xs … rounded-xl, rounded-full)',
  z: 'a named layer (z-dropdown, z-drawer, z-modal, z-popover, z-toast, z-tooltip)',
  duration: 'a motion token (duration-fast, duration-base, duration-slow, duration-panel)',
  ease: 'a motion curve (ease-out, ease-in, ease-inout, ease-emphasized)',
  leading: 'the line-height scale (leading-tight … leading-relaxed)',
  tracking: 'the tracking scale (tracking-display, tracking-caps)',
};

/**
 * Matches `<prefix>-[<value>]`, including a responsive or state variant
 * (`sm:text-[13px]`, `hover:rounded-[10px]`).
 */
const ARBITRARY =
  /(?:^|\s)(?:[a-z-]+:)*(text|rounded|z|duration|ease|leading|tracking)-\[([^\]]+)\]/g;

/** A value that is itself a token reference is the correct way to use a token. */
function isTokenReference(value) {
  return value.includes('var(--');
}

/**
 * `text-(--x)` and `text-[#fff]` are colour, not type size — colour
 * is the other rule's job (no-raw-colors), so this one stays out of it.
 */
function isColourValue(value) {
  return value.startsWith('color:') || value.startsWith('#') || value.startsWith('rgb');
}

export default {
  meta: {
    type: 'problem',
    docs: {
      description:
        'Disallow arbitrary Tailwind values for properties that have a design token scale',
    },
    schema: [],
    messages: {
      adHoc:
        "'{{utility}}-[{{value}}]' is an ad-hoc design value. Use {{guidance}}. " +
        'If no step fits, add one to the scale in src/styles/primitives.css so every ' +
        'screen gets it — see docs/W1-01_OPENRISK_DESIGN_SYSTEM.md.',
    },
  },

  create(context) {
    function checkString(node, raw) {
      if (typeof raw !== 'string') return;
      ARBITRARY.lastIndex = 0;
      let match;
      while ((match = ARBITRARY.exec(raw)) !== null) {
        const [, utility, value] = match;
        if (isTokenReference(value) || isColourValue(value)) continue;
        context.report({
          node,
          messageId: 'adHoc',
          data: { utility, value, guidance: SCALED[utility] },
        });
      }
    }

    return {
      Literal(node) {
        checkString(node, node.value);
      },
      TemplateElement(node) {
        checkString(node, node.value.raw);
      },
    };
  },
};
