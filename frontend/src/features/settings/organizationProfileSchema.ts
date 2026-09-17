// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Client-side rules for the organization profile form (#299). They mirror
// domain.OrganizationProfilePatch.Normalize; the server remains the authority.

import { z } from 'zod';

import { DATE_FORMATS, ORG_SIZES } from '../organization/organizationService';

type Tr = (fr: string, en: string) => string;

export function organizationProfileSchema(tr: Tr) {
  const optional = (max: number, fr: string, en: string) => z.string().trim().max(max, tr(fr, en));
  return z.object({
    name: z
      .string()
      .trim()
      .min(2, tr('Au moins 2 caractères.', 'At least 2 characters.'))
      .max(120, tr('120 caractères au plus.', 'At most 120 characters.')),
    industry: optional(80, '80 caractères au plus.', 'At most 80 characters.'),
    size: z.union([z.enum(ORG_SIZES as [string, ...string[]]), z.literal('')]),
    website: z
      .string()
      .trim()
      .max(255, tr('255 caractères au plus.', 'At most 255 characters.'))
      .refine(
        (v) => {
          if (v === '') return true;
          try {
            const u = new URL(v);
            return u.protocol === 'https:' && u.host !== '';
          } catch {
            return false;
          }
        },
        tr('Adresse complète en https:// attendue.', 'A full https:// address is expected.'),
      ),
    description: optional(500, '500 caractères au plus.', 'At most 500 characters.'),
    timezone: z.string(),
    default_locale: z.string(),
    date_format: z.union([z.enum(DATE_FORMATS as [string, ...string[]]), z.literal('')]),
  });
}

export type OrganizationProfileValues = z.infer<ReturnType<typeof organizationProfileSchema>>;
