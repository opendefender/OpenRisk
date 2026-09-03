// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * OtpField — the one-time code box, for MFA enrolment and step-up challenges.
 *
 * ONE INPUT, DRAWN AS SEGMENTS. The obvious build is N separate `<input>`s,
 * one per digit, and it is the wrong one. Split inputs break the three things a
 * one-time code depends on most: pasting the code (each box takes one character
 * and the other five are dropped), `autocomplete="one-time-code"` (iOS and
 * Android offer the SMS code to a single field, not to a row of six), and screen
 * readers (six unlabelled boxes announce as six separate fields, and the user
 * has no way to know how many are left). So this is ONE real input, visually
 * segmented: the boxes are presentational and `aria-hidden`, the input sits over
 * them and carries the whole value.
 *
 * WHAT IT REPLACES. Three screens hand-rolled this before it existed —
 * `AuthScreen` twice and `MFAEnrollmentDialog` once — and each got a different
 * subset right. The dialog's version accepted `maxLength={8}` for a six-digit
 * code; none of the three sanitised a pasted value, so a code copied out of an
 * email as "123 456" or "Your code is 123456" was rejected as invalid without
 * ever reaching the server. Sanitising is not a nicety here: the user cannot see
 * why the code they can plainly read was refused.
 *
 * WHY NOT JUST AN INPUT WITH maxLength. Because the segments are what tell the
 * user how long the code is before they start typing, and how many characters
 * remain while they do. That is the entire job of this control; a bare box does
 * not do it.
 */

import { forwardRef, useId, useMemo, type ClipboardEvent, type ChangeEvent } from 'react';
import { cn } from './cn';
import { useControlWiring } from './fieldContext';
import { sanitiseCode as sanitise } from './otpCode';

export type { OtpAlphabet } from './otpCode';
import type { OtpAlphabet } from './otpCode';

export interface OtpFieldProps {
  /** How many characters the code has. Drawn as this many segments. */
  length?: number;
  value: string;
  onValueChange: (next: string) => void;
  /**
   * Fired once, when the value reaches `length`. This is the "submit the form"
   * hook: a user who has typed the last digit has finished, and making them
   * reach for a button afterwards is the most common complaint about this
   * control.
   */
  onComplete?: (value: string) => void;
  /**
   * `numeric` (default) also sets `inputMode="numeric"`, which is what raises
   * the digit keypad on a phone. `alphanumeric` is for recovery/backup codes.
   */
  alphabet?: OtpAlphabet;
  disabled?: boolean;
  /**
   * Accessible name. Required when the control is used OUTSIDE a `Field`, which
   * is the common case here — an MFA dialog labels the code itself.
   */
  label?: string;
  id?: string;
  className?: string;
  autoFocus?: boolean;
  /**
   * Error state for a control used OUTSIDE a `Field`. Inside one the status is
   * inherited and this is unnecessary; the MFA dialogs label the code
   * themselves and carry their own error line, so they need to say it here.
   */
  invalid?: boolean;
  /** Id of an external error/description element to associate. */
  describedBy?: string;
  /** Placed on the real input, for existing E2E hooks. */
  testId?: string;
}

