// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).
//
// Contract of POST /risks/import (#755). The server is the authority on every
// row; the client only refuses what it can know without reading the file, and
// parses every response so the page never shows a number it was not sent.

import { z } from 'zod';

/** Mirrors risk.MaxImportBytes on the server. */
export const MAX_IMPORT_BYTES = 2 * 1024 * 1024;

type Tr = (fr: string, en: string) => string;

export function importFileSchema(tr: Tr) {
  return z
    .instanceof(File)
    .refine((f) => f.name.toLowerCase().endsWith('.csv'), {
      message: tr('Seuls les fichiers CSV sont acceptés.', 'Only CSV files are accepted.'),
    })
    .refine((f) => f.size > 0, { message: tr('Le fichier est vide.', 'The file is empty.') })
    .refine((f) => f.size <= MAX_IMPORT_BYTES, {
      message: tr(
        'Le fichier dépasse 2 Mo. Découpez-le en plusieurs fichiers.',
        'The file is larger than 2 MB. Split it into several files.',
      ),
    });
}

export const importRowErrorSchema = z.object({
  line: z.number().int(),
  column: z.string().optional(),
  message: z.string(),
});
export type ImportRowError = z.infer<typeof importRowErrorSchema>;

/** 200: every row was written. */
export const importSuccessSchema = z.object({
  created: z.number().int(),
  rejected: z.number().int(),
  risk_ids: z.array(z.string()),
  errors: z.array(importRowErrorSchema),
});
export type ImportSuccess = z.infer<typeof importSuccessSchema>;

/** 422: at least one row was invalid, nothing was written. */
export const importRejectedSchema = z.object({
  created: z.literal(0),
  rejected: z.number().int(),
  errors: z.array(importRowErrorSchema).min(1),
});
export type ImportRejected = z.infer<typeof importRejectedSchema>;

/** 402: the file would take the tenant past its plan's risk limit. */
export const importLimitSchema = z.object({
  code: z.literal('limit_reached'),
  requested: z.number().int().optional(),
  remaining: z.number().int().optional(),
});
export type ImportLimit = z.infer<typeof importLimitSchema>;

/** The current template: the product's scales, P in [0,1] and I in [0,10]. */
export const IMPORT_TEMPLATE = [
  'title,description,probability,impact,tags,frameworks',
  '"Phishing campaign against finance staff","Credential theft leading to fraudulent transfers",0.6,8,"email;people","ISO27001"',
  '"Ransomware on file servers","Encryption of shared drives, no tested restore",0.3,10,"backup","ISO27001;NIST CSF"',
  '"Cloud provider outage","Loss of the hosted CRM for more than 24 hours",0.2,5,"supplier",',
].join('\n');
