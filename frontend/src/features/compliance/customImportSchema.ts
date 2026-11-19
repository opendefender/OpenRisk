// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Client-side contract of the custom framework JSON import (#307). The server
// stays the authority (it re-validates everything and writes all or nothing);
// this only refuses what can be known before the request, with the same limits
// as the Go use case, so a bad file is explained before it is sent.

import { z } from 'zod';

import type { FrameworkExportDocument } from '../../types/compliance';

/** Mirrors the server's body limit. */
export const MAX_FRAMEWORK_IMPORT_BYTES = 2 * 1024 * 1024;
export const MAX_CONTROLS = 2000;
export const MAX_PROBLEMS = 100;

const LIMITS = {
  frameworkName: 255,
  frameworkVersion: 50,
  description: 10000,
  referenceCode: 50,
  controlName: 255,
  sourceReference: 255,
} as const;

/** The catalogue translator, as useI18n().t provides it. */
export type T = (key: string, params?: Record<string, string | number>) => string;

export function frameworkFileSchema(t: T) {
  return z
    .instanceof(File)
    .refine((f) => f.name.toLowerCase().endsWith('.json'), {
      message: t('compliance.customImport.onlyJson'),
    })
    .refine((f) => f.size > 0, { message: t('compliance.customImport.fileEmpty') })
    .refine((f) => f.size <= MAX_FRAMEWORK_IMPORT_BYTES, {
      message: t('compliance.customImport.tooLarge'),
    });
}

/** One problem, in the shape of the server's errors[] entries. */
export interface Problem {
  /** 0-based position in controls[]; null for the document or framework. */
  index: number | null;
  field: string;
  message: string;
}

// A JSON object of unknown fields; each one is checked below.
const objectShape = z.record(z.string(), z.unknown());

function textProblem(
  value: unknown,
  index: number | null,
  field: string,
  max: number,
  required: boolean,
  t: T,
  out: Problem[],
): string {
  if (value === undefined || value === null) {
    if (required) {
      out.push({ index, field, message: t('compliance.customImport.problems.required') });
    }
    return '';
  }
  if (typeof value !== 'string') {
    out.push({ index, field, message: t('compliance.customImport.problems.notText') });
    return '';
  }
  const text = value.trim();
  if (required && text === '') {
    out.push({ index, field, message: t('compliance.customImport.problems.required') });
  } else if ([...value].length > max) {
    out.push({
      index,
      field,
      message: t('compliance.customImport.problems.tooLong', { max }),
    });
  }
  return value;
}

export type ParseResult =
  | { ok: true; document: FrameworkExportDocument }
  | { ok: false; problems: Problem[]; truncated: boolean };

/** Parses file text and checks its shape. Never throws. */
export function parseFrameworkDocument(text: string, t: T): ParseResult {
  const fail = (problems: Problem[]): ParseResult => ({
    ok: false,
    problems: problems.slice(0, MAX_PROBLEMS),
    truncated: problems.length > MAX_PROBLEMS,
  });

  let raw: unknown;
  try {
    raw = JSON.parse(text);
  } catch {
    return fail([
      { index: null, field: 'file', message: t('compliance.customImport.problems.notJson') },
    ]);
  }

  const doc = objectShape.safeParse(raw);
  if (!doc.success) {
    return fail([
      { index: null, field: 'file', message: t('compliance.customImport.problems.notObject') },
    ]);
  }

  const problems: Problem[] = [];
  if (doc.data.schema_version !== 1) {
    problems.push({
      index: null,
      field: 'schema_version',
      message: t('compliance.customImport.problems.schemaVersion'),
    });
  }

  const fwParsed = objectShape.safeParse(doc.data.framework);
  const fw: Record<string, unknown> = fwParsed.success ? fwParsed.data : {};
  const name = textProblem(
    fw.name,
    null,
    'framework.name',
    LIMITS.frameworkName,
    true,
    t,
    problems,
  );
  const version = textProblem(
    fw.version,
    null,
    'framework.version',
    LIMITS.frameworkVersion,
    false,
    t,
    problems,
  );
  const description = textProblem(
    fw.description,
    null,
    'framework.description',
    LIMITS.description,
    false,
    t,
    problems,
  );

  const controls: FrameworkExportDocument['controls'] = [];
  if (!Array.isArray(doc.data.controls) || doc.data.controls.length === 0) {
    problems.push({
      index: null,
      field: 'controls',
      message: t('compliance.customImport.problems.noControls'),
    });
  } else if (doc.data.controls.length > MAX_CONTROLS) {
    problems.push({
      index: null,
      field: 'controls',
      message: t('compliance.customImport.problems.tooManyControls', { max: MAX_CONTROLS }),
    });
  } else {
    const seen = new Set<string>();
    doc.data.controls.forEach((item: unknown, index: number) => {
      const c = objectShape.safeParse(item);
      if (!c.success) {
        problems.push({
          index,
          field: 'control',
          message: t('compliance.customImport.problems.notObject'),
        });
        return;
      }
      const code = textProblem(
        c.data.reference_code,
        index,
        'reference_code',
        LIMITS.referenceCode,
        true,
        t,
        problems,
      ).trim();
      if (code !== '') {
        if (seen.has(code)) {
          problems.push({
            index,
            field: 'reference_code',
            message: t('compliance.customImport.problems.duplicateCode', { code }),
          });
        }
        seen.add(code);
      }
      const cname = textProblem(c.data.name, index, 'name', LIMITS.controlName, true, t, problems);
      const cdesc = textProblem(
        c.data.description,
        index,
        'description',
        LIMITS.description,
        false,
        t,
        problems,
      );
      const source = textProblem(
        c.data.source_reference,
        index,
        'source_reference',
        LIMITS.sourceReference,
        false,
        t,
        problems,
      );
      controls.push({
        reference_code: code,
        name: cname,
        description: cdesc,
        source_reference: source,
      });
    });
  }

  if (problems.length > 0) return fail(problems);
  return {
    ok: true,
    document: {
      schema_version: 1,
      framework: { name, version, description },
      controls,
    },
  };
}

/** 400 from the server: the message and, when present, the offending rows. */
export const importRejectedSchema = z.object({
  error: z.string().optional(),
  errors: z
    .array(
      z.object({
        index: z.number().int().nullable(),
        field: z.string(),
        message: z.string(),
      }),
    )
    .optional(),
});
