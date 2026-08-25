// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1

/**
 * Button — the canonical control.
 *
 * ANATOMY   [ optional leading icon ][ label ][ optional trailing icon ]
 *           inside a token-sized box; loading swaps the leading slot for a
 *           spinner so the label never moves.
 *
 * VARIANTS  primary | secondary | ghost | destructive | link
 *           One primary per view. The hierarchy is carried by fill: primary is
 *           the only filled accent, destructive is the only filled red, and
 *           everything else is a surface or nothing.
 *
 * SIZES     sm (28px) | md (36px, default) | lg (44px)
 *           Heights come from --control-h-*, shared with the form controls, so
 *           a button in a filter bar lines up with the select next to it.
 *
 * STATES    default, hover, active, focus-visible, disabled, loading, and a
 *           transient `feedback` of 'success' | 'error' for the moment after a
 *           mutation resolves.
 *
 * MOTION    Colour on --motion-hover; a 1px downward press on --motion-press.
 *           Transform and colour only, no layout, so a table of 200 rows with
 *           an action button each does not reflow on hover.
 *
 * A11Y      - `loading` sets aria-busy and disables, so a mutation cannot be
 *             fired twice; the label stays visible (a spinner that replaces the
 *             label leaves a screen-reader user with an unnamed button).
 *           - An icon-only button REQUIRES an accessible name. The types make
 *             it impossible to omit: no children means `aria-label` is
 *             mandatory, checked by the compiler rather than by review.
 *           - Focus uses the global :focus-visible ring; it is never removed.
 *
 * WHY NOT the two that came before: components/ui/Button used
 * `text-text-primary` on an accent fill, which measures 3.1:1 in White mode,
 * and shared/ui.Btn took `primary?: boolean; danger?: boolean` — magic booleans
 * that cannot express "tertiary" and made a third state (both true) legal.
 */

import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react';
import { Check, Loader2, TriangleAlert, type LucideIcon } from 'lucide-react';
import { cn } from './cn';

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'destructive' | 'link';
export type ButtonSize = 'sm' | 'md' | 'lg';

interface ButtonBase extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'children'> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  /** Disables and marks aria-busy. The label stays put. */
  loading?: boolean;
  /** Transient result of the last action. Swaps the leading icon only. */
  feedback?: 'success' | 'error';
  icon?: LucideIcon;
  /** Leading by default — a trailing icon reads as "this goes somewhere". */
  iconPosition?: 'leading' | 'trailing';
  fullWidth?: boolean;
}

/* An icon-only button has no visible text, so its accessible name has to come
   from somewhere. Requiring it in the type is the difference between a rule
   people remember and a rule the build enforces. */
type ButtonProps =
  | (ButtonBase & { children: ReactNode; 'aria-label'?: string })
  | (ButtonBase & { children?: undefined; 'aria-label': string });

const VARIANT: Record<ButtonVariant, string> = {
  /* The only filled accent in the product. No gradient and no glow: the
     previous primary was `linear-gradient(135deg, …)` under a 12px accent
     shadow, which is the single strongest "generic dashboard" signal a UI can
     send, on its most-used control. --accent-solid + --text-on-solid is a
     contrast-verified pair (checked in CI). */
  primary:
    'bg-accent-solid text-text-on-solid border border-transparent hover:brightness-[1.08] active:brightness-95',
  /* The default. A real surface with a control-grade border, so it is
     identifiable as a control at 3:1 without being loud. */
  secondary:
    'bg-surface-2 text-text-primary border border-control hover:bg-surface-3 active:bg-surface-3',
  /* No border, no fill until hover. For toolbars and row actions, where five
     bordered buttons in a row would read as a fence. */
  ghost:
    'bg-transparent text-text-secondary border border-transparent hover:bg-surface-3 hover:text-text-primary',
  /* Filled, not tinted. A destructive action that looks like a secondary
     button is a trap; --danger-solid is dark enough to carry white text. */
  destructive:
    'bg-danger-solid text-text-on-solid border border-transparent hover:brightness-110 active:brightness-95',
  /* Inline, in prose. Underlined always — colour alone is not an affordance
     (WCAG 1.4.1). */
  link: 'bg-transparent text-accent border-none underline underline-offset-2 hover:text-accent-hover px-0 h-auto',
};

const SIZE: Record<ButtonSize, string> = {
  sm: 'h-[var(--control-h-sm)] px-[var(--control-px-sm)] text-xs rounded-sm gap-[var(--control-gap)]',
  md: 'h-[var(--control-h-md)] px-[var(--control-px-md)] text-sm rounded-md gap-[var(--control-gap)]',
  lg: 'h-[var(--control-h-lg)] px-[var(--control-px-lg)] text-base rounded-md gap-[var(--control-gap)]',
};

/** Square when there is no label, so an icon button is not a wide pill. */
const ICON_ONLY_SIZE: Record<ButtonSize, string> = {
  sm: 'w-[var(--control-h-sm)] px-0',
  md: 'w-[var(--control-h-md)] px-0',
  lg: 'w-[var(--control-h-lg)] px-0',
};

const ICON_PX: Record<ButtonSize, number> = { sm: 14, md: 16, lg: 18 };

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  {
    variant = 'secondary',
    size = 'md',
    loading = false,
    feedback,
    icon: Icon,
    iconPosition = 'leading',
    fullWidth = false,
    className,
    children,
    disabled,
    type = 'button',
    ...rest
  },
  ref,
) {
  const iconOnly = children == null;
  const px = ICON_PX[size];

  /* Precedence: in flight beats the previous result. A spinner and a tick at
     the same time is a lie about one of them. */
  const LeadIcon = loading
    ? Loader2
    : feedback === 'success'
      ? Check
      : feedback === 'error'
        ? TriangleAlert
        : Icon;

  const glyph = LeadIcon ? (
    <LeadIcon
      size={px}
      strokeWidth={1.9}
      aria-hidden="true"
      className={loading ? 'motion-safe:animate-spin' : undefined}
    />
  ) : null;

  return (
    <button
      ref={ref}
      type={type}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={cn(
        'inline-flex shrink-0 select-none items-center justify-center font-semibold',
        'transition-[background-color,color,border-color,filter,transform] duration-fast ease-out',
        /* Presses move 1px. Enough to feel mechanical, small enough that it
           never reads as the button falling over. */
        variant !== 'link' && 'motion-safe:active:translate-y-px',
        'disabled:pointer-events-none disabled:opacity-55',
        VARIANT[variant],
        variant !== 'link' && SIZE[size],
        variant !== 'link' && iconOnly && ICON_ONLY_SIZE[size],
        fullWidth && 'w-full',
        className,
      )}
      {...rest}
    >
      {iconPosition === 'leading' && glyph}
      {children}
      {iconPosition === 'trailing' && glyph}
    </button>
  );
});
