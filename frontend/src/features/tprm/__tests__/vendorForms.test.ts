// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Vendor screens' form rules and value mappings (#673).

import { describe, expect, it } from 'vitest';

import {
  emptySendDraft,
  endOfDay,
  toSendInput,
  validateLink,
  validateSend,
  type SendDraft,
} from '../vendorForms';
import { isOpenAssessment, tierIntent, verbKey } from '../vendorDisplay';

const NOW = new Date(2026, 8, 15, 10, 0, 0);

function draft(overrides: Partial<SendDraft> = {}): SendDraft {
  return {
    templateId: 't-1',
    dueDate: '2026-10-01',
    contactEmail: '',
    contactLanguage: '',
    ...overrides,
  };
}

describe('validateLink', () => {
  it('requires an asset and a verb the server accepts', () => {
    expect(validateLink({ assetId: '', verb: 'managed_by' })).toEqual({ assetId: 'asset' });
    expect(validateLink({ assetId: 'a-1', verb: 'runs_on' })).toEqual({ verb: 'verb' });
    expect(validateLink({ assetId: 'a-1', verb: 'processes_data_of' })).toEqual({});
  });
});

describe('validateSend', () => {
  const rules = { now: NOW, emailRequired: false };

  it('accepts a complete draft', () => {
    expect(validateSend(draft(), rules)).toEqual({});
  });

  it('requires a questionnaire', () => {
    expect(validateSend(draft({ templateId: '' }), rules)).toEqual({ templateId: 'template' });
  });

  it('refuses a missing or impossible date', () => {
    expect(validateSend(draft({ dueDate: '' }), rules)).toEqual({ dueDate: 'dueDate' });
    expect(validateSend(draft({ dueDate: '2026-02-31' }), rules)).toEqual({ dueDate: 'dueDate' });
  });

  it('refuses a due date in the past, as the server does', () => {
    expect(validateSend(draft({ dueDate: '2026-09-14' }), rules)).toEqual({ dueDate: 'duePast' });
  });

  it('accepts today, since the vendor has until the end of the day', () => {
    expect(validateSend(draft({ dueDate: '2026-09-15' }), rules)).toEqual({});
  });

  it('checks the email only when one is typed, unless the vendor has none on record', () => {
    expect(validateSend(draft({ contactEmail: 'not-an-email' }), rules)).toEqual({
      contactEmail: 'email',
    });
    expect(validateSend(draft(), { now: NOW, emailRequired: true })).toEqual({
      contactEmail: 'emailRequired',
    });
    expect(
      validateSend(draft({ contactEmail: 'secu@acme.test' }), { now: NOW, emailRequired: true }),
    ).toEqual({});
  });
});

describe('toSendInput', () => {
  it('sends the end of the chosen local day and omits what the server defaults', () => {
    const input = toSendInput(draft());
    expect(input).toEqual({
      template_id: 't-1',
      due_at: new Date(2026, 9, 1, 23, 59, 59).toISOString(),
    });
  });

  it('passes a typed email, trimmed, and a chosen language', () => {
    const input = toSendInput(draft({ contactEmail: '  secu@acme.test ', contactLanguage: 'en' }));
    expect(input.contact_email).toBe('secu@acme.test');
    expect(input.contact_language).toBe('en');
  });
});

describe('helpers', () => {
  it('proposes a due date two weeks out', () => {
    expect(emptySendDraft(NOW).dueDate).toBe('2026-09-29');
  });

  it('reads a date input as the end of that day', () => {
    expect(endOfDay('2026-09-15')?.getHours()).toBe(23);
    expect(endOfDay('15/09/2026')).toBeNull();
  });

  it('maps tiers case-insensitively and never crashes on an unknown one', () => {
    expect(tierIntent('CRITICAL')).toBe('danger');
    expect(tierIntent('low')).toBe('success');
    expect(tierIntent('severe')).toBe('neutral');
    expect(tierIntent(null)).toBe('neutral');
  });

  it('labels stored verb aliases like the verbs they stand for', () => {
    expect(verbKey('hosted_on')).toBe('hosted_by');
    expect(verbKey('stores_data_in')).toBe('processes_data_of');
    expect(verbKey('connects_to')).toBe('other');
  });

  it('treats only sent and in-progress questionnaires as open', () => {
    expect(isOpenAssessment('sent')).toBe(true);
    expect(isOpenAssessment('in_progress')).toBe(true);
    expect(isOpenAssessment('submitted')).toBe(false);
    expect(isOpenAssessment('expired')).toBe(false);
  });
});
