// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * RadioGroup — one choice from a small, fully visible set.
 *
 * WHY NATIVE RADIOS   Radios sharing a `name` already implement the roving
 *   tab stop that ARIA's radiogroup pattern describes: the group is one Tab
 *   stop, arrows move AND select within it, and the browser wraps at the ends.
 *   A `<div role="radiogroup">` with `role="radio"` children has to rebuild all
 *   of that by hand, and the usual result is a control where every option is a
 *   separate Tab stop — which is not a radio group, it is a list of checkboxes
 *   that happen to look round.
 *
 * WHEN NOT TO USE IT   More than about seven options, or options that need
 *   searching: that is a Select. A radio group's whole advantage is that every
 *   choice is visible at once, and it stops being an advantage when they are not.
 *
 * A11Y   fieldset/legend names the group, so entering it announces
 *   "group, <legend>" and the user knows what they are choosing between. Each
 *   option's label is associated by htmlFor/id; the row is the hit target at
 *   28px (WCAG 2.5.8).
 */

import { useId, type ReactNode } from 'react';
import { cn } from './cn';

export interface RadioOption {
  value: string;
  label: ReactNode;
  description?: ReactNode;
  disabled?: boolean;
}

export interface RadioGroupProps {
  legend: ReactNode;
  description?: ReactNode;
  options: ReadonlyArray<RadioOption>;
  value: string | null;
  onValueChange: (next: string) => void;
  /** Shared across the inputs; generated when not supplied. */
  name?: string;
  disabled?: boolean;
  orientation?: 'vertical' | 'horizontal';
  className?: string;
}

export function RadioGroup({
  legend,
  description,
  options,
  value,
  onValueChange,
  name,
  disabled = false,
  orientation = 'vertical',
  className,
}: RadioGroupProps) {
  const generated = useId();
  const groupName = name ?? generated;
  const descriptionId = description ? `${generated}-description` : undefined;

  return (
    <fieldset
      className={cn('flex flex-col gap-1.5', className)}
      disabled={disabled}
      aria-describedby={descriptionId}
    >
      <legend className="text-xs font-medium text-fg-secondary">{legend}</legend>
      {description && (
        <p id={descriptionId} className="text-2xs leading-snug text-fg-muted">
          {description}
        </p>
      )}
      <div
        className={cn(
          'flex gap-x-4',
          orientation === 'vertical' ? 'flex-col' : 'flex-row flex-wrap',
        )}
      >
        {options.map((option) => {
          const id = `${groupName}-${option.value}`;
          const optionDescriptionId = option.description ? `${id}-description` : undefined;
          return (
            <div key={option.value} className="flex flex-col gap-0.5">
              <label
                htmlFor={id}
                className={cn(
                  'flex min-h-(--control-h-sm) cursor-pointer items-center gap-2 text-sm text-fg-primary',
                  (option.disabled || disabled) && 'cursor-not-allowed opacity-55',
                )}
              >
                <span className="relative inline-flex shrink-0 items-center justify-center">
                  <input
                    type="radio"
                    id={id}
                    name={groupName}
                    value={option.value}
                    checked={value === option.value}
                    disabled={option.disabled}
                    aria-describedby={optionDescriptionId}
                    onChange={() => onValueChange(option.value)}
                    className={cn(
                      'peer size-4 appearance-none rounded-full border bg-surface-1 border-control',
                      'transition-[background-color,border-color] duration-fast ease-out',
                      'hover:border-strong',
                      'checked:border-accent-solid',
                      'disabled:opacity-55 disabled:pointer-events-none',
                    )}
                  />
                  {/* Sibling of the input, so `peer-checked` reaches it. */}
                  <span
                    aria-hidden="true"
                    className="pointer-events-none absolute size-2 rounded-full bg-accent-solid opacity-0 peer-checked:opacity-100"
                  />
                </span>
                {option.label}
              </label>
              {option.description && (
                <p id={optionDescriptionId} className="pl-6 text-2xs leading-snug text-fg-muted">
                  {option.description}
                </p>
              )}
            </div>
          );
        })}
      </div>
    </fieldset>
  );
}
