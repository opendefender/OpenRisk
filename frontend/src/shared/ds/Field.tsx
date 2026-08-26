// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1

/**
 * Form system — Field, Input, Textarea, Select.
 *
 * ANATOMY   Label [required] ─ description ─ [control] ─ error | hint
 *
 * The point of Field is the wiring, not the layout. Before this, every feature
 * assembled its own label/input/error stack, which meant every feature decided
 * independently whether to associate the label (`htmlFor`), whether to announce
 * the error (`aria-describedby` + `role="alert"`), and whether to mark the
 * control invalid (`aria-invalid`). Most did one of the three.
 *
 * Field does all of it from one `id`, generated once and threaded to the
 * control, the description and the error. A screen author cannot forget it
 * because they never write it.
 *
 * STATES    default, hover, focus, filled, disabled, readonly, invalid,
 *           warning, success, loading.
 *
 * A11Y      - label ↔ control via htmlFor/id
 *           - description and error both reach the control through
 *             aria-describedby, so a screen reader gets the hint AND the reason
 *             it was rejected, in that order
 *           - aria-invalid on error; the error text is role="alert" so it is
 *             announced when it appears (WCAG 3.3.1)
 *           - required is marked in the accessible name AND visually, never by
 *             colour alone (WCAG 1.4.1)
 *           - the invalid state changes the border AND shows text; a red ring
 *             on its own is not an error message
 */

import {
  createContext,
  forwardRef,
  useContext,
  useId,
  type InputHTMLAttributes,
  type ReactNode,
  type SelectHTMLAttributes,
  type TextareaHTMLAttributes,
} from 'react';
import { Loader2 } from 'lucide-react';
import { cn } from './cn';

export type FieldStatus = 'default' | 'invalid' | 'warning' | 'success';

interface FieldContextValue {
  id: string;
  descriptionId?: string;
  messageId?: string;
  status: FieldStatus;
  required: boolean;
  disabled: boolean;
}

const FieldContext = createContext<FieldContextValue | null>(null);

/** Controls read their wiring from context; using one outside a Field is legal
 *  (a bare search box needs no label stack) and simply yields no ids. */
function useFieldContext(): FieldContextValue | null {
  return useContext(FieldContext);
}

export interface FieldProps {
  label?: ReactNode;
  /** Sits under the label, before the control: what this field is for. */
  description?: ReactNode;
  /** The reason the value was rejected. Presence does NOT imply invalid —
   *  pass status to say so, since the same slot carries warnings and hints. */
  message?: ReactNode;
  status?: FieldStatus;
  required?: boolean;
  disabled?: boolean;
  /** Override the generated id when a caller already owns one. */
  htmlFor?: string;
  className?: string;
  children: ReactNode;
}

