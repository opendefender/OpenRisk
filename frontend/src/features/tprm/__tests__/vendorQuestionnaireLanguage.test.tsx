// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #966 — the questionnaire follows the language the organization chose for the
// vendor, not the visitor's stored language, and only French ships in the
// entry bundle. The page must fetch the English catalogue itself. The shared
// test setup preloads every catalogue, which hid exactly this, so these tests
// start from fresh modules: the state of a cold page load on a vendor's browser.

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import type { VendorQuestionnaireView } from '../vendorQuestionnaireService';

const TOKEN = 'tok_language_test_ABCDEFGHIJKLMNOPQRSTUVWXYZ';

function view(language: string): VendorQuestionnaireView {
  return {
    organization_name: 'Example Bank',
    vendor_name: 'Acme Cloud',
    language,
    due_at: '2026-03-15T00:00:00Z',
    status: 'sent',
    read_only: false,
    items: [],
  };
}

async function freshPage(language: string) {
  vi.resetModules();
  vi.doMock('../vendorQuestionnaireService', async () => {
    const actual = await vi.importActual<typeof import('../vendorQuestionnaireService')>(
      '../vendorQuestionnaireService',
    );
    return {
      ...actual,
      vendorQuestionnaireService: {
        get: vi.fn().mockResolvedValue(view(language)),
        saveAnswers: vi.fn(),
        submit: vi.fn(),
      },
    };
  });
  const { VendorQuestionnairePage } = await import('../VendorQuestionnairePage');
  const { isCatalogLoaded } = await import('../../../i18n/catalog');
  render(
    <MemoryRouter initialEntries={[`/vendor-questionnaire#${TOKEN}`]}>
      <VendorQuestionnairePage />
    </MemoryRouter>,
  );
  return { isCatalogLoaded };
}

describe('the vendor questionnaire in a language the entry bundle does not carry (#966)', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('renders an English questionnaire in English on a cold load', async () => {
    const { isCatalogLoaded } = await freshPage('en');
    expect(isCatalogLoaded('en')).toBe(false);

    expect(
      await screen.findByRole('heading', { level: 1, name: 'Security questionnaire' }),
    ).toBeInTheDocument();
    expect(screen.queryByText('Questionnaire de sécurité')).not.toBeInTheDocument();
  });

  it('switches a French questionnaire to English from its own toggle', async () => {
    await freshPage('fr');
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Questionnaire de sécurité' }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByTestId('questionnaire-lang-toggle'));

    expect(
      await screen.findByRole('heading', { level: 1, name: 'Security questionnaire' }),
    ).toBeInTheDocument();
  });
});
