// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * The wiring a Field hands to whatever control sits inside it.
 *
 * This lives beside Field.tsx rather than inside it for a mechanical reason:
 * a file that exports both components and hooks breaks React Fast Refresh
 * (`react-refresh/only-export-components`), and the lint rule that says so is
 * at error level here. The rule is right — a module exporting a component and a
 * hook cannot be hot-replaced reliably — so the shared wiring gets its own file.
 *
 * It is deliberately NOT re-exported from index.ts. This is how the module
 * builds controls; a screen composes `Field` + a control and never sees it.
 */

import { createContext, useContext, useId } from 'react';
import { cn } from './cn';

export type FieldStatus = 'default' | 'invalid' | 'warning' | 'success';

export interface FieldContextValue {
  id: string;
  descriptionId?: string;
  messageId?: string;
  status: FieldStatus;
  required: boolean;
  disabled: boolean;
}

export const FieldContext = createContext<FieldContextValue | null>(null);

/**
 * Controls read their wiring from context; using one outside a Field is legal
 * (a bare search box needs no label stack) and simply yields no ids.
 */
export function useFieldContext(): FieldContextValue | null {
  return useContext(FieldContext);
}

/**
 * Wires a control to its Field. Caller-supplied ids/aria always win.
 *
 * Shared by every control in this directory so that one written later inherits
 * the same wiring instead of reimplementing it — which is how three of the four
 * screens that predated Field ended up associating the label but not the error.
 */
export function useControlWiring(explicitId?: string) {
  const ctx = useFieldContext();
  const fallbackId = useId();
  if (!ctx) return { id: explicitId ?? fallbackId, status: 'default' as FieldStatus, aria: {} };
  const describedBy = [ctx.descriptionId, ctx.messageId].filter(Boolean).join(' ') || undefined;
  return {
    id: explicitId ?? ctx.id,
    status: ctx.status,
    aria: {
      'aria-describedby': describedBy,
      'aria-invalid': ctx.status === 'invalid' || undefined,
      'aria-required': ctx.required || undefined,
      disabled: ctx.disabled || undefined,
    },
  };
}

/* The border is the state. --border-control is the contrast-verified default;
   invalid/warning/success swap it for the matching semantic colour and are
   always accompanied by text in the message slot. */
const STATUS_BORDER: Record<FieldStatus, string> = {
  default: 'border-control',
  invalid: 'border-danger',
  warning: 'border-warning',
  success: 'border-success',
};

/** Shared by Input, Textarea, Select and Listbox so the four look like one family. */
export function controlClasses(status: FieldStatus, extra?: string): string {
  return cn(
    'w-full bg-surface-1 text-fg-primary placeholder:text-fg-muted',
    'border rounded-md px-(--control-px-md)',
    'transition-[border-color,background-color] duration-fast ease-out',
    'hover:border-strong',
    'focus:border-accent',
    'disabled:opacity-55 disabled:pointer-events-none',
    'read-only:bg-surface-sunken read-only:text-fg-secondary',
    STATUS_BORDER[status],
    extra,
  );
}
