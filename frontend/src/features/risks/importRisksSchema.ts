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
  /** Stable code; the page renders it in the reader's language. */
  code: z.string().optional(),
  params: z.record(z.string(), z.string()).optional(),
  /** The server's English rendering, shown when a code is unknown. */
  message: z.string(),
});
export type ImportRowError = z.infer<typeof importRowErrorSchema>;

/**
 * Renders one server error in the reader's language. The codes mirror
 * risk.ImportRowError on the server (TestImportRisks_ErrorsCarryCodesForTranslation);
 * an unknown code falls back to the server's English message rather than to
 * nothing.
 */
export function importErrorMessage(e: ImportRowError, tr: Tr): string {
  const p = e.params ?? {};
  const col = p.column ?? e.column ?? '';
  switch (e.code) {
    case 'file_empty':
      return tr('Le fichier est vide.', 'The file is empty.');
    case 'not_utf8':
      return tr(
        'Le fichier n’est pas en UTF-8 : enregistrez-le au format « CSV UTF-8 ».',
        'The file is not UTF-8 text: save it as "CSV UTF-8".',
      );
    case 'header_unreadable':
      return tr('L’en-tête est illisible.', 'The header cannot be read.');
    case 'unknown_column':
      return tr(
        `Colonne inconnue « ${col} ». Colonnes acceptées : ${p.accepted ?? ''}.`,
        `Unknown column "${col}". Accepted columns: ${p.accepted ?? ''}.`,
      );
    case 'duplicate_column':
      return tr(`La colonne « ${col} » apparaît deux fois.`, `Column "${col}" appears twice.`);
    case 'missing_column':
      return tr(
        `La colonne obligatoire « ${col} » est absente.`,
        `Required column "${col}" is missing.`,
      );
    case 'line_unreadable':
      return tr(
        'Cette ligne est illisible (guillemet non fermé ?).',
        'This line cannot be read (unclosed quote?).',
      );
    case 'too_many_rows':
      return tr(
        `Le fichier dépasse ${p.max ?? ''} lignes : découpez-le en plusieurs fichiers.`,
        `The file has more than ${p.max ?? ''} rows: split it into several files.`,
      );
    case 'cell_count':
      return tr(
        `La ligne a ${p.cells ?? ''} cellules, l’en-tête en a ${p.header ?? ''}.`,
        `The line has ${p.cells ?? ''} cells but the header has ${p.header ?? ''}.`,
      );
    case 'required':
      return tr('Valeur obligatoire.', 'A value is required.');
    case 'too_long':
      return tr(`${p.max ?? ''} caractères au plus.`, `At most ${p.max ?? ''} characters.`);
    case 'not_a_number':
      return tr(`« ${p.value ?? ''} » n’est pas un nombre.`, `"${p.value ?? ''}" is not a number.`);
    case 'out_of_range':
      return tr(
        `Doit être entre ${p.min ?? ''} et ${p.max ?? ''} (valeur : ${p.value ?? ''}).`,
        `Must be between ${p.min ?? ''} and ${p.max ?? ''} (got ${p.value ?? ''}).`,
      );
    case 'no_rows':
      return tr(
        'Le fichier a un en-tête mais aucune ligne de risque.',
        'The file has a header but no risk rows.',
      );
    case 'legacy_scale':
      return tr(
        'Ce fichier utilise l’ancienne échelle 1–5. OpenRisk attend une probabilité entre 0 et 1 et un impact entre 0 et 10. Téléchargez le modèle actuel et convertissez les valeurs (probabilité 3/5 → 0,6 ; impact 4/5 → 8).',
        'This file uses the old 1–5 scale. OpenRisk expects probability between 0 and 1 and impact between 0 and 10. Download the current template and convert the values (probability 3/5 → 0.6, impact 4/5 → 8).',
      );
    default:
      return e.message;
  }
}

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
