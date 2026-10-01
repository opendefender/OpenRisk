// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * DeleteButton — the icon-only TRIGGER for a routine, undoable removal.
 *
 * It is deliberately dumb: no arming state, no two-step "click again to
 * confirm" morph, no built-in dialog. It fires the caller's own `onClick` —
 * an undo-toast remove (`useSoftDelete`) for routine content, or a dialog
 * open for anything vital enough to need one (`AlertDialog` / `DangerConfirm`
 * / `ImpactDialog`). A second confirmation pattern living INSIDE this button
 * would be a third way to ask "are you sure?" next to the two the design
 * system already has.
 *
 * Calm by default, not permanently red: the danger tint only shows up on
 * hover/focus (`--motion-hover`), so a row of these does not read as a wall
 * of destructive actions before the user has touched any of them.
 *
 * A11Y  `aria-label` is REQUIRED, not optional — this is an icon with no
 *       visible text, so the type system makes the accessible name
 *       impossible to omit rather than relying on review to catch it.
 */

import { forwardRef, type ButtonHTMLAttributes } from 'react';
import { Trash2 } from 'lucide-react';
import { cn } from './cn';

export interface DeleteButtonProps extends Omit<
  ButtonHTMLAttributes<HTMLButtonElement>,
  'children'
> {
  'aria-label': string;
  size?: 'sm' | 'md';
}

const SIZE: Record<'sm' | 'md', string> = {
  sm: 'size-(--control-h-sm)',
  md: 'size-(--control-h-md)',
};

const ICON_PX: Record<'sm' | 'md', number> = { sm: 13, md: 15 };

export const DeleteButton = forwardRef<HTMLButtonElement, DeleteButtonProps>(function DeleteButton(
  { size = 'sm', className, type = 'button', disabled, ...rest },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      disabled={disabled}
      className={cn(
        'inline-flex shrink-0 items-center justify-center rounded-md',
        'bg-transparent text-fg-secondary border border-transparent',
        'transition-[background-color,color,border-color] duration-fast ease-out',
        'hover:bg-danger-surface hover:text-danger-text hover:border-danger',
        'focus-visible:bg-danger-surface focus-visible:text-danger-text focus-visible:border-danger',
        'disabled:pointer-events-none disabled:opacity-55',
        SIZE[size],
        className,
      )}
      {...rest}
    >
      <Trash2 size={ICON_PX[size]} aria-hidden="true" />
    </button>
  );
});
