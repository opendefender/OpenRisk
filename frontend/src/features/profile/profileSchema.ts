// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Client-side rules for the profile form (#719). They mirror
// domain.UserProfilePatch.Normalize; the server remains the authority.

import { z } from 'zod';

type Tr = (fr: string, en: string) => string;

export const THEME_MODES = ['light', 'dark', 'system'] as const;
export const DATE_PATTERNS = ['DD/MM/YYYY', 'MM/DD/YYYY', 'YYYY-MM-DD'] as const;

export function profileSchema(tr: Tr) {
  return z.object({
    full_name: z
      .string()
      .trim()
      .min(1, tr('Indiquez votre nom.', 'Enter your name.'))
      .max(120, tr('120 caractères au plus.', 'At most 120 characters.')),
    job_title: z.string().trim().max(80, tr('80 caractères au plus.', 'At most 80 characters.')),
    phone: z
      .string()
      .trim()
      .refine(
        (v) => v === '' || /^\+?[0-9 ().-]{4,32}$/.test(v),
        tr('Numéro de téléphone invalide.', 'Invalid phone number.'),
      ),
    bio: z.string().trim().max(500, tr('500 caractères au plus.', 'At most 500 characters.')),
    timezone: z.string(),
    locale: z.string(),
    date_format: z.union([z.enum(DATE_PATTERNS), z.literal('')]),
    theme_mode: z.union([z.enum(THEME_MODES), z.literal('')]),
  });
}

export type ProfileValues = z.infer<ReturnType<typeof profileSchema>>;
