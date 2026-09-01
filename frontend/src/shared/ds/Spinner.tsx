// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Spinner — the indeterminate-progress atom.
 *
 * NOT the same thing as LoadingState, and the difference is the reason both
 * exist. `LoadingState` is a BLOCK: a centred spinner with a label and generous
 * padding, filling the region whose content has not arrived. `Spinner` is the
 * glyph itself, for the places that already have their own layout and label —
 * inside a button, beside a row that is refreshing, in a toolbar.
 *
 * Before this, that glyph was written out three times inside this directory —
 * Button, Field's loading input and LoadingState — each spelling
 * `<Loader2 className="motion-safe:animate-spin" />` slightly differently.
 * LoadingState now uses this component. Button and Field deliberately do NOT
 * yet: both take a `LucideIcon` in a component-type position and swap it for a
 * tick or a warning glyph, so threading a non-icon component through that union
 * would complicate two frozen APIs to save two lines. They are the next callers
 * when that union is revisited, not today.
 *
 * A11Y   The default is `aria-hidden`, which is deliberate and is the opposite
 *   of what a naive implementation does. A spinner is almost always sitting
 *   beside text that already says what is happening — a button whose label is
 *   "Saving…", a LoadingState with a caption — and announcing "busy" a second
 *   time is noise. Pass a `label` ONLY when the spinner is the sole indication,
 *   and it then becomes a `role="status"` live region.
 *
 * MOTION `motion-safe:` throughout: under prefers-reduced-motion the glyph
 *   stops turning rather than disappearing, so the affordance survives.
 */

import { Loader2 } from 'lucide-react';
import { cn } from './cn';

export type SpinnerSize = 'xs' | 'sm' | 'md' | 'lg';

/* Pixel sizes rather than a token scale: this is an icon, and the icon sizes in
   this system are the lucide `size` prop, which takes a number. They line up
   with the ICON_PX ramp in Button. */
const PX: Record<SpinnerSize, number> = { xs: 12, sm: 14, md: 18, lg: 24 };

export interface SpinnerProps {
  size?: SpinnerSize;
  /** Announces the spinner as a live region. Omit when adjacent text says it. */
  label?: string;
  className?: string;
}

export function Spinner({ size = 'md', label, className }: SpinnerProps) {
  /* Normalised before indexing, per the frontend charter: an unexpected union
     value must not yield `undefined` and an icon with no size. */
  const px = PX[size] ?? PX.md;

  const glyph = (
    <Loader2
      size={px}
      className={cn('motion-safe:animate-spin', className)}
      aria-hidden={label ? undefined : true}
    />
  );

  if (!label) return glyph;

  return (
    <span role="status" aria-live="polite" className="inline-flex items-center">
      {glyph}
      <span className="sr-only">{label}</span>
    </span>
  );
}
