// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Code sanitising, in its own module.
 *
 * Split out of OtpField.tsx for the same mechanical reason fieldContext.ts is
 * split out of Field.tsx: a module that exports both a component and a plain
 * function cannot be hot-replaced reliably, and `react-refresh/only-export-components`
 * is at error level here.
 *
 * It has a second consumer, which is why it is exported at all. The MFA LOGIN
 * field is not an OtpField — it accepts a 6-digit TOTP code OR a 12-character
 * recovery code, so it has no fixed length to draw segments for — but it needs
 * exactly this cleaning, because the defect being fixed (a pasted code with
 * spaces or surrounding words silently rejected) is worst on the login path.
 */

export type OtpAlphabet = 'numeric' | 'alphanumeric';

const PATTERN: Record<OtpAlphabet, RegExp> = {
  numeric: /[^0-9]/g,
  alphanumeric: /[^a-zA-Z0-9]/g,
};

/**
 * Keeps only the characters the alphabet allows, and truncates to `length` when
 * one is given. Omit `length` for a code whose length is not fixed.
 *
 * Deliberately tolerant of what a real paste contains: spaces and hyphens from a
 * formatted code, and the surrounding words when someone selects a whole line
 * out of an email. Anything that is not a code character is simply not a code
 * character.
 */
export function sanitiseCode(raw: string, alphabet: OtpAlphabet, length?: number): string {
  const cleaned = raw.replace(PATTERN[alphabet], '');
  const cased = alphabet === 'alphanumeric' ? cleaned.toUpperCase() : cleaned;
  return length === undefined ? cased : cased.slice(0, length);
}
