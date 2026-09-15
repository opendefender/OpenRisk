// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The questionnaire template editor's form model (#681): the draft the editor
// holds, the Zod rules that mirror the server's (backend domain
// VendorQuestionnaireQuestion.Validate), and the conversions to and from the API.
//
// Error messages are i18n keys under `vendorTemplates.errors`, resolved by the
// editor, so the same schema serves both languages.

import { z } from 'zod';
import type { LocaleCode } from '../../i18n';
import type {
  QuestionnaireTemplate,
  QuestionnaireTemplateInput,
} from './questionnaireTemplateService';

export const MAX_QUESTIONS = 200;
export const MAX_OPTIONS = 20;
export const MAX_TEXT = 2000;
export const MAX_NAME = 200;

export type AnswerType = 'choice' | 'text';
// The locale registry owns the set of languages (src/i18n/locales.ts). The
// server accepts fr and en for a questionnaire; toTemplateInput narrows to them.
export type TemplateLanguage = LocaleCode;

export interface OptionDraft {
  /** React key, local only. */
  key: string;
  /** The stored option value. Kept across edits so answers stay matched. */
  value: string;
  label: string;
  points: number;
}

export interface QuestionDraft {
  key: string;
  text: string;
  help: string;
  answerType: AnswerType;
  weight: number;
  required: boolean;
  naAllowed: boolean;
  controlRef: string;
  options: OptionDraft[];
}

export interface TemplateDraft {
  name: string;
  description: string;
  language: TemplateLanguage;
  questions: QuestionDraft[];
}

let keySeq = 0;
function nextKey(prefix: string): string {
  keySeq += 1;
  return `${prefix}-${keySeq}`;
}

/** A fresh option value, unique within the question. */
function freshValue(options: ReadonlyArray<OptionDraft>): string {
  const taken = new Set(options.map((o) => o.value));
  let n = options.length + 1;
  while (taken.has(`option-${n}`)) n += 1;
  return `option-${n}`;
}

export function newOption(
  existing: ReadonlyArray<OptionDraft>,
  label = '',
  points = 0,
): OptionDraft {
  return { key: nextKey('opt'), value: freshValue(existing), label, points };
}

export function newQuestion(language: TemplateLanguage): QuestionDraft {
  const yes = newOption([], language === 'fr' ? 'Oui' : 'Yes', 1);
  const no = newOption([yes], language === 'fr' ? 'Non' : 'No', 0);
  return {
    key: nextKey('q'),
    text: '',
    help: '',
    answerType: 'choice',
    weight: 5,
    required: true,
    naAllowed: false,
    controlRef: '',
    options: [yes, no],
  };
}

export function emptyDraft(language: TemplateLanguage): TemplateDraft {
  return { name: '', description: '', language, questions: [newQuestion(language)] };
}

export function fromTemplate(t: QuestionnaireTemplate): TemplateDraft {
  const language: TemplateLanguage = t.language === 'en' ? 'en' : 'fr';
  return {
    name: t.name,
    description: t.description ?? '',
    language,
    questions: [...(t.questions ?? [])]
      .sort((a, b) => a.position - b.position)
      .map((q) => ({
        key: nextKey('q'),
        text: q.text,
        help: q.help ?? '',
        answerType: q.answer_type === 'text' ? 'text' : 'choice',
        weight: q.weight ?? 0,
        required: q.required ?? false,
        naAllowed: q.na_allowed ?? false,
        controlRef: q.control_ref ?? '',
        options: (q.options ?? []).map((o) => ({
          key: nextKey('opt'),
          value: o.value,
          label: o.label,
          points: o.points,
        })),
      })),
  };
}

export function toTemplateInput(draft: TemplateDraft): QuestionnaireTemplateInput {
  return {
    name: draft.name.trim(),
    description: draft.description.trim(),
    language: draft.language === 'en' ? 'en' : 'fr',
    questions: draft.questions.map((q) =>
      q.answerType === 'text'
        ? {
            text: q.text.trim(),
            help: q.help.trim(),
            answer_type: 'text',
            weight: 0,
            required: q.required,
            na_allowed: q.naAllowed,
            control_ref: q.controlRef.trim(),
          }
        : {
            text: q.text.trim(),
            help: q.help.trim(),
            answer_type: 'choice',
            weight: q.weight,
            required: q.required,
            na_allowed: q.naAllowed,
            control_ref: q.controlRef.trim(),
            options: q.options.map((o) => ({
              value: o.value,
              label: o.label.trim(),
              points: o.points,
            })),
          },
    ),
  };
}

// ---------------------------------------------------------------------------
// Zod — the server's rules, so the author learns before the round trip.
// ---------------------------------------------------------------------------

const optionSchema = z.object({
  key: z.string(),
  value: z.string().min(1),
  label: z.string().trim().min(1, 'optionLabel'),
  // z.custom rather than z.number(): a cleared number input is NaN, and the
  // built-in type check would answer with its own untranslated message.
  points: z.custom<number>((v) => typeof v === 'number' && v >= 0 && v <= 1, {
    message: 'points',
  }),
});

const questionSchema = z
  .object({
    key: z.string(),
    text: z.string().trim().min(1, 'questionText').max(MAX_TEXT, 'questionTextLong'),
    help: z.string().max(MAX_TEXT, 'questionTextLong'),
    answerType: z.enum(['choice', 'text']),
    // Range-checked below, for choice questions only (a text question never scores).
    weight: z.custom<number>((v) => typeof v === 'number'),
    required: z.boolean(),
    naAllowed: z.boolean(),
    controlRef: z.string(),
    options: z.array(optionSchema),
  })
  .superRefine((q, ctx) => {
    if (q.answerType !== 'choice') return;
    if (!(q.weight >= 0 && q.weight <= 10)) {
      ctx.addIssue({ code: 'custom', path: ['weight'], message: 'weight' });
    }
    if (q.options.length < 2) {
      ctx.addIssue({ code: 'custom', path: ['options'], message: 'twoOptions' });
    }
    if (q.options.length > MAX_OPTIONS) {
      ctx.addIssue({ code: 'custom', path: ['options'], message: 'tooManyOptions' });
    }
    const seen = new Set<string>();
    q.options.forEach((o, i) => {
      const label = o.label.trim().toLowerCase();
      if (label && seen.has(label)) {
        ctx.addIssue({ code: 'custom', path: ['options', i, 'label'], message: 'uniqueOptions' });
      }
      seen.add(label);
    });
  });

export const templateSchema = z.object({
  name: z.string().trim().min(1, 'name').max(MAX_NAME, 'nameLong'),
  description: z.string(),
  language: z.enum(['fr', 'en']),
  questions: z.array(questionSchema).min(1, 'oneQuestion').max(MAX_QUESTIONS, 'tooManyQuestions'),
});

/** Validation errors keyed by dotted path ('name', 'questions.0.options.1.label'). */
export type FormErrors = Record<string, string>;

export function validateTemplate(draft: TemplateDraft): FormErrors {
  const result = templateSchema.safeParse(draft);
  if (result.success) return {};
  const errors: FormErrors = {};
  for (const issue of result.error.issues) {
    const path = issue.path.join('.');
    if (!(path in errors)) errors[path] = issue.message;
  }
  return errors;
}

/** Moves the item at `from` to `to`, returning a new array. */
export function move<T>(items: ReadonlyArray<T>, from: number, to: number): T[] {
  if (to < 0 || to >= items.length || from === to) return [...items];
  const next = [...items];
  const [item] = next.splice(from, 1);
  next.splice(to, 0, item);
  return next;
}
