// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The template editor's rules mirror the server's (#681).

import { describe, expect, it } from 'vitest';

import {
  emptyDraft,
  fromTemplate,
  move,
  newOption,
  newQuestion,
  toTemplateInput,
  validateTemplate,
  type TemplateDraft,
} from '../questionnaireTemplateForm';

function valid(): TemplateDraft {
  const draft = emptyDraft('fr');
  draft.name = 'Baseline sécurité';
  draft.questions[0].text = 'Imposez-vous le MFA ?';
  return draft;
}

describe('validateTemplate', () => {
  it('accepts a named questionnaire with a valid choice question', () => {
    expect(validateTemplate(valid())).toEqual({});
  });

  it.each([
    ['a blank name', (d: TemplateDraft) => (d.name = '   '), 'name', 'name'],
    ['no question', (d: TemplateDraft) => (d.questions = []), 'questions', 'oneQuestion'],
    ['a blank question', (d: TemplateDraft) => (d.questions[0].text = ''), 'questions.0.text', 'questionText'],
    ['one option only', (d: TemplateDraft) => (d.questions[0].options = d.questions[0].options.slice(0, 1)), 'questions.0.options', 'twoOptions'],
    ['a weight above 10', (d: TemplateDraft) => (d.questions[0].weight = 11), 'questions.0.weight', 'weight'],
    ['points above 1', (d: TemplateDraft) => (d.questions[0].options[0].points = 1.5), 'questions.0.options.0.points', 'points'],
    ['an option without a label', (d: TemplateDraft) => (d.questions[0].options[1].label = ' '), 'questions.0.options.1.label', 'optionLabel'],
    ['two options with the same label', (d: TemplateDraft) => (d.questions[0].options[1].label = 'oui'), 'questions.0.options.1.label', 'uniqueOptions'],
  ])('refuses %s', (_name, mutate, path, message) => {
    const draft = valid();
    mutate(draft);
    expect(validateTemplate(draft)[path]).toBe(message);
  });

  it('does not apply choice rules to a text question', () => {
    const draft = valid();
    draft.questions[0].answerType = 'text';
    draft.questions[0].options = [];
    draft.questions[0].weight = 99;
    expect(validateTemplate(draft)).toEqual({});
  });
});

describe('toTemplateInput', () => {
  it('sends a text question with weight 0 and no options, and trims', () => {
    const draft = valid();
    draft.name = '  Baseline  ';
    const text = newQuestion('fr');
    text.text = ' Décrivez ';
    text.answerType = 'text';
    text.weight = 8;
    draft.questions.push(text);

    const input = toTemplateInput(draft);

    expect(input.name).toBe('Baseline');
    expect(input.questions[1]).toMatchObject({ text: 'Décrivez', answer_type: 'text', weight: 0 });
    expect(input.questions[1].options).toBeUndefined();
    expect(input.questions[0].options?.map((o) => o.label)).toEqual(['Oui', 'Non']);
  });
});

describe('fromTemplate', () => {
  it('orders questions by position and keeps option values, so answers stay matched', () => {
    const draft = fromTemplate({
      id: 't-1',
      name: 'Baseline',
      language: 'en',
      version: 3,
      archived_at: null,
      created_at: '',
      updated_at: '',
      questions: [
        { id: 'q-2', template_id: 't-1', position: 2, text: 'Second', answer_type: 'text' },
        {
          id: 'q-1',
          template_id: 't-1',
          position: 1,
          text: 'First',
          answer_type: 'choice',
          weight: 4,
          options: [
            { value: 'kept-yes', label: 'Yes', points: 1 },
            { value: 'kept-no', label: 'No', points: 0 },
          ],
        },
      ],
    });

    expect(draft.language).toBe('en');
    expect(draft.questions.map((q) => q.text)).toEqual(['First', 'Second']);
    expect(draft.questions[0].options.map((o) => o.value)).toEqual(['kept-yes', 'kept-no']);
  });
});

describe('helpers', () => {
  it('gives a new option a value unique within its question', () => {
    const q = newQuestion('en');
    const third = newOption(q.options);
    expect(q.options.map((o) => o.value)).not.toContain(third.value);
  });

  it('moves an item and ignores out-of-range moves', () => {
    expect(move(['a', 'b', 'c'], 0, 2)).toEqual(['b', 'c', 'a']);
    expect(move(['a', 'b', 'c'], 0, -1)).toEqual(['a', 'b', 'c']);
    expect(move(['a', 'b', 'c'], 2, 3)).toEqual(['a', 'b', 'c']);
  });
});
