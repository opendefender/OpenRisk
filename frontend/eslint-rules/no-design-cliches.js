// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * openrisk/no-design-cliches
 *
 * Enforces the design guide's anti-cliché list as a lint rule rather than a
 * review preference — contribution rule 1 of the guide, in its own words.
 *
 * The list is not about taste. Each ban is a decision the console already took
 * once, which a component can silently retake in a way nothing else can see:
 *
 *   - **Blur beyond 4px.** `backdrop-blur-xl` on an overlay is the single most
 *     expensive thing a GRC console can paint, and it is repainted on every
 *     scroll frame behind a modal. The cap is `sm`, which is what
 *     --overlay-blur is set to; anything above it is a per-component decision
 *     to spend frames the product does not have on a link from Douala.
 *   - **Gradients.** A gradient is two colours the token layer never agreed to.
 *     `bg-gradient-*` and `bg-linear-*` cannot follow the theme the way a
 *     surface token does, and `text-transparent bg-clip-text` additionally
 *     deletes the text colour, so the label is invisible wherever the gradient
 *     fails to paint.
 *   - **`animate-pulse` outside the skeleton.** A pulsing element means
 *     "loading" in this product. Using it for emphasis teaches the user that
 *     the meaning is unreliable, which costs more than the emphasis gained.
 *   - **Accent-tinted shadows.** A coloured glow is the cliché that dates an
 *     interface hardest, and it puts the signature hue somewhere it carries no
 *     meaning — the accent identifies interactive things, not decorative ones.
 *
 * Applied at `error` on frontend/src/**, with an exemption list that may only
 * shrink. Not `warn`: dropping a rule to warn to accommodate the existing
 * backlog is exactly how 1559 arbitrary values accumulated before W1-01.
 * `frontend/src/shared/ds/**` is never exempted — it is clean today, and it is
 * where #443 vendors ~50 third-party components.
 *
 * Escape hatch: none in the rule. A genuine one-off carries an
 * eslint-disable-next-line with a reason, which is visible in review.
 */

/** Blur steps above the 4px cap. `backdrop-blur-sm` and below are fine. */
const BLUR = /(?:^|\s|:)backdrop-blur-(md|lg|xl|2xl|3xl)\b/;

/** Tailwind v3 (`bg-gradient-to-r`) and v4 (`bg-linear-to-r`) spellings. */
const GRADIENT = /(?:^|\s|:)bg-(?:gradient|linear|radial|conic)-/;

/** The gradient-text cliché. `bg-clip-text` is the load-bearing half. */
const CLIP_TEXT = /(?:^|\s|:)bg-clip-text\b/;

const PULSE = /(?:^|\s|:)animate-pulse\b/;

/**
 * A shadow whose colour is the accent. Catches the arbitrary form
 * (`shadow-[0_0_20px_var(--accent)]`) and the utility form (`shadow-accent`,
 * `shadow-primary/20`) alike, since both put the signature hue in a glow.
 */
const ACCENT_SHADOW =
  /(?:^|\s|:)shadow-(?:\[[^\]]*(?:--accent|--color-accent|--primary)[^\]]*\]|(?:accent|primary)\b)/;

/** The one file allowed to pulse: it is what `Skeleton` is made of. */
const PULSE_EXEMPT = /src[\\/]shared[\\/]ds[\\/]States\.tsx$/;

export default {
  meta: {
    type: 'problem',
    docs: {
      description:
        "Enforce the design guide's anti-cliché list: no heavy blur, no gradients, no pulse outside the skeleton, no accent-tinted shadows.",
    },
    schema: [],
    messages: {
      blur: 'backdrop-blur-{{ step }} exceeds the 4px cap. Use backdrop-blur-sm — the value --overlay-blur already carries.',
      gradient:
        'Gradient "{{ match }}" is two colours the token layer never agreed to, and it cannot follow the theme. Use a surface token.',
      clipText:
        '`bg-clip-text` with a transparent foreground deletes the text colour: the label disappears wherever the gradient does not paint. Use text-fg-primary.',
      pulse:
        '`animate-pulse` means "loading" in this product. Use the Skeleton primitive from src/shared/ds/States.tsx instead of pulsing this element.',
      accentShadow:
        'Accent-tinted shadow "{{ match }}": the accent identifies interactive things, not decorative glow. Use an elevation token (shadow-e1 / shadow-e2).',
    },
  },

  create(context) {
    const filename = context.filename ?? context.getFilename();
    const pulseAllowed = PULSE_EXEMPT.test(filename);

    function check(node, value) {
      if (typeof value !== 'string' || value.length === 0) return;

      const blur = value.match(BLUR);
      if (blur) {
        context.report({ node, messageId: 'blur', data: { step: blur[1] } });
      }

      const gradient = value.match(GRADIENT);
      if (gradient) {
        context.report({
          node,
          messageId: 'gradient',
          data: { match: gradient[0].trim() },
        });
      }

      if (CLIP_TEXT.test(value)) {
        context.report({ node, messageId: 'clipText' });
      }

      if (!pulseAllowed && PULSE.test(value)) {
        context.report({ node, messageId: 'pulse' });
      }

      const shadow = value.match(ACCENT_SHADOW);
      if (shadow) {
        context.report({
          node,
          messageId: 'accentShadow',
          data: { match: shadow[0].trim() },
        });
      }
    }

    return {
      Literal(node) {
        check(node, node.value);
      },
      // Covers `className={`... ${x} ...`}`, where these usually hide.
      TemplateElement(node) {
        check(node, node.value.raw);
      },
    };
  },
};
