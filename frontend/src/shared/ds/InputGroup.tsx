// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * InputGroup — one control assembled from several elements.
 *
 * A search box with a scope select in front of it; an amount with its currency
 * behind it; a URL field with a "Test" button attached. Written by hand this is
 * always the same three bugs: a doubled 1px border where two controls meet, two
 * separate focus rings, and rounded corners on the inside edges so the seam
 * shows.
 *
 * The group draws the border and the radius; the children are stripped of both
 * and joined with a single divider. Focus is drawn on the GROUP, using
 * :focus-within, so a group reads as one control to the eye exactly as it does
 * to a keyboard user moving through it.
 *
 * NOT A LAYOUT BOX   Everything in a group has to sit on the `--control-h-*`
 * ramp, which is why the height is a prop and not something a caller passes
 * through className. Two different heights inside one border is the thing this
 * exists to prevent.
 */

import { type ReactNode } from 'react';
import { cn } from './cn';

export type InputGroupSize = 'sm' | 'md' | 'lg';

export interface InputGroupProps {
  size?: InputGroupSize;
  /** Non-interactive text or icon fused to the leading edge (a unit, a prefix). */
  prefix?: ReactNode;
  /** Same, trailing. */
  suffix?: ReactNode;
  invalid?: boolean;
  disabled?: boolean;
  className?: string;
  children: ReactNode;
}

const HEIGHT: Record<InputGroupSize, string> = {
  sm: 'h-(--control-h-sm) text-xs',
  md: 'h-(--control-h-md) text-sm',
  lg: 'h-(--control-h-lg) text-base',
};

const ADDON_PX: Record<InputGroupSize, string> = {
  sm: 'px-(--control-px-sm)',
  md: 'px-(--control-px-md)',
  lg: 'px-(--control-px-lg)',
};

export function InputGroup({
  size = 'md',
  prefix,
  suffix,
  invalid = false,
  disabled = false,
  className,
  children,
}: InputGroupProps) {
  /* Normalise before indexing: an unexpected union value must not yield
     `undefined` classes and a zero-height group. */
  const scale: InputGroupSize = size === 'sm' ? 'sm' : size === 'lg' ? 'lg' : 'md';

  return (
    <div
      className={cn(
        'flex w-full items-stretch overflow-hidden rounded-md border bg-surface-1',
        invalid ? 'border-danger' : 'border-control',
        'focus-within:border-accent',
        'transition-[border-color] duration-fast ease-out',
        /* Inert, but the affix stays READABLE. Fading the whole group with
           opacity took the "%" and the "https://" down to 2.7:1, and an affix
           is part of the value rather than chrome — you still have to be able
           to read what unit the disabled number is in. The control itself
           renders its own `disabled:` styling, which is the part WCAG 1.4.3
           exempts and axe recognises. */
        disabled && 'pointer-events-none bg-surface-sunken',
        HEIGHT[scale],
        /* The children are controls in their own right and each arrives with a
           border, a radius and a focus ring. The group owns all three, so they
           are removed here rather than asking every call site to remember. */
        '[&_input]:h-full [&_input]:w-full [&_input]:min-w-0 [&_input]:border-0 [&_input]:bg-transparent [&_input]:outline-none',
        '[&_select]:h-full [&_select]:border-0 [&_select]:bg-transparent [&_select]:outline-none',
        '[&_button]:h-full [&_button]:rounded-none [&_button]:border-0',
        className,
      )}
    >
      {prefix !== undefined && (
        <span
          className={cn(
            /* --fg-secondary, not --fg-muted: an affix like "https://" or "%"
               is part of the value the user is reading, not a hint about it,
               and muted measures 2.7:1 on --surface-2 at this size. axe caught
               it on the gallery page. */
            'inline-flex items-center border-r border-control text-fg-secondary',
            disabled ? 'bg-surface-sunken' : 'bg-surface-2',
            ADDON_PX[scale],
          )}
        >
          {prefix}
        </span>
      )}
      {children}
      {suffix !== undefined && (
        <span
          className={cn(
            'inline-flex items-center border-l border-control text-fg-secondary',
            disabled ? 'bg-surface-sunken' : 'bg-surface-2',
            ADDON_PX[scale],
          )}
        >
          {suffix}
        </span>
      )}
    </div>
  );
}
