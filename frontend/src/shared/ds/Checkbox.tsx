// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Checkbox and CheckboxGroup.
 *
 * WHY A REAL <input>   The box is an `appearance-none` checkbox styled
 *   directly, not a `<div role="checkbox">` and not a visually-hidden input
 *   behind a fake box. It therefore keeps, for free and correctly: Space to
 *   toggle, participation in a form and its reset, the `:indeterminate`
 *   pseudo-class, and whatever the platform's screen reader and voice control
 *   already know about checkboxes. Every custom checkbox in this codebase's
 *   history reimplemented two of those four and shipped the other two broken.
 *
 * TARGET SIZE   The box is 16px because a 28px box looks like a button, but the
 *   *hit target* is the whole label row at `--control-h-sm` (28px), which is
 *   the floor in WCAG 2.5.8. Shrinking the row to fit a dense table is the one
 *   change that would silently break that, so the height is a token, not a
 *   literal.
 *
 * INDETERMINATE  Cannot be expressed in HTML markup — it is a DOM property, so
 *   it is set through a ref. It is a real third state, not "unchecked with a
 *   dash": a group whose children are partly selected must not report itself as
 *   unchecked to a screen reader.
 *
 * A11Y   - label ↔ input via htmlFor/id, always, generated when not supplied
 *        - description reaches the input through aria-describedby
 *        - inside a Field, the Field's id/description/error/invalid/disabled
 *          wiring is inherited (useControlWiring), so a checkbox in a form row
 *          announces the same error as an input would
 *        - the check glyph is aria-hidden; the state comes from the input
 */

import {
  forwardRef,
  useEffect,
  useId,
  useImperativeHandle,
  useRef,
  type InputHTMLAttributes,
  type ReactNode,
} from 'react';
import { Check, Minus } from 'lucide-react';
import { cn } from './cn';
import { useControlWiring } from './fieldContext';

export interface CheckboxProps
  extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type' | 'size' | 'children'> {
  /** Sits beside the box. Omit only when an aria-label is supplied instead. */
  label?: ReactNode;
  /** Secondary line under the label. Reaches the input via aria-describedby. */
  description?: ReactNode;
  /** The third state: some children selected, not all. */
  indeterminate?: boolean;
}

export const Checkbox = forwardRef<HTMLInputElement, CheckboxProps>(function Checkbox(
  { label, description, indeterminate = false, className, id, disabled, ...rest },
  ref,
) {
  const { id: wiredId, aria } = useControlWiring(id);
  const generated = useId();
  /* Whoever supplies the label owns the id. A checkbox with its own label
     takes its own id, so only that label names it; without one it adopts the
     Field's id and is named by the Field's label. Getting this backwards makes
     BOTH labels point at the input, and HTML concatenates them — the box then
     announces as "Consent I accept", which is two questions read as one. */
  const inputId = id ?? (label ? generated : wiredId);
  const descriptionId = description ? `${inputId}-description` : undefined;

  const inner = useRef<HTMLInputElement>(null);
  useImperativeHandle(ref, () => inner.current as HTMLInputElement, []);
  useEffect(() => {
    if (inner.current) inner.current.indeterminate = indeterminate;
  }, [indeterminate]);

  const describedBy = [aria['aria-describedby'], descriptionId].filter(Boolean).join(' ') || undefined;
  const isDisabled = disabled ?? aria.disabled;

  const box = (
    <span className="relative inline-flex shrink-0 items-center justify-center">
      <input
        ref={inner}
        type="checkbox"
        id={inputId}
        disabled={isDisabled}
        className={cn(
          'peer size-4 appearance-none rounded-xs border bg-surface-1',
          'border-control',
          'transition-[background-color,border-color] duration-fast ease-out',
          'hover:border-strong',
          'checked:border-accent-solid checked:bg-accent-solid',
          'indeterminate:border-accent-solid indeterminate:bg-accent-solid',
          'disabled:opacity-55 disabled:pointer-events-none',
          className,
        )}
        aria-invalid={aria['aria-invalid']}
        aria-required={aria['aria-required']}
        aria-describedby={describedBy}
        {...rest}
      />
      {/* Drawn over the input, never in front of it for pointer purposes.
          The visibility variants live HERE, on the input's sibling: `peer-*`
          only reaches siblings, so putting them on the icon inside would have
          styled nothing and left the glyph permanently visible. */}
      <span
        aria-hidden="true"
        className={cn(
          'pointer-events-none absolute inset-0 flex items-center justify-center text-fg-on-solid',
          'opacity-0 peer-checked:opacity-100 peer-indeterminate:opacity-100',
        )}
      >
        {indeterminate ? <Minus size={12} strokeWidth={3} /> : <Check size={12} strokeWidth={3} />}
      </span>
    </span>
  );

  if (!label && !description) return box;

  return (
    <div className="flex flex-col gap-0.5">
      <label
        htmlFor={inputId}
        className={cn(
          /* The row is the target, not the 16px box — WCAG 2.5.8. */
          'flex min-h-(--control-h-sm) cursor-pointer items-center gap-2 text-sm text-fg-primary',
          isDisabled && 'cursor-not-allowed opacity-55',
        )}
      >
        {box}
        {label}
      </label>
      {description && (
        <p id={descriptionId} className="pl-6 text-2xs leading-snug text-fg-muted">
          {description}
        </p>
      )}
    </div>
  );
});

export interface CheckboxGroupProps {
  /** Rendered as a real <legend>: it names the group for a screen reader. */
  legend: ReactNode;
  description?: ReactNode;
  options: ReadonlyArray<{ value: string; label: ReactNode; description?: ReactNode; disabled?: boolean }>;
  value: readonly string[];
  onValueChange: (next: string[]) => void;
  disabled?: boolean;
  /** Horizontal only suits two or three short options; the default stacks. */
  orientation?: 'vertical' | 'horizontal';
  className?: string;
}

/**
 * A set of related checkboxes.
 *
 * fieldset/legend rather than a div and a heading: it is the only construct a
 * screen reader announces as "group, <legend>" when focus enters any child, so
 * the user hears what the checkbox belongs to without leaving it. A div with a
 * styled span above it reads as four unrelated checkboxes.
 */
export function CheckboxGroup({
  legend,
  description,
  options,
  value,
  onValueChange,
  disabled = false,
  orientation = 'vertical',
  className,
}: CheckboxGroupProps) {
  const id = useId();
  const descriptionId = description ? `${id}-description` : undefined;

  function toggle(option: string, checked: boolean) {
    onValueChange(checked ? [...value, option] : value.filter((v) => v !== option));
  }

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
      <div className={cn('flex gap-x-4', orientation === 'vertical' ? 'flex-col' : 'flex-row flex-wrap')}>
        {options.map((option) => (
          <Checkbox
            key={option.value}
            value={option.value}
            label={option.label}
            description={option.description}
            disabled={option.disabled}
            checked={value.includes(option.value)}
            onChange={(e) => toggle(option.value, e.currentTarget.checked)}
          />
        ))}
      </div>
    </fieldset>
  );
}
