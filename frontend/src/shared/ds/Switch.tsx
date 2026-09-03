// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Switch — a setting that takes effect immediately.
 *
 * CHECKBOX OR SWITCH   Not interchangeable, and the distinction is about when
 *   the change lands, not about how it looks. A checkbox is a value you are
 *   about to submit; a switch is a setting that applies the moment it moves. A
 *   switch inside a form with a Save button is the wrong control and tells the
 *   user a lie about what just happened.
 *
 * WHY role="switch" ON A REAL CHECKBOX   The input keeps Space, form
 *   participation and the platform's own handling; `role="switch"` changes only
 *   what it is announced as — "switch, on" rather than "checkbox, checked",
 *   which is the difference the user needs to hear. A `<button role="switch">`
 *   would have to carry `aria-checked` by hand and leaves the form out entirely.
 *
 * NOT COLOUR ALONE   The thumb MOVES. A switch distinguished only by its track
 *   colour is a state encoded in hue (WCAG 1.4.1), which is unreadable to a
 *   large minority of users and invisible in a greyscale print of a compliance
 *   report — which this console's users produce.
 */

import { forwardRef, useId, type InputHTMLAttributes, type ReactNode } from 'react';
import { cn } from './cn';
import { useControlWiring } from './fieldContext';

export type SwitchSize = 'sm' | 'md';

export interface SwitchProps extends Omit<
  InputHTMLAttributes<HTMLInputElement>,
  'type' | 'size' | 'children' | 'role'
> {
  label?: ReactNode;
  description?: ReactNode;
  size?: SwitchSize;
  /** Puts the control before the label. Default is label first, control right —
   *  a settings list reads as "name … state", not "state … name". */
  controlFirst?: boolean;
}

/* Track and thumb travel, per size. The thumb offset is the track width minus
   the thumb width minus twice the 2px inset. */
const TRACK: Record<SwitchSize, string> = {
  sm: 'h-4 w-7',
  md: 'h-5 w-9',
};
const THUMB: Record<SwitchSize, string> = {
  sm: 'size-3 peer-checked:translate-x-3',
  md: 'size-4 peer-checked:translate-x-4',
};

export const Switch = forwardRef<HTMLInputElement, SwitchProps>(function Switch(
  { label, description, size = 'md', controlFirst = false, className, id, disabled, ...rest },
  ref,
) {
  const { id: wiredId, aria } = useControlWiring(id);
  const generated = useId();
  /* Whoever supplies the label owns the id. A switch with its own label
     takes its own id, so only that label names it; without one it adopts the
     Field's id and is named by the Field's label. Getting this backwards makes
     BOTH labels point at the input, and HTML concatenates them — the switch then
     announces as "Consent I accept", which is two questions read as one. */
  const inputId = id ?? (label ? generated : wiredId);
  const descriptionId = description ? `${inputId}-description` : undefined;
  const describedBy =
    [aria['aria-describedby'], descriptionId].filter(Boolean).join(' ') || undefined;
  const isDisabled = disabled ?? aria.disabled;

  /* `size` is a union from a caller; normalise before indexing so an unexpected
     value cannot produce `undefined` classes and an invisible control. */
  const scale: SwitchSize = size === 'sm' ? 'sm' : 'md';

  const control = (
    <span className="relative inline-flex shrink-0 items-center">
      <input
        ref={ref}
        type="checkbox"
        role="switch"
        id={inputId}
        disabled={isDisabled}
        aria-describedby={describedBy}
        className={cn(
          'peer appearance-none rounded-full border border-control bg-surface-3',
          'transition-[background-color,border-color] duration-fast ease-out',
          'hover:border-strong',
          'checked:border-accent-solid checked:bg-accent-solid',
          'disabled:opacity-55 disabled:pointer-events-none',
          TRACK[scale],
          className,
        )}
        {...rest}
      />
      {/* Sibling of the input so `peer-checked` reaches it. The MOVEMENT is the
          state; the colour only reinforces it. */}
      <span
        aria-hidden="true"
        className={cn(
          'pointer-events-none absolute left-0.5 rounded-full bg-fg-on-solid shadow-e1',
          'transition-transform duration-fast ease-out motion-reduce:transition-none',
          THUMB[scale],
        )}
      />
    </span>
  );

  if (!label && !description) return control;

  return (
    <div className="flex flex-col gap-0.5">
      <label
        htmlFor={inputId}
        className={cn(
          'flex min-h-(--control-h-sm) cursor-pointer items-center gap-3 text-sm text-fg-primary',
          controlFirst ? 'flex-row' : 'flex-row-reverse justify-between',
          isDisabled && 'cursor-not-allowed opacity-55',
        )}
      >
        {control}
        <span>{label}</span>
      </label>
      {description && (
        <p id={descriptionId} className="text-2xs leading-snug text-fg-muted">
          {description}
        </p>
      )}
    </div>
  );
});