export const OtpField = forwardRef<HTMLInputElement, OtpFieldProps>(function OtpField(
  {
    length = 6,
    value,
    onValueChange,
    onComplete,
    alphabet = 'numeric',
    disabled = false,
    label,
    id,
    className,
    autoFocus,
    invalid = false,
    describedBy,
    testId,
  },
  ref,
) {
  const { id: wiredId, status: fieldStatus, aria } = useControlWiring(id);
  /* A Field, when there is one, is the authority — it already knows whether
       the group is invalid. `invalid` only speaks for a control standing on its
       own, which is how both MFA dialogs use it. */
  const status = fieldStatus !== 'default' ? fieldStatus : invalid ? 'invalid' : 'default';
  const describedById = useId();
  const segments = useMemo(() => Array.from({ length }, (_, i) => i), [length]);

  /* The INCOMING value is sanitised too, not just what the user types. A
       caller hydrating from a store, a URL or a test fixture can hand over
       "123 456", and without this the space takes a segment of its own — the
       code renders with a hole in it and the caret lands one box to the right of
       where the next character will actually go. A segment can only ever hold a
       code character, so making that true of the rendered value is the fix. */
  const shown = useMemo(() => sanitise(value, alphabet, length), [value, alphabet, length]);

  /* The caret sits on the first empty segment, or on the last one when the code
     is full — otherwise a complete code shows no active segment at all and the
     control looks disabled at exactly the moment it is ready to submit. */
  const activeIndex = Math.min(shown.length, length - 1);

  function commit(next: string) {
    if (next === value) return;
    onValueChange(next);
    if (next.length === length) onComplete?.(next);
  }

  function handleChange(e: ChangeEvent<HTMLInputElement>) {
    commit(sanitise(e.target.value, alphabet, length));
  }

  /* Paste is handled explicitly rather than left to `change`, because the raw
     clipboard text is the only place the un-sanitised value is visible — by the
     time `change` fires the browser has already applied `maxLength` and cut a
     spaced "123 456" down to "123 ". */
  function handlePaste(e: ClipboardEvent<HTMLInputElement>) {
    e.preventDefault();
    commit(sanitise(e.clipboardData.getData('text'), alphabet, length));
  }

  return (
    <div className={cn('inline-flex flex-col gap-1', className)}>
      <div className="relative inline-flex" data-disabled={disabled || undefined}>
        {/* The real control. Transparent rather than hidden: `opacity-0` keeps it
            focusable, hit-testable and in the accessibility tree, whereas
            `display:none` or `visibility:hidden` would take it out of all three.
            It spans the segments so a click anywhere lands in it. */}
        <input
          ref={ref}
          id={wiredId}
          type="text"
          /* `shown`, not the raw prop, so the input and the segments can never
               disagree. Any edit then commits the sanitised value upward, which
               is what normalises a dirty prop rather than leaving the parent
               holding a value the control never displayed. */
          value={shown}
          onChange={handleChange}
          onPaste={handlePaste}
          disabled={disabled}
          autoFocus={autoFocus}
          inputMode={alphabet === 'numeric' ? 'numeric' : 'text'}
          autoComplete="one-time-code"
          /* One-time codes are not words. Left on, the keyboard capitalises and
             corrects them, and on iOS the first character of an alphanumeric
             backup code arrives upper-cased against the user's intent. */
          autoCorrect="off"
          autoCapitalize="off"
          spellCheck={false}
          maxLength={length}
          aria-label={label}
          aria-describedby={cn(aria['aria-describedby'], describedBy, describedById) || undefined}
          aria-invalid={status === 'invalid' || undefined}
          data-testid={testId}
          className="absolute inset-0 z-10 h-full w-full cursor-text bg-transparent text-transparent caret-transparent outline-none disabled:cursor-not-allowed"
          {...aria}
        />

        {/* Presentational. Hidden from assistive tech because the input above
            already carries the value — announcing both would read the code
            twice, once as a field and once as six stray characters. */}
        <div aria-hidden="true" className="flex gap-2">
          {segments.map((i) => {
            const char = shown[i] ?? '';
            const isActive = !disabled && i === activeIndex;
            return (
              <span
                key={i}
                data-testid="otp-segment"
                data-filled={char ? '' : undefined}
                className={cn(
                  'flex w-10 items-center justify-center rounded-md border',
                  'h-(--control-h-lg) bg-surface-1',
                  'font-mono text-base tabular-nums text-fg-primary',
                  'transition-[border-color] duration-fast ease-out',
                  status === 'invalid' ? 'border-danger' : 'border-control',
                  /* The focus ring is drawn on the SEGMENT, not the input: the
                     input is transparent and spans all of them, so a ring on it
                     would outline the whole row and lose the caret position that
                     is the only cue for where the next character lands. */
                  isActive && 'border-accent ring-2 ring-accent/30',
                  disabled && 'opacity-55',
                )}
              >
                {char}
              </span>
            );
          })}
        </div>
      </div>

      {/* The length, said once, for a screen reader. A sighted user counts the
          boxes; without this a non-sighted user has no way to know the code is
          six characters and not four or eight. */}
      <span id={describedById} className="sr-only">
        {alphabet === 'numeric' ? `${length}-digit code` : `${length}-character code`}
      </span>
    </div>
  );
});
