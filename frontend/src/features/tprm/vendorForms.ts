// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The vendor screens' two forms (#673): linking an asset to a vendor, and
// sending a questionnaire. The Zod rules mirror the server's
// (domain.IsCreatableVendorLinkVerb, domain.NewVendorAssessment,
// SendVendorAssessmentUseCase), so the user learns before the round trip.
//
// Error messages are i18n keys, resolved by the screen.

import { z } from 'zod';
import type { LocaleCode } from '../../i18n';
import type { SendVendorAssessmentInput, VendorLinkVerb } from './vendorService';

/** The verbs a vendor link may be created with, in the backend's order. */
export const LINK_VERBS: readonly VendorLinkVerb[] = [
  'managed_by',
  'hosted_by',
  'processes_data_of',
  'depends_on',
];

/** Validation errors keyed by field name. */
export type FormErrors = Record<string, string>;

function collect(result: z.ZodSafeParseResult<unknown>): FormErrors {
  if (result.success) return {};
  const errors: FormErrors = {};
  for (const issue of result.error.issues) {
    const path = issue.path.join('.');
    if (!(path in errors)) errors[path] = issue.message;
  }
  return errors;
}

// ---------------------------------------------------------------------------
// Link an asset
// ---------------------------------------------------------------------------

export interface LinkDraft {
  assetId: string;
  verb: string;
}

const linkSchema = z.object({
  assetId: z.string().min(1, 'asset'),
  verb: z.string().refine((v) => (LINK_VERBS as readonly string[]).includes(v), 'verb'),
});

export function validateLink(draft: LinkDraft): FormErrors {
  return collect(linkSchema.safeParse(draft));
}

// ---------------------------------------------------------------------------
// Send a questionnaire
// ---------------------------------------------------------------------------

export interface SendDraft {
  templateId: string;
  /** yyyy-mm-dd, from a date input. */
  dueDate: string;
  contactEmail: string;
  /** '' leaves the choice to the server: the questionnaire's own language. */
  contactLanguage: LocaleCode | '';
}

const DATE = /^(\d{4})-(\d{2})-(\d{2})$/;
const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

/**
 * The end of the chosen day, in the user's time zone. A vendor given "the 30th"
 * has the whole of the 30th, not until midnight at its start.
 */
export function endOfDay(date: string): Date | null {
  const m = DATE.exec(date);
  if (!m) return null;
  const d = new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]), 23, 59, 59);
  // new Date(2026, 1, 31) silently rolls over to March: refuse it instead.
  if (d.getMonth() !== Number(m[2]) - 1) return null;
  return d;
}

/** yyyy-mm-dd for a local date. */
export function toDateInput(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

export function emptySendDraft(now: Date): SendDraft {
  const due = new Date(now);
  due.setDate(due.getDate() + 14);
  return { templateId: '', dueDate: toDateInput(due), contactEmail: '', contactLanguage: '' };
}

export interface SendRules {
  now: Date;
  /** True when the vendor has no contact email on record, so the field must be filled. */
  emailRequired: boolean;
}

export function validateSend(draft: SendDraft, rules: SendRules): FormErrors {
  const schema = z
    .object({
      templateId: z.string().min(1, 'template'),
      dueDate: z.string(),
      contactEmail: z.string(),
      contactLanguage: z.string(),
    })
    .superRefine((d, ctx) => {
      const due = endOfDay(d.dueDate);
      if (!due) {
        ctx.addIssue({ code: 'custom', path: ['dueDate'], message: 'dueDate' });
      } else if (due.getTime() <= rules.now.getTime()) {
        // domain.NewVendorAssessment: "due_at must be in the future".
        ctx.addIssue({ code: 'custom', path: ['dueDate'], message: 'duePast' });
      }
      const email = d.contactEmail.trim();
      if (email === '') {
        if (rules.emailRequired) {
          ctx.addIssue({ code: 'custom', path: ['contactEmail'], message: 'emailRequired' });
        }
      } else if (!EMAIL.test(email)) {
        ctx.addIssue({ code: 'custom', path: ['contactEmail'], message: 'email' });
      }
    });
  return collect(schema.safeParse(draft));
}

export function toSendInput(draft: SendDraft): SendVendorAssessmentInput {
  const due = endOfDay(draft.dueDate);
  const input: SendVendorAssessmentInput = {
    template_id: draft.templateId,
    due_at: (due ?? new Date(Number.NaN)).toISOString(),
  };
  const email = draft.contactEmail.trim();
  if (email) input.contact_email = email;
  if (draft.contactLanguage) input.contact_language = draft.contactLanguage === 'en' ? 'en' : 'fr';
  return input;
}
