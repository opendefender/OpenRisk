// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).
//
// Contract of the CSV imports: POST /risks/import (#755) and POST
// /assets/import (#861) answer the same shapes. The server is the authority on
// every row; the client only refuses what it can know without reading the file,
// and parses every response so the page never shows a number it was not sent.

import { z } from 'zod';

/** Mirrors MaxImportBytes of the server import use cases. */
export const MAX_IMPORT_BYTES = 2 * 1024 * 1024;

/** The catalogue translator, as useI18n().t provides it. */
export type T = (key: string, params?: Record<string, string | number>) => string;

export function importFileSchema(t: T) {
  return z
    .instanceof(File)
    .refine((f) => f.name.toLowerCase().endsWith('.csv'), { message: t('csvImport.onlyCsv') })
    .refine((f) => f.size > 0, { message: t('csvImport.fileEmpty') })
    .refine((f) => f.size <= MAX_IMPORT_BYTES, { message: t('csvImport.tooLarge') });
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

/** Codes the catalogue knows, under csvImport.errors. */
const KNOWN_CODES = new Set([
  'file_empty',
  'not_utf8',
  'header_unreadable',
  'unknown_column',
  'duplicate_column',
  'missing_column',
  'line_unreadable',
  'too_many_rows',
  'cell_count',
  'required',
  'too_long',
  'not_a_number',
  'out_of_range',
  'no_rows',
  'unknown_asset',
  'ambiguous_asset',
  'assets_unavailable',
  'legacy_scale',
  'asset_exists',
  'duplicate_in_file',
  'invalid_criticality',
]);

/**
 * Renders one server error in the reader's language from its code and params
 * (csvImport.errors in the catalogue). The codes mirror ImportRowError on the
 * server; an unknown code falls back to the server's English message rather
 * than to nothing.
 */
export function importErrorMessage(e: ImportRowError, t: T): string {
  if (!e.code || !KNOWN_CODES.has(e.code)) return e.message;
  return t(`csvImport.errors.${e.code}`, { column: e.column ?? '', ...e.params });
}

/** 200: every row was written. */
export const importSuccessSchema = z.object({
  created: z.number().int(),
  rejected: z.number().int(),
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
