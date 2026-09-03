// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Fieldset — a named section of a form.
 *
 * Field wires ONE control. This wires a set of them, which is a different job:
 * "Notification channels" containing four switches, or "Scope" containing a
 * date range and two selects. CheckboxGroup and RadioGroup use a fieldset
 * internally for their own options; this is the general case, for a group whose
 * children are arbitrary controls.
 *
 * WHY NOT a div and a heading: only fieldset/legend is announced as
 * "group, <legend>" when focus enters a child. With a heading, a screen-reader
 * user tabbing into the third switch hears the switch and nothing about what it
 * belongs to — and in a settings screen the group name is usually the half that
 * carries the meaning.
 *
 * `disabled` on a fieldset cascades to every control inside it, which is a real
 * HTML feature and the reason this is worth having: disabling a section is one
 * attribute rather than a prop threaded through every child.
 */

import { useId, type ReactNode } from 'react';
import { cn } from './cn';

export interface FieldsetProps {
  legend: ReactNode;
  description?: ReactNode;
  /** Cascades to every control inside — a native fieldset behaviour. */
  disabled?: boolean;
  className?: string;
  children: ReactNode;
}

export function Fieldset({
  legend,
  description,
  disabled = false,
  className,
  children,
}: FieldsetProps) {
  const id = useId();
  const descriptionId = description ? `${id}-description` : undefined;

  return (
    <fieldset
      className={cn('flex flex-col gap-2', className)}
      disabled={disabled}
      aria-describedby={descriptionId}
    >
      <legend className="text-xs font-semibold uppercase tracking-caps text-fg-secondary">
        {legend}
      </legend>
      {description && (
        <p id={descriptionId} className="text-2xs leading-snug text-fg-muted">
          {description}
        </p>
      )}
      {children}
    </fieldset>
  );
}
