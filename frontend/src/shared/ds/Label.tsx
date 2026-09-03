// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * A standalone label, for the cases Field cannot cover.
 *
 * Field owns the label of a normal form row and a screen author should never
 * write one by hand there. This exists for the two places that legitimately
 * need a label without a Field: the legend-like heading above a group of
 * controls that is not a fieldset, and a control rendered inside a table cell
 * or a toolbar where the Field label/description/message stack would be wrong.
 *
 * It carries the SAME required marker as Field, for one reason: an asterisk
 * that is only a coloured glyph is colour-plus-shape with no text (WCAG 1.4.1),
 * and a screen reader either skips it or says "star". The visible marker is
 * aria-hidden and a real word goes in the accessible name.
 */

import { type LabelHTMLAttributes, type ReactNode } from 'react';
import { cn } from './cn';

export interface LabelProps extends LabelHTMLAttributes<HTMLLabelElement> {
  required?: boolean;
  disabled?: boolean;
  children: ReactNode;
}

export function Label({
  required = false,
  disabled = false,
  className,
  children,
  ...rest
}: LabelProps) {
  return (
    <label
      className={cn('text-xs font-medium text-fg-secondary', disabled && 'opacity-55', className)}
      {...rest}
    >
      {children}
      {required && (
        <>
          <span aria-hidden="true" className="ml-0.5 text-danger">
            *
          </span>
          <span className="sr-only"> (required)</span>
        </>
      )}
    </label>
  );
}
