// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

/**
 * Merge class names, letting a later class win over an earlier one in the same
 * Tailwind group.
 *
 * One definition. It previously existed twice — exported from
 * components/ui/Button.tsx (and imported from there by Input, which is how a
 * button file became a utility module) and redefined privately in Badge.tsx.
 */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