export function Field({
  label,
  description,
  message,
  status = 'default',
  required = false,
  disabled = false,
  htmlFor,
  className,
  children,
}: FieldProps) {
  const generated = useId();
  const id = htmlFor ?? generated;
  const descriptionId = description ? `${id}-description` : undefined;
  const messageId = message ? `${id}-message` : undefined;

  const messageTone =
    status === 'invalid'
      ? 'text-danger-text'
      : status === 'warning'
        ? 'text-warning-text'
        : status === 'success'
          ? 'text-success-text'
          : 'text-text-muted';

  return (
    <FieldContext.Provider value={{ id, descriptionId, messageId, status, required, disabled }}>
      <div className={cn('flex flex-col gap-1.5', className)}>
        {label && (
          <label
            htmlFor={id}
            className={cn(
              'text-xs font-medium text-text-secondary',
              disabled && 'opacity-55',
            )}
          >
            {label}
            {required && (
              <>
                {/* Visible marker plus a word in the accessible name: an
                    asterisk alone is colour-and-glyph, which a screen reader
                    reads as "star" if it reads it at all. */}
                <span aria-hidden="true" className="ml-0.5 text-danger">
                  *
                </span>
                <span className="sr-only"> (required)</span>
              </>
            )}
          </label>
        )}

        {description && (
          <p id={descriptionId} className="text-2xs leading-snug text-text-muted">
            {description}
          </p>
        )}

        {children}

        {message && (
          <p
            id={messageId}
            /* Announced on appearance. Only errors are assertive — a hint that
               interrupts is worse than a hint that waits. */
            role={status === 'invalid' ? 'alert' : undefined}
            className={cn('text-2xs leading-snug', messageTone)}
          >
            {message}
          </p>
        )}
      </div>
    </FieldContext.Provider>
  );
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

function controlClasses(status: FieldStatus, extra?: string): string {
  return cn(
    'w-full bg-surface-1 text-text-primary placeholder:text-text-muted',
    'border rounded-md px-[var(--control-px-md)]',
    'transition-[border-color,background-color] duration-fast ease-out',
    'hover:border-strong',
    'focus:border-accent',
    'disabled:opacity-55 disabled:pointer-events-none',
    'read-only:bg-surface-sunken read-only:text-text-secondary',
    STATUS_BORDER[status],
    extra,
  );
}

/** Wires a control to its Field. Caller-supplied ids/aria always win. */
function useControlWiring(explicitId?: string) {
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

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  /** Shows a spinner in the trailing slot (async validation, remote lookup). */
  loading?: boolean;
  /** Rendered inside the control, before the text. */
  leadingIcon?: ReactNode;
  trailingSlot?: ReactNode;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { className, id, loading, leadingIcon, trailingSlot, ...rest },
  ref,
) {
  const { id: fieldId, status, aria } = useControlWiring(id);
  const hasLead = Boolean(leadingIcon);
  const hasTrail = Boolean(trailingSlot) || loading;

  const control = (
    <input
      ref={ref}
      id={fieldId}
      className={cn(
        controlClasses(status, 'h-[var(--control-h-md)] text-sm'),
        hasLead && 'pl-9',
        hasTrail && 'pr-9',
        className,
      )}
      {...aria}
      {...rest}
    />
  );

  if (!hasLead && !hasTrail) return control;

  return (
    <div className="relative">
      {hasLead && (
        <span
          aria-hidden="true"
          className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-text-muted"
        >
          {leadingIcon}
        </span>
      )}
      {control}
      {hasTrail && (
        <span className="absolute right-3 top-1/2 -translate-y-1/2 text-text-muted">
          {loading ? (
            <Loader2 size={14} className="motion-safe:animate-spin" aria-hidden="true" />
          ) : (
            trailingSlot
          )}
        </span>
      )}
    </div>
  );
});

export type TextareaProps = TextareaHTMLAttributes<HTMLTextAreaElement>;

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
  { className, id, rows = 4, ...rest },
  ref,
) {
  const { id: fieldId, status, aria } = useControlWiring(id);
  return (
    <textarea
      ref={ref}
      id={fieldId}
      rows={rows}
      className={cn(controlClasses(status, 'py-2 text-sm resize-y min-h-[80px]'), className)}
      {...aria}
      {...rest}
    />
  );
});

export type SelectProps = SelectHTMLAttributes<HTMLSelectElement>;

export const Select = forwardRef<HTMLSelectElement, SelectProps>(function Select(
  { className, id, children, ...rest },
  ref,
) {
  const { id: fieldId, status, aria } = useControlWiring(id);
  return (
    /* A native select. It gets the platform's keyboard behaviour, the
       platform's mobile picker and the platform's screen-reader support for
       free — all of which a custom listbox has to reimplement and usually
       reimplements incompletely. The custom one exists (see Combobox usage in
       DataTable) for the cases native cannot do: search and multi-select. */
    <select
      ref={ref}
      id={fieldId}
      className={cn(
        controlClasses(status, 'h-[var(--control-h-md)] text-sm pr-8 appearance-none'),
        'bg-[image:var(--select-caret)] bg-[length:10px] bg-[position:right_12px_center] bg-no-repeat',
        className,
      )}
      {...aria}
      {...rest}
    >
      {children}
    </select>
  );
});
